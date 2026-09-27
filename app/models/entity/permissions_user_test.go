package entity_test

import (
	"slices"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestUserTargetPermissions(t *testing.T) {
	roles := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}
	ordinary := []enum.Role{enum.RoleVisitor, enum.RoleHelper}
	nonAdmins := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator}
	moderatedByCollaborator := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator}
	for _, test := range []struct {
		viewer   enum.Role
		read     []enum.Role
		edit     []enum.Role
		block    []enum.Role
		moderate []enum.Role
		remove   []enum.Role
		expire   []enum.Role
		role     []enum.Role
		visual   []enum.Role
	}{
		{viewer: enum.RoleVisitor},
		{viewer: enum.RoleHelper},
		{
			viewer:   enum.RoleModerator,
			read:     roles,
			edit:     moderatedByCollaborator,
			moderate: ordinary,
			expire:   ordinary,
		},
		{
			viewer:   enum.RoleCollaborator,
			read:     roles,
			edit:     roles,
			block:    ordinary,
			moderate: moderatedByCollaborator,
			remove:   moderatedByCollaborator,
			expire:   moderatedByCollaborator,
			visual:   nonAdmins,
		},
		{
			viewer:   enum.RoleAdministrator,
			read:     roles,
			edit:     roles,
			block:    nonAdmins,
			moderate: nonAdmins,
			remove:   nonAdmins,
			expire:   nonAdmins,
			role:     roles,
			visual:   roles,
		},
	} {
		for _, targetRole := range roles {
			t.Run(test.viewer.String()+"/"+targetRole.String(), func(t *testing.T) {
				viewer := &entity.User{ID: 1, Role: test.viewer, Status: enum.UserActive}
				target := &entity.User{ID: 2, Role: targetRole, Status: enum.UserActive}
				want := entity.UserPermissions{
					ReadProfile:      slices.Contains(test.read, targetRole),
					EditName:         slices.Contains(test.edit, targetRole),
					EditAvatar:       slices.Contains(test.edit, targetRole),
					Block:            slices.Contains(test.block, targetRole),
					Moderate:         slices.Contains(test.moderate, targetRole),
					DeleteModeration: slices.Contains(test.remove, targetRole),
					ExpireModeration: slices.Contains(test.expire, targetRole),
					ChangeRole:       slices.Contains(test.role, targetRole),
					ChangeVisualRole: slices.Contains(test.visual, targetRole),
				}
				for _, state := range []string{"active", "muted viewer", "blocked target"} {
					viewer.Muted = state == "muted viewer"
					if state == "blocked target" {
						target.Status = enum.UserBlocked
						want.Moderate = false
					}
					if got := target.AllowedActions(viewer, nil); got != want {
						t.Errorf("%s: got %+v, want %+v", state, got, want)
					}
				}
			})
		}
		t.Run(test.viewer.String()+"/self", func(t *testing.T) {
			user := &entity.User{ID: 1, Role: test.viewer, Status: enum.UserActive}
			want := entity.UserPermissions{
				ReadProfile:      true,
				EditName:         true,
				EditAvatar:       true,
				ChangeVisualRole: test.viewer == enum.RoleCollaborator || test.viewer == enum.RoleAdministrator,
			}
			if got := user.AllowedActions(user, nil); got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestUserTargetPermissionsUnavailableViewerOrTarget(t *testing.T) {
	for _, test := range []struct {
		name   string
		viewer *entity.User
		target *entity.User
		tenant *entity.Tenant
	}{
		{
			name:   "anonymous",
			target: &entity.User{ID: 2, Status: enum.UserActive},
		},
		{
			name:   "missing target",
			viewer: &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive},
		},
		{
			name:   "blocked viewer",
			viewer: &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserBlocked},
			target: &entity.User{ID: 2, Status: enum.UserActive},
		},
		{
			name:   "deleted viewer",
			viewer: &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserDeleted},
			target: &entity.User{ID: 2, Status: enum.UserActive},
		},
		{
			name:   "deleted target",
			viewer: &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive},
			target: &entity.User{ID: 2, Status: enum.UserDeleted},
		},
		{
			name:   "locked self profile",
			viewer: &entity.User{ID: 1, Status: enum.UserActive},
			target: &entity.User{ID: 1, Status: enum.UserActive},
			tenant: &entity.Tenant{Status: enum.TenantLocked},
		},
		{
			name:   "disabled tenant",
			viewer: &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive},
			target: &entity.User{ID: 2, Status: enum.UserActive},
			tenant: &entity.Tenant{Status: enum.TenantDisabled},
		},
		{
			name:   "other tenant",
			viewer: &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive},
			target: &entity.User{ID: 2, Status: enum.UserActive, Tenant: &entity.Tenant{ID: 2}},
			tenant: &entity.Tenant{ID: 1},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.target.AllowedActions(test.viewer, test.tenant); got != (entity.UserPermissions{}) {
				t.Errorf("unavailable user has actions: %+v", got)
			}
		})
	}
}
