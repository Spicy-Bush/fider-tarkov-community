package postgres

import (
	"context"
	"image"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

type mediaUpload struct {
	*dto.PreparedImage
	Name   string
	Public bool
	PageID int
}

func prepareMediaImage(source image.Image, key, contentType string, content []byte) (*dto.PreparedImage, error) {
	prepared := &dto.PreparedImage{
		Key:         key,
		ContentType: contentType,
		Content:     content,
		Width:       source.Bounds().Dx(),
		Height:      source.Bounds().Dy(),
		Thumbnails:  make(map[int][]byte, 2),
	}
	for _, size := range [...]int{imagic.ThumbnailSmall, imagic.ThumbnailLarge} {
		if prepared.Width <= size && prepared.Height <= size {
			continue
		}

		content, err := imagic.Thumbnail(source, size)
		if err != nil {
			return nil, err
		}
		prepared.Thumbnails[size] = content
	}
	return prepared, nil
}

func storePreparedImage(ctx context.Context, prepared *dto.PreparedImage) error {
	if blob.StorageCapabilities().TransactionalWrites {
		return nil
	}

	return bus.Dispatch(ctx, &cmd.StoreBlob{
		Key:         prepared.Key,
		Content:     prepared.Content,
		ContentType: prepared.ContentType,
	})
}

func saveMediaImage(ctx context.Context, upload mediaUpload) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if blob.StorageCapabilities().TransactionalWrites {
			if err := bus.Dispatch(ctx, &cmd.StoreBlob{
				Key: upload.Key, Content: upload.Content, ContentType: upload.ContentType,
			}); err != nil {
				return err
			}
		}

		changed, err := trx.Execute(`
			INSERT INTO media_assets (tenant_id, key, name, content_type, size, created_at, width, height, is_public, storage_source, page_id, cataloged_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, 0), NOW())
			ON CONFLICT (tenant_id, key) DO UPDATE
			SET name = EXCLUDED.name, content_type = EXCLUDED.content_type, size = EXCLUDED.size,
			    width = EXCLUDED.width, height = EXCLUDED.height, storage_source = EXCLUDED.storage_source,
			    is_public = EXCLUDED.is_public, page_id = EXCLUDED.page_id,
			    created_at = CASE WHEN media_assets.name IS NULL THEN EXCLUDED.created_at ELSE media_assets.created_at END,
			    cataloged_at = clock_timestamp()
			WHERE media_assets.state NOT IN ('deleting', 'deleted')
		`, tenant.ID, upload.Key, upload.Name, upload.ContentType, len(upload.Content), time.Now(), upload.Width, upload.Height, upload.Public, blob.StorageSource(), upload.PageID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return validate.Failed("This image has been deleted. Choose it again to upload a new copy.")
		}
		if _, err := trx.Execute(`
			DELETE FROM media_inventory_candidates WHERE tenant_id=$1 AND storage_source=$2 AND key=$3
		`, tenant.ID, blob.StorageSource(), upload.Key); err != nil {
			return err
		}

		for size, content := range upload.Thumbnails {
			if _, err := trx.Execute(`
				INSERT INTO media_thumbnails (tenant_id, key, size, content)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (tenant_id, key, size) DO UPDATE SET content = EXCLUDED.content
			`, tenant.ID, upload.Key, size, content); err != nil {
				return err
			}
		}

		return nil
	})
}
