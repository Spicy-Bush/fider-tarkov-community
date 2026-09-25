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

		_, err := trx.Execute(`UPDATE `+table+` SET moderation_pending=$1,
            moderation_data=CASE WHEN $1 THEN '{"source":"staff"}'::jsonb ELSE NULL END
            WHERE id=$2 AND tenant_id=$3`, c.Pending, c.ContentID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to set moderation_pending for %s %d", c.ContentType, c.ContentID)
		}

		_, err = trx.Execute(`UPDATE moderation_checks SET revision=revision+1,state='canceled',text_content='',blob_keys='[]',result=NULL,updated_at=NOW()
            WHERE tenant_id=$1 AND content_type=$2 AND content_id=$3`, tenant.ID, c.ContentType, c.ContentID)
		return err
	})
}
