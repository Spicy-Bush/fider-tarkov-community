package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func uploadImageFile(ctx context.Context, c *cmd.UploadImageFile) error {
	return uploadManagedImage(ctx, c, entity.ManageFiles)
}

func uploadSponsorImage(ctx context.Context, c *cmd.UploadSponsorImage) error {
	upload := &cmd.UploadImageFile{Name: c.Name, Content: c.Content, SubmissionID: c.SubmissionID, Type: enum.FileUploadPublic}
	err := uploadManagedImage(ctx, upload, entity.ManageSponsorship)
	c.Result = upload.Result
	return err
}

func uploadManagedImage(ctx context.Context, c *cmd.UploadImageFile, permission entity.Permission) error {
	c.Result = nil
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user := ctx.Value(app.UserCtxKey).(*entity.User)
	prefix := c.Type.Prefix()
	digest := sha256.Sum256(c.Content)
	fingerprintBytes, err := json.Marshal([]string{c.Name, prefix, hex.EncodeToString(digest[:])})
	if err != nil {
		return err
	}
	fingerprint := string(fingerprintBytes)
	receipt := commandReceipt{
		TenantID: tenant.ID, UserID: user.ID, Kind: "media-upload",
		SubmissionID: c.SubmissionID, Fingerprint: fingerprint,
	}

	var prepared *dto.PreparedImage
	for {
		err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			ctx, err := permissionContext(ctx, trx, tenant, user, permission)
			if err != nil {
				return err
			}

			var key string
			replayed, err := receipt.read(trx, &key)
			if err != nil {
				return err
			}
			if !replayed {
				if prepared == nil {
					return nil
				}

				key = prepared.Key
				upload := mediaUpload{
					PreparedImage: prepared,
					Name:          c.Name,
					Public:        c.Type == enum.FileUploadPublic,
				}
				if err := saveMediaImage(ctx, upload); err != nil {
					return err
				}
				if err := receipt.save(trx, key); err != nil {
					return err
				}
			}

			stored := &query.GetMediaFile{BlobKey: key}
			if err := getMediaFile(ctx, stored); err != nil {
				return err
			}
			c.Result = stored.Result
			return nil
		})
		if err != nil || c.Result != nil {
			return err
		}

		source, format, err := imagic.Decode(c.Content)
		if err != nil {
			return validate.Failed(err.Error())
		}

		content := c.Content
		contentType := "image/" + format
		if imageNeedsResize(source.Bounds().Dx(), source.Bounds().Dy()) {
			source = imagic.Resize(maxImageDimension)(source, format)
			content, err = imagic.EncodeWebP(source)
			if err != nil {
				return err
			}
			contentType = "image/webp"
		}

		identity := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s:%s", tenant.ID, user.ID, c.SubmissionID, fingerprint)))
		prepared, err = prepareMediaImage(source, prefix+hex.EncodeToString(identity[:]), contentType, content)
		if err != nil {
			return err
		}
		if err := storePreparedImage(ctx, prepared); err != nil {
			return err
		}
	}
}

func renameImageFile(ctx context.Context, c *cmd.RenameImageFile) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		changed, err := trx.Execute(`
			UPDATE media_assets SET name = $3
			WHERE tenant_id = $1 AND key = $2 AND deleted_at IS NULL AND deletion_requested_at IS NULL AND storage_source=$4
		`, tenant.ID, c.BlobKey, c.Name, blob.StorageSource())
		if err != nil {
			return err
		}
		if changed == 0 {
			return app.ErrNotFound
		}

		stored := &query.GetMediaFile{BlobKey: c.BlobKey}
		if err := getMediaFile(ctx, stored); err != nil {
			return err
		}
		c.Result = stored.Result
		return nil
	})
}
