package entity_test

import (
	"slices"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestResponseConfigurationOwnsAllowedStatuses(t *testing.T) {
	admin := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}
	for _, role := range entity.PermissionRoles {
		for _, status := range entity.ResponseOptions {
			tenant := &entity.Tenant{Status: enum.TenantActive, RolePostResponses: entity.RolePostResponses{role: {}}}
			permissions, responses, blocked := tenant.ApplyRoleConfiguration(admin, nil, []entity.RoleResponseChange{{Role: role, Status: status, Granted: true}})
			if blocked != "" {
				t.Fatalf("grant %v/%v: %s", role, status, blocked)
			}
			tenant.RolePermissions, tenant.RolePostResponses = permissions, responses
			user := &entity.User{ID: 2, Role: role, Status: enum.UserActive}
			if got := entity.AllowedPostResponses(user, tenant); !slices.Equal(got, []enum.PostStatus{status}) || !entity.Can(user, tenant, entity.RespondToPosts) {
				t.Fatalf("grant %v/%v returned %v", role, status, got)
			}
			permissions, responses, blocked = tenant.ApplyRoleConfiguration(admin, nil, []entity.RoleResponseChange{{Role: role, Status: status, Granted: false}})
			if blocked != "" {
				t.Fatal(blocked)
			}
			tenant.RolePermissions, tenant.RolePostResponses = permissions, responses
			if entity.Can(user, tenant, entity.RespondToPosts) || len(entity.AllowedPostResponses(user, tenant)) != 0 {
				t.Fatal("empty response selection retained response authority")
			}
		}
	}
}

func TestResponseConfigurationDependencyAndDelegation(t *testing.T) {
	admin := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}
	tenant := &entity.Tenant{Status: enum.TenantActive}
	permissions, responses, blocked := tenant.ApplyRoleConfiguration(admin,
		[]entity.RolePermissionChange{{Role: enum.RoleModerator, Permission: entity.ReadResponses, Granted: false}}, nil)
	if blocked != "" || permissions.Grants(enum.RoleModerator, entity.ReadResponses) || len(responses.ForRole(enum.RoleModerator)) != 0 {
		t.Fatalf("dependency revoke: permissions=%v responses=%v blocked=%s", permissions, responses, blocked)
	}
	permissions, responses, blocked = tenant.ApplyRoleConfiguration(admin,
		[]entity.RolePermissionChange{{Role: enum.RoleModerator, Permission: entity.ReadResponses, Granted: false}},
		[]entity.RoleResponseChange{{Role: enum.RoleModerator, Status: enum.PostPlanned, Granted: true}})
	if blocked != "" || !permissions.Grants(enum.RoleModerator, entity.ReadResponses) || !slices.Equal(responses.ForRole(enum.RoleModerator), []enum.PostStatus{enum.PostPlanned}) {
		t.Fatalf("explicit grant after revoke: permissions=%v responses=%v blocked=%s", permissions, responses, blocked)
	}

	permissions, responses, blocked = tenant.ApplyRoleConfiguration(admin,
		[]entity.RolePermissionChange{
			{Role: enum.RoleModerator, Permission: entity.ReadResponses, Granted: false},
			{Role: enum.RoleModerator, Permission: entity.ManageResponses, Granted: true},
		}, nil)
	if blocked != "" || !permissions.Grants(enum.RoleModerator, entity.ReadResponses) || len(responses.ForRole(enum.RoleModerator)) != 0 {
		t.Fatalf("a restored requirement restored revoked response choices: permissions=%v responses=%v blocked=%s", permissions, responses, blocked)
	}

	moderator := &entity.User{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive}
	permissions, responses, blocked = tenant.ApplyRoleConfiguration(moderator,
		[]entity.RolePermissionChange{{Role: enum.RoleVisitor, Permission: entity.ReadResponses, Granted: true}},
		[]entity.RoleResponseChange{{Role: enum.RoleVisitor, Status: enum.PostPlanned, Granted: true}})
	if blocked == "" || permissions != nil || responses != nil || tenant.RolePermissions != nil || tenant.RolePostResponses != nil {
		t.Fatal("unowned response partially changed unrelated permission cells")
	}
	_, responses, blocked = tenant.ApplyRoleConfiguration(moderator, nil,
		[]entity.RoleResponseChange{{Role: enum.RoleVisitor, Status: enum.PostDuplicate, Granted: true}})
	if blocked != "" || !slices.Equal(responses.ForRole(enum.RoleVisitor), []enum.PostStatus{enum.PostDuplicate}) {
		t.Fatalf("permitted response delegation failed: %s %v", blocked, responses)
	}
	_, _, blocked = tenant.ApplyRoleConfiguration(moderator, nil,
		[]entity.RoleResponseChange{{Role: enum.RoleAdministrator, Status: enum.PostDuplicate, Granted: false}})
	if blocked == "" {
		t.Fatal("lower role changed administrator response choices")
	}
}
