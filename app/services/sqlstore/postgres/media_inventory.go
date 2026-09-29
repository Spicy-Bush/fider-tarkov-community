package postgres

import (
	"context"
	"database/sql"
	"errors"
	"path"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/lib/pq"
)

var errInventoryReplaced = errors.New("media inventory scan was replaced")

type mediaInventoryProgress struct {
	Version       int64        `db:"version"`
	Cursor        string       `db:"cursor"`
	Scanned       int64        `db:"scanned"`
	Skipped       int64        `db:"skipped"`
	ScanStartedAt sql.NullTime `db:"scan_started_at"`
	CompletedAt   sql.NullTime `db:"completed_at"`
	LastError     string       `db:"last_error"`
}

func getMediaInventory(ctx context.Context, q *query.GetMediaInventory) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var progress mediaInventoryProgress
		err := trx.Get(&progress, `
			SELECT version, cursor, scanned, skipped, completed_at, last_error
			FROM media_inventory WHERE tenant_id=$1 AND storage_source=$2
		`, tenant.ID, blob.StorageSource())
		if err != nil && err != app.ErrNotFound {
			return err
		}
		q.Result = &dto.MediaInventory{
			State: "pending", Scanned: progress.Scanned,
			Skipped: progress.Skipped, LastError: progress.LastError,
		}
		if progress.CompletedAt.Valid {
			q.Result.State = "ready"
		} else if progress.LastError != "" {
			q.Result.State = "retrying"
		}
		return nil
	})
}

func refreshMediaInventory(ctx context.Context, _ *cmd.RefreshMediaInventory) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			INSERT INTO media_inventory (tenant_id, storage_source) VALUES ($1, $2)
			ON CONFLICT (tenant_id, storage_source) DO UPDATE
			SET version=media_inventory.version+1, cursor='', scanned=0, skipped=0, completed_at=NULL, scan_started_at=NULL,
			    last_error='', retry_after=NOW(), updated_at=NOW()
		`, tenant.ID, blob.StorageSource())
		return err
	})
}

func importMediaInventory(ctx context.Context, c *cmd.ImportMediaInventory) error {
	if ctx.Value(app.TransactionCtxKey) != nil {
		return errors.New("Media inventory requires a request without an open transaction")
	}
	c.Found = false
	source := blob.StorageSource()
	var tenantID int
	var progress mediaInventoryProgress
	err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		err := trx.Scalar(&tenantID, `
			SELECT t.id FROM tenants t
			LEFT JOIN media_inventory i ON i.tenant_id=t.id AND i.storage_source=$1
			WHERE i.tenant_id IS NULL OR (i.completed_at IS NULL AND i.retry_after<=NOW())
			ORDER BY i.updated_at NULLS FIRST, t.id LIMIT 1
		`, source)
		if err == app.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := trx.Execute(`
			INSERT INTO media_inventory (tenant_id, storage_source) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, tenantID, source); err != nil {
			return err
		}
		if err := trx.Get(&progress, `
			SELECT version, cursor, scanned, skipped, completed_at, last_error, scan_started_at
			FROM media_inventory WHERE tenant_id=$1 AND storage_source=$2 FOR UPDATE
		`, tenantID, source); err != nil {
			return err
		}
		if progress.CompletedAt.Valid || (progress.ScanStartedAt.Valid && blob.StorageCapabilities().ResumableScan) {
			return nil
		}

		if err := trx.Get(&progress, `
			UPDATE media_inventory SET version=version+1, cursor='', scanned=0, skipped=0, scan_started_at=clock_timestamp()
			WHERE tenant_id=$1 AND storage_source=$2
			RETURNING version, cursor, scanned, skipped, scan_started_at
		`, tenantID, source); err != nil {
			return err
		}
		if _, err := trx.Execute("DELETE FROM media_inventory_candidates WHERE tenant_id=$1 AND storage_source=$2", tenantID, source); err != nil {
			return err
		}
		_, err = trx.Execute(`
			INSERT INTO media_inventory_candidates (tenant_id,storage_source,key,cataloged_at)
			SELECT tenant_id,storage_source,key,cataloged_at FROM media_assets
			WHERE tenant_id=$1 AND storage_source=$2
			AND deleted_at IS NULL AND deletion_requested_at IS NULL
		`, tenantID, source)
		return err
	})
	if err != nil || tenantID == 0 || progress.CompletedAt.Valid {
		return err
	}
	c.Found = true
	ctx = context.WithValue(ctx, app.TenantCtxKey, &entity.Tenant{ID: tenantID})
	scan := &query.ScanBlobMetadata{Cursor: progress.Cursor, BatchSize: 200}
	skippedBeforeScan := progress.Skipped
	scan.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
		return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			var current int64
			if err := trx.Scalar(&current, `
				SELECT version FROM media_inventory
				WHERE tenant_id=$1 AND storage_source=$2 FOR UPDATE
			`, tenant.ID, source); err != nil {
				return err
			}
			if current != progress.Version {
				return errInventoryReplaced
			}

			if err := saveInventoryBatch(trx, tenant.ID, source, files); err != nil {
				return err
			}
			if complete {
				if _, err := trx.Execute(`
					UPDATE media_assets a SET storage_source=NULL
					FROM media_inventory_candidates candidate
					WHERE candidate.tenant_id=a.tenant_id AND candidate.key=a.key AND candidate.storage_source=a.storage_source
					AND a.tenant_id=$1 AND a.storage_source=$2 AND a.cataloged_at=candidate.cataloged_at
					AND a.deleted_at IS NULL AND a.deletion_requested_at IS NULL
				`, tenant.ID, source); err != nil {
					return err
				}
				if _, err := trx.Execute(`
					DELETE FROM media_thumbnails t USING media_assets a,media_inventory_candidates candidate
					WHERE t.tenant_id=a.tenant_id AND t.key=a.key
					AND candidate.tenant_id=a.tenant_id AND candidate.key=a.key AND candidate.storage_source=$2
					AND a.tenant_id=$1 AND a.storage_source IS NULL AND a.cataloged_at=candidate.cataloged_at
					AND a.deleted_at IS NULL AND a.deletion_requested_at IS NULL
				`, tenant.ID, source); err != nil {
					return err
				}
				if _, err := trx.Execute("DELETE FROM media_inventory_candidates WHERE tenant_id=$1 AND storage_source=$2", tenant.ID, source); err != nil {
					return err
				}
			}
			_, err := trx.Execute(`
				UPDATE media_inventory SET version=version+1, cursor=$3, scanned=$4,
				    completed_at=CASE WHEN $5 THEN NOW() END,
				    skipped=$6, last_error='', retry_after=NOW(), updated_at=NOW()
				WHERE tenant_id=$1 AND storage_source=$2
			`, tenant.ID, source, next, progress.Scanned+int64(len(files)), complete, skippedBeforeScan+scan.Skipped)
			if err == nil {
				progress.Version++
				progress.Scanned += int64(len(files))
			}
			return err
		})
	}
	err = bus.Dispatch(ctx, scan)
	if err == nil || errors.Is(err, errInventoryReplaced) {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	storeErr := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, storeErr := trx.Execute(`
			UPDATE media_inventory SET version=version+1, last_error=$4,
			    retry_after=NOW()+INTERVAL '30 seconds', updated_at=NOW()
			WHERE tenant_id=$1 AND storage_source=$2 AND version=$3
		`, tenant.ID, source, progress.Version, err.Error())
		return storeErr
	})
	if storeErr != nil {
		return storeErr
	}
	return err
}

func saveInventoryBatch(trx *dbx.Trx, tenantID int, source string, files []dto.BlobMetadata) error {
	if len(files) == 0 {
		return nil
	}
	keys := make([]string, len(files))
	names := make([]string, len(files))
	types := make([]string, len(files))
	sizes := make([]int64, len(files))
	modified := make([]string, len(files))
	for index, file := range files {
		keys[index] = file.Key
		names[index] = path.Base(file.Key)
		types[index] = file.ContentType
		sizes[index] = file.Size
		if !file.ModifiedAt.IsZero() {
			modified[index] = file.ModifiedAt.Format(time.RFC3339Nano)
		}
	}

	if _, err := trx.Execute(`
		DELETE FROM media_thumbnails t USING media_assets a,
		    unnest($2::text[], $3::bigint[], $4::text[]) AS incoming(key,size,modified_at)
		WHERE t.tenant_id=a.tenant_id AND t.key=a.key
		  AND a.tenant_id=$1 AND a.key=incoming.key
		  AND (a.storage_source IS DISTINCT FROM $5 OR a.size IS DISTINCT FROM incoming.size
		       OR (a.storage_modified_at IS NOT NULL AND a.storage_modified_at<>NULLIF(incoming.modified_at,'')::timestamptz))
	`, tenantID, pq.Array(keys), pq.Array(sizes), pq.Array(modified), source); err != nil {
		return err
	}
	_, err := trx.Execute(`
		INSERT INTO media_assets
		    (tenant_id,key,name,content_type,size,created_at,storage_source,storage_modified_at,cataloged_at)
		SELECT $1,key,name,content_type,size,COALESCE(NULLIF(modified_at,'')::timestamptz,NOW()),
		    $2,NULLIF(modified_at,'')::timestamptz,NOW()
		FROM unnest($3::text[],$4::text[],$5::text[],$6::bigint[],$7::text[])
		    AS incoming(key,name,content_type,size,modified_at)
		ON CONFLICT (tenant_id,key) DO UPDATE
		SET storage_source=EXCLUDED.storage_source,
		    name=CASE WHEN media_assets.name IS NULL THEN EXCLUDED.name ELSE media_assets.name END,
		    created_at=CASE WHEN media_assets.name IS NULL THEN EXCLUDED.created_at ELSE media_assets.created_at END,
		    cataloged_at=CASE WHEN media_assets.storage_source IS DISTINCT FROM EXCLUDED.storage_source
		        THEN NOW() ELSE COALESCE(media_assets.cataloged_at,NOW()) END,
		    storage_modified_at=CASE WHEN media_assets.storage_source IS DISTINCT FROM EXCLUDED.storage_source
		        THEN EXCLUDED.storage_modified_at ELSE COALESCE(EXCLUDED.storage_modified_at,media_assets.storage_modified_at) END,
		    size=EXCLUDED.size,
		    content_type=CASE WHEN media_assets.storage_source IS DISTINCT FROM EXCLUDED.storage_source
		        OR media_assets.content_type IS NULL OR media_assets.content_type='application/octet-stream' THEN EXCLUDED.content_type
		        ELSE media_assets.content_type END,
		    width=CASE WHEN media_assets.storage_source IS DISTINCT FROM EXCLUDED.storage_source
		        OR media_assets.size IS DISTINCT FROM EXCLUDED.size
		        OR (media_assets.storage_modified_at IS NOT NULL AND media_assets.storage_modified_at<>EXCLUDED.storage_modified_at)
		        THEN 0 ELSE media_assets.width END,
		    height=CASE WHEN media_assets.storage_source IS DISTINCT FROM EXCLUDED.storage_source
		        OR media_assets.size IS DISTINCT FROM EXCLUDED.size
		        OR (media_assets.storage_modified_at IS NOT NULL AND media_assets.storage_modified_at<>EXCLUDED.storage_modified_at)
		        THEN 0 ELSE media_assets.height END
		WHERE media_assets.state NOT IN ('deleting', 'deleted')
	`, tenantID, source, pq.Array(keys), pq.Array(names), pq.Array(types), pq.Array(sizes), pq.Array(modified))
	if err != nil {
		return err
	}
	_, err = trx.Execute(`
		DELETE FROM media_inventory_candidates
		WHERE tenant_id=$1 AND storage_source=$2 AND key=ANY($3)
	`, tenantID, source, pq.Array(keys))
	return err
}
