package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func userPermissionsForUpdate(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, viewer *entity.User, userID int) (context.Context, entity.UserPermissions, error) {
	if tenant == nil || viewer == nil {
		return nil, entity.UserPermissions{}, validate.Unauthorized()
	}

	currentTenant, err := lockPermissionTenant(trx, tenant)
	if err != nil {
		return nil, entity.UserPermissions{}, err
	}

	var members []*struct {
		ID     int             `db:"id"`
		Role   enum.Role       `db:"role"`
		Status enum.UserStatus `db:"status"`
	}

	// A consistent lock order prevents deadlocks between reciprocal edits.
	if err := trx.Select(&members, `
		SELECT id, role, status FROM users
		WHERE tenant_id=$1 AND id IN ($2,$3)
		ORDER BY id FOR UPDATE
	`, tenant.ID, viewer.ID, userID); err != nil {
		return nil, entity.UserPermissions{}, err
	}

	var currentViewer, target *entity.User
	for _, member := range members {
		if member.ID == viewer.ID {
			current := *viewer
			current.Role, current.Status = member.Role, member.Status
			currentViewer = &current
		}
		if member.ID == userID && member.Status != enum.UserDeleted {
			target = &entity.User{ID: member.ID, Role: member.Role, Status: member.Status}
		}
	}

	if currentViewer == nil {
		return nil, entity.UserPermissions{}, validate.Unauthorized()
	}
	if target == nil {
		return nil, entity.UserPermissions{}, app.ErrNotFound
	}

	ctx = context.WithValue(ctx, app.TenantCtxKey, currentTenant)
	ctx = context.WithValue(ctx, app.UserCtxKey, currentViewer)

	return ctx, target.AllowedActions(currentViewer, currentTenant), nil
}
