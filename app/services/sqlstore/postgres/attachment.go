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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type attachmentChanges struct {
	uploaded []string
	removed  []string
}

func uploadAttachments(ctx context.Context, images []*dto.ImageUpload) (attachmentChanges, error) {
	var changes attachmentChanges
	for _, attachment := range images {
		if attachment.Remove {
			changes.removed = append(changes.removed, attachment.BlobKey)
			continue
		}

		if attachment.Upload == nil {
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if tenant.IsPrivate && user == nil {
			return nil
		}

		var owners []*struct {
			PostID    int `db:"post_id"`
			CommentID int `db:"comment_id"`
		}

		if err := trx.Select(&owners, `
            SELECT COALESCE(post_id, 0) AS post_id, COALESCE(comment_id, 0) AS comment_id
            FROM attachments WHERE tenant_id = $1 AND attachment_bkey = $2
        `, tenant.ID, q.Key); err != nil {
			return err
		}

		for _, owner := range owners {
			if owner.CommentID != 0 {
				discussion := &query.GetDiscussion{CommentID: owner.CommentID}
				if err := getDiscussion(ctx, discussion); err != nil {
					if errors.Cause(err) == app.ErrNotFound {
						continue
					}

					return err
				}

				comment, err := readCommentVisibility(trx, tenant.ID, owner.CommentID)
				if err != nil {
					return err
				}

				q.Result = comment.ContentState(user, discussion.Result) == "visible"
			} else {
				post := &query.GetPostByID{PostID: owner.PostID}
				if err := getPostByID(ctx, post); err != nil {
					if errors.Cause(err) == app.ErrNotFound {
						continue
					}

					return err
				}

				staff := user != nil && (user.IsCollaborator() || user.IsModerator())
				author := user != nil && post.Result.User.ID == user.ID
				q.Result = post.Result.Status != enum.PostDeleted && (!post.Result.ModerationPending || staff || author)
			}

			if q.Result {
				return nil
			}
		}

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

	src, format, err := imagic.Decode(c.Image.Upload.Content)
	if err != nil {
		return validate.Failed(err.Error())
	}

	if imageNeedsResize(src.Bounds().Dx(), src.Bounds().Dy()) {
		src = imagic.Resize(maxImageDimension)(src, format)
	}

	content, err := imagic.EncodeWebP(src)
	if err != nil {
		return errors.Wrap(err, "failed to encode image as WebP")
	}

	bkey := fmt.Sprintf("%s/%s.webp", c.Folder, rand.String(32))

	err = bus.Dispatch(ctx, &cmd.StoreBlob{
		Key:         bkey,
		Content:     content,
		ContentType: "image/webp",
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload new blob")
	}

	c.Image.BlobKey = bkey
	return nil
}
