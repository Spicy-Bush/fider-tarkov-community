package postgres

import (
	"context"
	"database/sql"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

// Waiting requests must not retain original image bytes or database connections.
var thumbnailGeneration = make(chan struct{}, 2)

type mediaThumbnailState struct {
	Exists       bool           `db:"exists"`
	Width        int            `db:"width"`
	Height       int            `db:"height"`
	Unavailable  bool           `db:"unavailable"`
	Content      []byte         `db:"content"`
	Source       sql.NullString `db:"storage_source"`
	OriginalSize sql.NullInt64  `db:"size"`
	ModifiedAt   sql.NullTime   `db:"storage_modified_at"`
	CatalogedAt  sql.NullTime   `db:"cataloged_at"`
}

func readMediaThumbnail(ctx context.Context, key string, size int) (mediaThumbnailState, error) {
	var state mediaThumbnailState
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		err := trx.Get(&state, `
			SELECT true AS exists,
			       CASE WHEN a.storage_source=$4 THEN a.width ELSE 0 END AS width,
			       CASE WHEN a.storage_source=$4 THEN a.height ELSE 0 END AS height,
			       a.state IN ('unavailable', 'deleting', 'deleted') AS unavailable,
			       t.content,a.storage_source,a.size,a.storage_modified_at,a.cataloged_at
			FROM media_assets a
			LEFT JOIN media_thumbnails t ON t.tenant_id = a.tenant_id AND t.key = a.key AND t.size = $3 AND a.storage_source=$4
			WHERE a.tenant_id = $1 AND a.key = $2
			FOR SHARE OF a
		`, tenant.ID, key, size, blob.StorageSource())
		if err == app.ErrNotFound {
			return nil
		}

		return err
	})
	if err == nil && state.Unavailable {
		err = app.ErrNotFound
	}

	return state, err
}

func getMediaThumbnail(ctx context.Context, q *query.GetMediaThumbnail) error {
	q.Result = nil
	if !imagic.ValidThumbnailSize(q.Size) {
		return imagic.ErrThumbnailSize
	}

	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	if tenant == nil || (q.AllowUnpublishedAvatar && !entity.Can(user, tenant, entity.ManageFiles)) {
		return app.ErrNotFound
	}

	if trx, _ := ctx.Value(app.TransactionCtxKey).(*dbx.Trx); trx != nil {
		return errors.New("Media thumbnails require a request without an open transaction")
	}

	original := &query.GetBlobByKey{
		Key:                    q.Key,
		AllowUnpublishedAvatar: q.AllowUnpublishedAvatar,
		MaxBytes:               imagic.MaxImageBytes,
	}
	if err := blob.AuthorizeRead(ctx, original); err != nil {
		return err
	}

	state, err := readMediaThumbnail(ctx, q.Key, q.Size)
	if err != nil {
		return err
	}

	if state.Content != nil {
		q.Result = &dto.Blob{Content: state.Content, ContentType: "image/webp", Size: int64(len(state.Content))}
		return nil
	}

	if state.Width > 0 && state.Height > 0 && state.Width <= q.Size && state.Height <= q.Size {
		if err := bus.Dispatch(ctx, original); err != nil {
			return err
		}

		q.Result = original.Result
		return nil
	}

	select {
	case thumbnailGeneration <- struct{}{}:
		defer func() { <-thumbnailGeneration }()
	case <-ctx.Done():
		return ctx.Err()
	}

	for attempt := 0; attempt < 3; attempt++ {
		state, err = readMediaThumbnail(ctx, q.Key, q.Size)
		if err != nil {
			return err
		}

		if state.Content != nil {
			q.Result = &dto.Blob{Content: state.Content, ContentType: "image/webp", Size: int64(len(state.Content))}
			return nil
		}

		if err := bus.Dispatch(ctx, original); err != nil {
			return err
		}

		metadata, err := imagic.Parse(original.Result.Content)
		if err != nil {
			return err
		}

		if original.Result.ContentType == "" || original.Result.ContentType == "application/octet-stream" {
			original.Result.ContentType = "image/" + metadata.Format
		}

		var thumbnail []byte
		if metadata.Width > q.Size || metadata.Height > q.Size {
			source, _, err := imagic.Decode(original.Result.Content)
			if err != nil {
				return err
			}

			thumbnail, err = imagic.Thumbnail(source, q.Size)
			if err != nil {
				return err
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		accepted := false
		err = using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			current, err := readMediaThumbnail(ctx, q.Key, q.Size)
			if err != nil {
				return err
			}

			if current.Exists != state.Exists || current.Source != state.Source ||
				current.OriginalSize != state.OriginalSize ||
				current.ModifiedAt.Valid != state.ModifiedAt.Valid ||
				!current.ModifiedAt.Time.Equal(state.ModifiedAt.Time) ||
				current.CatalogedAt.Valid != state.CatalogedAt.Valid ||
				!current.CatalogedAt.Time.Equal(state.CatalogedAt.Time) {
				return nil
			}

			accepted = true
			if thumbnail == nil || current.Source.String != blob.StorageSource() {
				return nil
			}

			_, err = trx.Execute(`
				INSERT INTO media_thumbnails (tenant_id, key, size, content)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (tenant_id, key, size) DO NOTHING
			`, tenant.ID, q.Key, q.Size, thumbnail)
			return err
		})
		if err != nil {
			return err
		}

		if !accepted {
			original.Result = nil
			continue
		}

		if thumbnail == nil {
			q.Result = original.Result
		} else {
			q.Result = &dto.Blob{Content: thumbnail, ContentType: "image/webp", Size: int64(len(thumbnail))}
		}

		return nil
	}

	return app.ErrConflict
}
