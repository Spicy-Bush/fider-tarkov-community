package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/lib/pq"
)

var errMediaReferenced = errors.New("image has protected references")

func newFileDeletion() dto.FileDeletion {
	return dto.FileDeletion{
		Deleted: []string{},
		Pending: []string{},
		Skipped: []string{},
		Errors:  []dto.FileFailure{},
	}
}

func deleteFiles(ctx context.Context, c *cmd.DeleteFiles) error {
	if ctx.Value(app.TransactionCtxKey) != nil {
		return errors.New("file deletion must own its transactions")
	}

	c.Result = newFileDeletion()
	removal := mediaReferenceRemoval{Force: c.Force, IncludeDeleted: c.IncludeDeleted, IncludeDrafts: c.IncludeDrafts}
	keys := slices.Clone(c.BlobKeys)
	slices.Sort(keys)
	keys = slices.Compact(keys)
	for _, key := range deleteUnreferencedMedia(ctx, keys, &c.Result) {
		deleteMediaFile(ctx, key, removal, &c.Result)
	}
	slices.Sort(c.Result.Deleted)

	return nil
}

func deleteUnreferencedMedia(ctx context.Context, keys []string, result *dto.FileDeletion) []string {
	if len(keys) == 0 {
		return nil
	}

	var claimed []string
	var completed []string
	transactional := blob.StorageCapabilities().TransactionalWrites
	err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var files []*struct {
			Key     string `db:"key"`
			Deleted bool   `db:"deleted"`
		}
		if err := trx.Select(&files, `
			SELECT key, deleted_at IS NOT NULL AS deleted
			FROM media_assets
			WHERE tenant_id=$1 AND key=ANY($2) AND storage_source=$3
			ORDER BY key FOR UPDATE
		`, tenant.ID, pq.Array(keys), blob.StorageSource()); err != nil {
			return err
		}

		for _, file := range files {
			if file.Deleted {
				completed = append(completed, file.Key)
			}
		}

		// A writer can commit new references while the asset lock is waiting.
		var unused []*struct {
			Key string `db:"key"`
		}
		if err := trx.Select(&unused, `
			UPDATE media_assets asset SET deletion_requested_at=COALESCE(deletion_requested_at,NOW())
			WHERE tenant_id=$1 AND key=ANY($2) AND storage_source=$3 AND deleted_at IS NULL
			  AND NOT EXISTS (
				SELECT 1 FROM media_asset_refs ref WHERE ref.tenant_id=asset.tenant_id AND ref.key=asset.key
			  )
			RETURNING key
		`, tenant.ID, pq.Array(keys), blob.StorageSource()); err != nil {
			return err
		}

		for _, file := range unused {
			claimed = append(claimed, file.Key)
		}

		if transactional {
			for _, key := range claimed {
				if err := bus.Dispatch(ctx, &cmd.DeleteBlob{Key: key}); err != nil && !errors.Is(err, blob.ErrNotFound) {
					return err
				}
			}

			return finishMediaDeletions(ctx, claimed, nil)
		}

		return nil
	})
	if err != nil {
		log.Error(ctx, err)
		return keys
	}

	result.Deleted = append(result.Deleted, completed...)
	if transactional {
		result.Deleted = append(result.Deleted, claimed...)
	} else {
		var deleted, failed []string
		for _, key := range claimed {
			err := bus.Dispatch(ctx, &cmd.DeleteBlob{Key: key})
			if err == nil || errors.Is(err, blob.ErrNotFound) {
				deleted = append(deleted, key)
			} else {
				log.Error(ctx, err)
				failed = append(failed, key)
			}
		}

		if err := finishMediaDeletions(ctx, deleted, failed); err != nil {
			log.Error(ctx, err)
			result.Pending = append(result.Pending, claimed...)
		} else {
			result.Deleted = append(result.Deleted, deleted...)
			result.Pending = append(result.Pending, failed...)
		}
	}

	remaining := make([]string, 0, len(keys)-len(claimed)-len(completed))
	for _, key := range keys {
		if !slices.Contains(claimed, key) && !slices.Contains(completed, key) {
			remaining = append(remaining, key)
		}
	}
	return remaining
}

func deleteMediaFile(ctx context.Context, key string, removal mediaReferenceRemoval, result *dto.FileDeletion) {
	finished := false
	err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var deletedAt sql.NullTime
		if err := trx.Scalar(&deletedAt, `
			SELECT deleted_at FROM media_assets WHERE tenant_id=$1 AND key=$2 AND storage_source=$3
		`, tenant.ID, key, blob.StorageSource()); err != nil {
			return err
		}

		if deletedAt.Valid {
			finished = true
			return nil
		}

		blocked, err := unlinkMediaAssetReferences(ctx, trx, tenant.ID, key, removal)
		if err != nil {
			return err
		}

		if len(blocked) > 0 {
			return errMediaReferenced
		}

		if err := trx.Scalar(&deletedAt, `
			SELECT deleted_at FROM media_assets WHERE tenant_id=$1 AND key=$2 AND storage_source=$3 FOR UPDATE
		`, tenant.ID, key, blob.StorageSource()); err != nil {
			return err
		}

		if deletedAt.Valid {
			finished = true
			return nil
		}

		var protected bool
		err = trx.Scalar(&protected, `
			SELECT EXISTS (
				SELECT 1 FROM media_references
				WHERE tenant_id=$1 AND key=$2 AND media_reference_blocks(scope, $3, $4, $5)
			)
		`, tenant.ID, key, removal.Force, removal.IncludeDeleted, removal.IncludeDrafts)
		if err != nil {
			return err
		}

		if protected {
			return errors.New("media unlink left protected references")
		}

		if _, err := trx.Execute(`
			UPDATE media_assets SET deletion_requested_at=COALESCE(deletion_requested_at,NOW())
			WHERE tenant_id=$1 AND key=$2
		`, tenant.ID, key); err != nil {
			return err
		}

		if blob.StorageCapabilities().TransactionalWrites {
			if err := bus.Dispatch(ctx, &cmd.DeleteBlob{Key: key}); err != nil && !errors.Is(err, blob.ErrNotFound) {
				return err
			}

			if err := finishMediaDeletion(ctx, key, nil); err != nil {
				return err
			}

			finished = true
		}

		return nil
	})
	if errors.Is(err, errMediaReferenced) {
		result.Skipped = append(result.Skipped, key)
		return
	}

	if err != nil {
		log.Error(ctx, err)
		message := "Could not delete this file. Try again."
		if errors.Is(err, app.ErrNotFound) {
			message = "This file was not found. Refresh the file list."
		}

		result.Errors = append(result.Errors, dto.FileFailure{BlobKey: key, Message: message})
		return
	}

	if !finished {
		deleteErr := bus.Dispatch(ctx, &cmd.DeleteBlob{Key: key})
		if errors.Is(deleteErr, blob.ErrNotFound) {
			deleteErr = nil
		}

		if err := finishMediaDeletion(ctx, key, deleteErr); err != nil {
			log.Error(ctx, err)
		} else {
			finished = deleteErr == nil
		}
	}

	if finished {
		result.Deleted = append(result.Deleted, key)
	} else {
		result.Pending = append(result.Pending, key)
	}
}

func finishMediaDeletion(ctx context.Context, key string, failure error) error {
	if failure != nil {
		log.Error(ctx, failure)
		return finishMediaDeletions(ctx, nil, []string{key})
	}

	return finishMediaDeletions(ctx, []string{key}, nil)
}

func finishMediaDeletions(ctx context.Context, deleted, failed []string) error {
	if len(deleted) == 0 && len(failed) == 0 {
		return nil
	}

	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if len(failed) > 0 {
			if _, err := trx.Execute(`
				UPDATE media_assets SET last_error='Storage is unavailable. Deletion will retry.',
					next_deletion_attempt=NOW()+INTERVAL '1 minute'
					WHERE tenant_id=$1 AND key=ANY($2) AND deleted_at IS NULL
			`, tenant.ID, pq.Array(failed)); err != nil {
				return err
			}
		}

		if len(deleted) == 0 {
			return nil
		}

		if _, err := trx.Execute("DELETE FROM media_thumbnails WHERE tenant_id=$1 AND key=ANY($2)", tenant.ID, pq.Array(deleted)); err != nil {
			return err
		}

		_, err := trx.Execute(`
			UPDATE media_assets SET deleted_at=COALESCE(deleted_at,NOW()), last_error=''
			WHERE tenant_id=$1 AND key=ANY($2) AND deletion_requested_at IS NOT NULL
		`, tenant.ID, pq.Array(deleted))
		return err
	})
}

func pruneFiles(ctx context.Context, c *cmd.PruneFiles) error {
	if ctx.Value(app.TransactionCtxKey) != nil {
		return errors.New("file cleanup must own its transactions")
	}

	identity, err := json.Marshal([]any{
		blob.StorageSource(), c.Search, c.Type, c.Before.UTC(), c.Cursor, c.IncludeDeleted, c.IncludeDrafts,
	})
	if err != nil {
		return err
	}

	digest := sha256.Sum256(identity)
	batchID := hex.EncodeToString(digest[:])
	var batch struct {
		Keys       []string `json:"keys"`
		NextCursor string   `json:"nextCursor"`
	}
	err = using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		receipt := commandReceipt{
			TenantID:     tenant.ID,
			UserID:       user.ID,
			Kind:         "media-prune",
			SubmissionID: batchID,
			Fingerprint:  batchID,
		}

		if replayed, err := receipt.read(trx, &batch); err != nil || replayed {
			return err
		}

		var files []*struct {
			Key string `db:"key"`
		}
		err := trx.Select(&files, `
			SELECT key FROM media_file_listing($1,$2,$3,'unused',$4,$5,$6,$8)
			WHERE key>$7 ORDER BY key LIMIT 51
		`, tenant.ID, fileSearchPattern(c.Search), c.Type, c.Before, c.IncludeDeleted, c.IncludeDrafts, c.Cursor, blob.StorageSource())
		if err != nil {
			return err
		}

		if len(files) > 50 {
			files = files[:50]
			batch.NextCursor = files[49].Key
		}

		batch.Keys = make([]string, 0, len(files))
		for _, file := range files {
			batch.Keys = append(batch.Keys, file.Key)
		}

		return receipt.save(trx, batch)
	})
	if err != nil {
		return err
	}

	c.Result = newFileDeletion()
	c.Result.NextCursor = batch.NextCursor
	removal := mediaReferenceRemoval{IncludeDeleted: c.IncludeDeleted, IncludeDrafts: c.IncludeDrafts}
	for _, key := range deleteUnreferencedMedia(ctx, batch.Keys, &c.Result) {
		deleteMediaFile(ctx, key, removal, &c.Result)
	}
	slices.Sort(c.Result.Deleted)

	return nil
}

func retryMediaDeletions(ctx context.Context, _ *cmd.RetryMediaDeletions) error {
	for attempt := 0; attempt < 20; attempt++ {
		var pending struct {
			TenantID int    `db:"tenant_id"`
			Key      string `db:"key"`
		}
		err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			return trx.Get(&pending, `
				WITH candidate AS (
					SELECT tenant_id,key FROM media_assets
					WHERE storage_source=$1 AND deletion_requested_at IS NOT NULL AND deleted_at IS NULL AND next_deletion_attempt<=NOW()
					ORDER BY next_deletion_attempt,tenant_id,key FOR UPDATE SKIP LOCKED LIMIT 1
				)
				UPDATE media_assets a SET next_deletion_attempt=NOW()+INTERVAL '1 minute'
				FROM candidate c WHERE a.tenant_id=c.tenant_id AND a.key=c.key
				RETURNING a.tenant_id,a.key
			`, blob.StorageSource())
		})
		if err == app.ErrNotFound {
			return nil
		}

		if err != nil {
			return err
		}

		tenantContext := context.WithValue(ctx, app.TenantCtxKey, &entity.Tenant{ID: pending.TenantID})
		operation, cancel := context.WithTimeout(tenantContext, 30*time.Second)
		deleteErr := bus.Dispatch(operation, &cmd.DeleteBlob{Key: pending.Key})
		cancel()
		if errors.Is(deleteErr, blob.ErrNotFound) {
			deleteErr = nil
		}

		if err := finishMediaDeletion(tenantContext, pending.Key, deleteErr); err != nil {
			return err
		}
	}

	return nil
}
