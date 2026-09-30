package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func deleteSponsorCampaign(ctx context.Context, c *cmd.DeleteSponsorCampaign) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		if err := lockSponsorConfiguration(trx, tenant.ID, true); err != nil {
			return err
		}

		if _, err := trx.Execute(`
			WITH deleted AS (
				UPDATE sponsor_campaigns SET state='deleted', revision=revision+1
				WHERE tenant_id=$1 AND id=$2 AND state<>'deleted'
				RETURNING tenant_id, id, revision, state
			)
			INSERT INTO sponsor_changes (tenant_id, campaign_id, revision, state)
			SELECT tenant_id, id, revision, state FROM deleted
		`, tenant.ID, c.ID); err != nil {
			return err
		}

		_, err := trx.Execute(`
			UPDATE sponsor_creatives
			SET data=jsonb_set(data,'{state}','"deleted"'), image_key='', logo_key='', revision=revision+1
			WHERE tenant_id=$1 AND campaign_id=$2 AND data->>'state'<>'deleted'
		`, tenant.ID, c.ID)
		return err
	})
}

func deleteSponsorCreative(ctx context.Context, c *cmd.DeleteSponsorCreative) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		if err := lockSponsorConfiguration(trx, tenant.ID, true); err != nil {
			return err
		}

		_, err := trx.Execute(`
			UPDATE sponsor_creatives
			SET data=jsonb_set(data,'{state}','"deleted"'), image_key='', logo_key='', revision=revision+1
			WHERE tenant_id=$1 AND id=$2 AND data->>'state'<>'deleted'
		`, tenant.ID, c.ID)
		return err
	})
}
