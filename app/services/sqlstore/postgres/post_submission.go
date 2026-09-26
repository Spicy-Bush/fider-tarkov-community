package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func submitPost(ctx context.Context, c *cmd.SubmitPost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if c.SubmissionID != "" {
			lock := fmt.Sprintf("post:%d:%d:%s", tenant.ID, user.ID, c.SubmissionID)
			if _, err := trx.Execute("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lock); err != nil {
				return err
			}

			var receipt struct {
				Hash   string `db:"submission_hash"`
				Result string `db:"submission_result"`
			}

			err := trx.Get(&receipt, `SELECT submission_hash, submission_result::text
				FROM posts WHERE tenant_id = $1 AND user_id = $2 AND submission_id = $3`,
				tenant.ID, user.ID, c.SubmissionID)
			if err == nil {
				if receipt.Hash != c.Fingerprint {
					return app.ErrConflict
				}

				return json.Unmarshal([]byte(receipt.Result), &c.Result)
			}

			if errors.Cause(err) != app.ErrNotFound {
				return err
			}
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

		if c.SubmissionID != "" {
			encoded, err := json.Marshal(c.Result)
			if err != nil {
				return err
			}

			if _, err := trx.Execute(`UPDATE posts SET submission_id = $1,
				submission_hash = $2, submission_result = $3 WHERE id = $4 AND tenant_id = $5`,
				c.SubmissionID, c.Fingerprint, string(encoded), post.ID, tenant.ID); err != nil {
				return err
			}
		}

		return schedulePostNotification(ctx, &cmd.SchedulePostNotification{
			Post:    post,
			BaseURL: c.BaseURL,
		})
	})
}
