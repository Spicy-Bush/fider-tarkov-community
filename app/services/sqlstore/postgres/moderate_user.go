package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func deleteWarning(ctx context.Context, c *cmd.DeleteWarning) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			DELETE FROM user_warnings 
			WHERE id = $1 AND user_id = $2 AND tenant_id = $3
		`, c.WarningID, c.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete warning")
		}
		return nil
	})
}

func deleteMute(ctx context.Context, c *cmd.DeleteMute) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			DELETE FROM user_mutes 
			WHERE id = $1 AND user_id = $2 AND tenant_id = $3
		`, c.MuteID, c.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete mute")
		}
		return nil
	})
}

func expireWarning(ctx context.Context, c *cmd.ExpireWarning) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE user_warnings 
			SET expires_at = NOW()
			WHERE id = $1 AND user_id = $2 AND tenant_id = $3 AND (expires_at IS NULL OR expires_at > NOW())
		`, c.WarningID, c.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to expire warning")
		}
		return nil
	})
}

func expireMute(ctx context.Context, c *cmd.ExpireMute) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE user_mutes 
			SET expires_at = NOW()
			WHERE id = $1 AND user_id = $2 AND tenant_id = $3 AND (expires_at IS NULL OR expires_at > NOW())
		`, c.MuteID, c.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to expire mute")
		}
		return nil
	})
}
