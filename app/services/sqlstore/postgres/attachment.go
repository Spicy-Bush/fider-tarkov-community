package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/chai2010/webp"
)

func setAttachments(ctx context.Context, c *cmd.SetAttachments) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		postID := c.Post.ID
		var commentID sql.NullInt64
		if c.Comment != nil {
			err := commentID.Scan(c.Comment.ID)
			if err != nil {
				return errors.Wrap(err, "failed scan comment id")
			}
		}

		for _, attachment := range c.Attachments {
			if attachment.Remove {
				if _, err := trx.Execute(
					"DELETE FROM attachments WHERE tenant_id = $1 AND post_id = $2 AND (comment_id = $3 OR ($3 IS NULL AND comment_id IS NULL)) AND attachment_bkey = $4",
					tenant.ID, postID, commentID, attachment.BlobKey,
				); err != nil {
					return errors.Wrap(err, "failed to delete attachment")
				}
			} else {
				if _, err := trx.Execute(
					"INSERT INTO attachments (tenant_id, post_id, comment_id, user_id, attachment_bkey) VALUES ($1, $2, $3, $4, $5)",
					tenant.ID, postID, commentID, user.ID, attachment.BlobKey,
				); err != nil {
					return errors.Wrap(err, "failed to insert attachment")
				}
			}
		}

		return nil
	})
}

func getAttachments(ctx context.Context, q *query.GetAttachments) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]string, 0)

		postID := q.Post.ID
		var commentID sql.NullInt64
		if q.Comment != nil {
			err := commentID.Scan(q.Comment.ID)
			if err != nil {
				return errors.Wrap(err, "failed scan comment id")
			}
		}

		type entry struct {
			BlobKey string `db:"attachment_bkey"`
		}

		entries := []*entry{}
		err := trx.Select(&entries, `
			SELECT attachment_bkey
			FROM attachments
			WHERE tenant_id = $1 AND post_id = $2 AND (comment_id = $3 OR ($3 IS NULL AND comment_id IS NULL))
		`, tenant.ID, postID, commentID)
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

func uploadImage(ctx context.Context, c *cmd.UploadImage) error {
	if c.Image.Upload == nil || len(c.Image.Upload.Content) == 0 {
		return nil
	}

	src, _, err := image.Decode(bytes.NewReader(c.Image.Upload.Content))
	if err != nil {
		fileBase := strings.TrimSuffix(
			c.Image.Upload.FileName,
			filepath.Ext(c.Image.Upload.FileName),
		)

		encodedName := base64.RawURLEncoding.EncodeToString([]byte(fileBase))
		bkey := fmt.Sprintf("%s/%s-%s.webp", c.Folder, rand.String(64), encodedName)

		err = bus.Dispatch(ctx, &cmd.StoreBlob{
			Key:         bkey,
			Content:     c.Image.Upload.Content,
			ContentType: c.Image.Upload.ContentType,
		})
		if err != nil {
			return errors.Wrap(err, "failed to upload new blob")
		}

		c.Image.BlobKey = bkey
		return nil
	}

	var buf bytes.Buffer
	encodeOptions := &webp.Options{
		Lossless: false,
		Quality:  80,
	}
	if err := webp.Encode(&buf, src, encodeOptions); err != nil {
		return errors.Wrap(err, "failed to encode image as WebP")
	}

	c.Image.Upload.Content = buf.Bytes()
	c.Image.Upload.ContentType = "image/webp"

	fileBase := strings.TrimSuffix(
		c.Image.Upload.FileName,
		filepath.Ext(c.Image.Upload.FileName),
	)

	encodedName := base64.RawURLEncoding.EncodeToString([]byte(fileBase))

	bkey := fmt.Sprintf("%s/%s-%s.webp", c.Folder, rand.String(64), encodedName)

	err = bus.Dispatch(ctx, &cmd.StoreBlob{
		Key:         bkey,
		Content:     c.Image.Upload.Content,
		ContentType: c.Image.Upload.ContentType,
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload new blob")
	}

	c.Image.BlobKey = bkey
	return nil
}

func uploadImages(ctx context.Context, c *cmd.UploadImages) error {
	for _, img := range c.Images {
		if err := bus.Dispatch(ctx, &cmd.UploadImage{
			Image:  img,
			Folder: c.Folder,
		}); err != nil {
			return err
		}
	}
	return nil
}
