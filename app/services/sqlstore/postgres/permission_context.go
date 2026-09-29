package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

// The tenant lock precedes the actor lock so waiting cannot retain revoked authority.
func lockPermissionActor(trx *dbx.Trx, tenantID int, user *entity.User) (*entity.User, error) {
	if user == nil {
		return nil, validate.Unauthorized()
	}

	var stored struct {
		Role   enum.Role       `db:"role"`
		Status enum.UserStatus `db:"status"`
	}
	if err := trx.Get(&stored, `
		SELECT role, status FROM users WHERE tenant_id=$1 AND id=$2 FOR SHARE
	`, tenantID, user.ID); err != nil {
		if errors.Cause(err) == app.ErrNotFound {
			return nil, validate.Unauthorized()
		}
		return nil, err
	}

	current := *user
	current.Role = stored.Role
	current.Status = stored.Status

	return &current, nil
}

func permissionContext(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User, permission entity.Permission) (context.Context, error) {
	ctx, err := lockedPermissionContext(ctx, trx, tenant, user)
	if err != nil {
		return nil, err
	}

	currentUser := ctx.Value(app.UserCtxKey).(*entity.User)
	currentTenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if !entity.Can(currentUser, currentTenant, permission) {
		return nil, validate.Unauthorized()
	}

	return ctx, nil
}

func lockedPermissionContext(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) (context.Context, error) {
	if tenant == nil || user == nil {
		return nil, validate.Unauthorized()
	}

	currentTenant, err := lockPermissionTenant(trx, tenant)
	if err != nil {
		return nil, err
	}

	currentUser, err := lockPermissionActor(trx, tenant.ID, user)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, app.TenantCtxKey, currentTenant)
	return context.WithValue(ctx, app.UserCtxKey, currentUser), nil
}

func lockPermissionTenant(trx *dbx.Trx, tenant *entity.Tenant) (*entity.Tenant, error) {
	var stored struct {
		Status      enum.TenantStatus `db:"status"`
		Permissions string            `db:"role_permissions"`
		Responses   string            `db:"role_post_responses"`
	}
	if err := trx.Get(&stored, `
		SELECT status, role_permissions, role_post_responses FROM tenants WHERE id=$1 FOR SHARE
	`, tenant.ID); err != nil {
		return nil, err
	}

	permissions, err := parseRolePermissions(stored.Permissions)
	if err != nil {
		return nil, err
	}

	currentTenant := *tenant
	currentTenant.Status = stored.Status
	currentTenant.RolePermissions = permissions
	if currentTenant.RolePostResponses, err = parseRolePostResponses(stored.Responses); err != nil {
		return nil, err
	}

	return &currentTenant, nil
}
