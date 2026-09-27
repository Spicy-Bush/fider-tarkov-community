package postgres

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func userPermissionsForUpdate(trx *dbx.Trx, tenant *entity.Tenant, viewer *entity.User, userID int) (entity.UserPermissions, error) {
	var target struct {
		ID     int             `db:"id"`
		Role   enum.Role       `db:"role"`
		Status enum.UserStatus `db:"status"`
	}

	if err := trx.Get(&target, `
		SELECT id, role, status FROM users
		WHERE id = $1 AND tenant_id = $2 AND status <> $3
		FOR UPDATE`, userID, tenant.ID, enum.UserDeleted); err != nil {
		return entity.UserPermissions{}, err
	}

	user := &entity.User{ID: target.ID, Role: target.Role, Status: target.Status}
	return user.AllowedActions(viewer, tenant), nil
}
