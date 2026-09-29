package postgres

import (
	"context"
	"encoding/json"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func lockedContentContext(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) (context.Context, error) {
	ctx, err := lockedPermissionContext(ctx, trx, tenant, user)
	if err != nil {
		return nil, err
	}

	tenant = ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user = ctx.Value(app.UserCtxKey).(*entity.User)
	if !entity.CanAct(user, tenant) {
		return nil, validate.Unauthorized()
	}

	var current struct {
		Settings string `db:"settings"`
		Muted    bool   `db:"muted"`
	}
	if err := trx.Get(&current, `
		SELECT COALESCE(general_settings::text, '{}') AS settings,
			EXISTS (SELECT 1 FROM user_mutes
				WHERE tenant_id=$1 AND user_id=$2
				  AND (expires_at IS NULL OR expires_at > statement_timestamp())) AS muted
		FROM tenants WHERE id=$1
	`, tenant.ID, user.ID); err != nil {
		return nil, err
	}

	settings := new(entity.GeneralSettings)
	if err := json.Unmarshal([]byte(current.Settings), settings); err != nil {
		return nil, err
	}
	tenant.GeneralSettings = settings
	user.Muted = current.Muted
	return ctx, nil
}
