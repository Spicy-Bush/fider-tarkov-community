package postgres

import (
	"context"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/lib/pq"
)

type attachmentChanges struct {
	uploaded []string
	stored   []string
	removed  []string
}

func canUseStoredImage(ctx context.Context, q *query.CanUseStoredImage) error {
	q.Result = false
	if err := blob.ValidateKey(q.Key); err != nil {
		return nil
	}

	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil || tenant == nil || user.Status != enum.UserActive {
			return nil
		}

		return trx.Scalar(&q.Result, `
			SELECT EXISTS (
				SELECT 1 FROM media_assets a
				WHERE a.tenant_id = $1 AND a.key = $2 AND a.storage_source = $3
				  AND a.deleted_at IS NULL AND a.deletion_requested_at IS NULL
				  AND ($5::integer = 0 OR a.size <= $5::bigint * 1024)
				  AND (a.is_public OR EXISTS (
					SELECT 1 FROM page_drafts d
					WHERE d.tenant_id = a.tenant_id AND d.banner_image_bkey = a.key
					  AND (d.user_id = $4 OR (d.shared AND $6))
				) OR ($6 AND a.page_id IS NOT NULL))
			)
		`, tenant.ID, q.Key, blob.StorageSource(), user.ID, q.MaxKilobytes, entity.Can(user, tenant, entity.ManagePages))
	})
}

func uploadAttachments(ctx context.Context, images []*dto.ImageUpload) (attachmentChanges, error) {
	var changes attachmentChanges
	for _, attachment := range images {
		if attachment.Remove {
			changes.removed = append(changes.removed, attachment.BlobKey)
			continue
		}

		if attachment.Upload == nil {
			if attachment.BlobKey != "" {
				changes.stored = append(changes.stored, attachment.BlobKey)
			}
			continue
		}

		if len(attachment.Upload.Content) == 0 {
			return attachmentChanges{}, validate.Failed("The image is empty.")
		}

		uploaded := &dto.ImageUpload{Upload: attachment.Upload}
		if err := uploadImage(ctx, &cmd.UploadImage{Image: uploaded, Folder: "attachments"}); err != nil {
			return attachmentChanges{}, err
		}

		changes.uploaded = append(changes.uploaded, uploaded.BlobKey)
	}

	return changes, nil
}

func (changes attachmentChanges) apply(ctx context.Context, postID, commentID int) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		for _, key := range changes.removed {
			if _, err := trx.Execute(`
				DELETE FROM attachments
				WHERE tenant_id = $1 AND post_id IS NOT DISTINCT FROM NULLIF($2, 0)
				  AND comment_id IS NOT DISTINCT FROM NULLIF($3, 0) AND attachment_bkey = $4
			`, tenant.ID, postID, commentID, key); err != nil {
				return errors.Wrap(err, "failed to delete attachment")
			}
		}

		seen := make(map[string]bool, len(changes.stored))
		for _, key := range changes.removed {
			seen[key] = true
		}

		for _, key := range changes.stored {
			if seen[key] {
				continue
			}
			seen[key] = true
			var exists bool
			if err := trx.Scalar(&exists, `SELECT EXISTS(SELECT 1 FROM attachments
				WHERE tenant_id=$1 AND post_id IS NOT DISTINCT FROM NULLIF($2,0)
				AND comment_id IS NOT DISTINCT FROM NULLIF($3,0) AND attachment_bkey=$4)`, tenant.ID, postID, commentID, key); err != nil {
				return err
			}
			if exists {
				continue
			}

			claim := &query.CanUseStoredImage{Key: key}
			if err := canUseStoredImage(ctx, claim); err != nil {
				return err
			}
			if !claim.Result {
				return validate.Failed("The stored image is unavailable or belongs to another account.")
			}
			changes.uploaded = append(changes.uploaded, key)
		}

		for _, key := range changes.uploaded {
			if _, err := trx.Execute(`
				INSERT INTO attachments (tenant_id, post_id, comment_id, user_id, attachment_bkey)
				VALUES ($1, NULLIF($2, 0), NULLIF($3, 0), $4, $5)
			`, tenant.ID, postID, commentID, user.ID, key); err != nil {
				return errors.Wrap(err, "failed to insert attachment")
			}
		}

		return nil
	})
}

func setCommentAttachments(ctx context.Context, postID, commentID int, images []*dto.ImageUpload) error {
	changes, err := uploadAttachments(ctx, images)
	if err != nil {
		return err
	}

	return changes.apply(ctx, postID, commentID)
}

func getPostAttachments(ctx context.Context, q *query.GetPostAttachments) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		type entry struct {
			BlobKey string `db:"attachment_bkey"`
		}

		entries := []*entry{}
		err := trx.Select(&entries, `
			SELECT attachment_bkey
			FROM attachments
			WHERE tenant_id = $1 AND post_id = $2 AND comment_id IS NULL
		`, tenant.ID, q.PostID)
		if err != nil {
			return errors.Wrap(err, "failed to get attachments")
		}

		q.Result = make([]string, len(entries))
		for i, entry := range entries {
			q.Result[i] = entry.BlobKey
		}

		return nil
	})
}

func canReadAttachment(ctx context.Context, q *query.CanReadAttachment) error {
	q.Result = false
	q.Version = ""
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	if tenant == nil || (tenant.IsPrivate && user == nil) {
		return nil
	}

	return dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		var access struct {
			Allowed bool   `db:"allowed"`
			Version string `db:"version"`
		}
		err := trx.Get(&access, `
			SELECT allowed, version
			FROM image_access($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, tenant.ID, q.Key, blob.StorageSource(), viewerID(user), viewerRole(user),
			entity.Can(user, tenant, entity.ManagePages), entity.Can(user, tenant, entity.ModeratePosts),
			pq.Array(entity.ModeratedContentRoles(user, tenant)), user != nil && user.Status == enum.UserActive, enum.PostDeleted)
		if err != nil {
			return err
		}

		q.Result, q.Version = access.Allowed, access.Version
		return nil
	})
}

const maxImageDimension = 1500

func imageNeedsResize(width, height int) bool {
	return width > maxImageDimension && height > maxImageDimension
}

func uploadImage(ctx context.Context, c *cmd.UploadImage) error {
	if c.Image.Upload == nil || len(c.Image.Upload.Content) == 0 {
		return nil
	}
	if err := validate.ImageUploadMetadata(c.Image.Upload); err != nil {
		return err
	}

	bkey := fmt.Sprintf("%s/%s.webp", c.Folder, rand.String(32))
	prepared := c.Image.Upload.Prepared
	if prepared == nil {
		var err error
		prepared, err = prepareInlineImage(ctx, c.Image.Upload.Content, bkey)
		if err != nil {
			return err
		}
	}

	err := saveMediaImage(ctx, mediaUpload{
		PreparedImage: prepared,
		Name:          c.Image.Upload.FileName,
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload new blob")
	}

	c.Image.BlobKey = prepared.Key
	return nil
}
