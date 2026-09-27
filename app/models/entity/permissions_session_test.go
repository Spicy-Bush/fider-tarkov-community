package entity_test

import (
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestSessionPermissionsRequireActiveUser(t *testing.T) {
	tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive}
	for _, test := range []struct {
		name   string
		status enum.UserStatus
	}{
		{"zero", 0},
		{"unknown", 99},
		{"negative", -1},
		{"blocked", enum.UserBlocked},
		{"deleted", enum.UserDeleted},
	} {
		t.Run(test.name, func(t *testing.T) {
			user := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: test.status}
			for permission, allowed := range entity.PermissionsFor(user, tenant) {
				if allowed || entity.Can(user, tenant, permission) {
					t.Errorf("nonactive user granted %s", permission)
				}
			}
		})
	}

	active := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}
	if !entity.Can(active, tenant, entity.ManageSettings) {
		t.Fatal("active administrator lost settings permission")
	}
}
