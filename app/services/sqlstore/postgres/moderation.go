package postgres

import (
	"context"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func setModerationPending(ctx context.Context, c *cmd.SetModerationPending) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		c.Comment = nil
		c.Discussion = nil
		var table string
		switch c.ContentType {
		case "post":
			table = "posts"
		case "comment":
			owner := &query.GetDiscussion{CommentID: c.ContentID, LockOwner: true}
			if err := getDiscussion(ctx, owner); err != nil {
				return err
			}

			comment := &query.GetCommentByID{CommentID: c.ContentID}
			if err := getCommentByID(ctx, comment); err != nil {
				return err
			}

			if !comment.Result.AllowedActions(user, owner.Result, tenant, time.Now()).Moderate {
				return validate.Unauthorized()
			}
			c.Discussion = owner.Result
			c.Comment = comment.Result

			table = "comments"
		default:
			return errors.New("invalid content type: %s", c.ContentType)
		}

		_, err := trx.Execute(`UPDATE `+table+` SET moderation_pending=$1,
            moderation_data=CASE WHEN $1 THEN '{"source":"staff"}'::jsonb ELSE NULL END
            WHERE id=$2 AND tenant_id=$3`, c.Pending, c.ContentID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to set moderation_pending for %s %d", c.ContentType, c.ContentID)
		}

		err = cancelModerationCheck(trx, tenant.ID, c.ContentID, c.ContentType)
		if err == nil && c.Comment != nil {
			c.Comment.ModerationPending = c.Pending
			c.Comment.ModerationData = ""
			if c.Pending {
				c.Comment.ModerationData = `{"source":"staff"}`
			}
		}
		return err
	})
}
