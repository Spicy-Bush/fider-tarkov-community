package postgres

import (
	"context"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func submitPost(ctx context.Context, c *cmd.SubmitPost) error {
	if !validate.ValidSubmissionID(c.SubmissionID) {
		return validate.Failed("Invalid submission identity.")
	}

	for {
		prepare := false
		var imageIdentity string
		err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			if user == nil {
				return validate.Unauthorized()
			}

			receipt := commandReceipt{
				TenantID: tenant.ID, UserID: user.ID, Kind: "post",
				SubmissionID: c.SubmissionID, Fingerprint: c.Fingerprint,
			}
			var savedID int
			replayed, err := receipt.read(trx, &savedID)
			if err != nil {
				return err
			}

			if !replayed {
				// Allocating a post number also writes this tenant row.
				if _, err := trx.Execute("SELECT id FROM tenants WHERE id=$1 FOR NO KEY UPDATE", tenant.ID); err != nil {
					return err
				}
			}
			ctx, err = lockedContentContext(ctx, trx, tenant, user)
			if err != nil {
				return err
			}
			tenant = ctx.Value(app.TenantCtxKey).(*entity.Tenant)
			user = ctx.Value(app.UserCtxKey).(*entity.User)

			if replayed {
				var saved struct {
					ID     int    `db:"id"`
					Number int    `db:"number"`
					Title  string `db:"title"`
					Slug   string `db:"slug"`
				}
				if err := trx.Get(&saved, `
					SELECT post.id, post.number, COALESCE(visible.title, '') AS title,
						COALESCE(visible.slug, '') AS slug
					FROM posts post
					LEFT JOIN visible_posts_for($1, $3::boolean, $2) visible ON visible.id=post.id
					WHERE post.tenant_id=$1 AND post.id=$4
				`, tenant.ID, user.ID, entity.Can(user, tenant, entity.ModeratePosts), savedID); err != nil {
					return err
				}
				c.Result = &dto.PostSubmissionReceipt{ID: saved.ID, Number: saved.Number, Title: saved.Title, Slug: saved.Slug}
				return nil
			}

			if err := c.Validate(ctx); err != nil {
				return err
			}
			if imagesNeedPreparation(c.Attachments) {
				prepare = true
				imageIdentity = fmt.Sprintf("post:%d:%d:%s:%s", tenant.ID, user.ID, c.SubmissionID, c.Fingerprint)
				return nil
			}

			post, err := c.Create(ctx)
			if err != nil || post == nil {
				return err
			}
			c.Result = &dto.PostSubmissionReceipt{
				ID:     post.ID,
				Number: post.Number,
				Title:  post.Title,
				Slug:   post.Slug,
			}

			if err := receipt.save(trx, post.ID); err != nil {
				return err
			}

			if err := scheduleNotification(ctx, &cmd.ScheduleNotification{
				PostID:  post.ID,
				BaseURL: c.BaseURL,
			}); err != nil {
				return err
			}
			return nil
		})
		if err != nil || !prepare {
			return err
		}
		if err := prepareSubmissionImages(ctx, c.Attachments, imageIdentity); err != nil {
			return err
		}
	}
}
