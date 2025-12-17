package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func setModerationPending(ctx context.Context, c *cmd.SetModerationPending) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var table string
		switch c.ContentType {
		case "post":
			table = "posts"
		case "comment":
			table = "comments"
		default:
			return errors.New("invalid content type: %s", c.ContentType)
		}

		var err error
		if c.ModerationData != "" {
			_, err = trx.Execute(`
				UPDATE `+table+` 
				SET moderation_pending = $1, moderation_data = $2::jsonb 
				WHERE id = $3 AND tenant_id = $4
			`, c.Pending, c.ModerationData, c.ContentID, tenant.ID)
		} else {
			_, err = trx.Execute(`
				UPDATE `+table+` 
				SET moderation_pending = $1 
				WHERE id = $2 AND tenant_id = $3
			`, c.Pending, c.ContentID, tenant.ID)
		}
		if err != nil {
			return errors.Wrap(err, "failed to set moderation_pending for %s %d", c.ContentType, c.ContentID)
		}

		return nil
	})
}

