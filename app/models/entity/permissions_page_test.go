package entity_test

import (
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestPagePermissions(t *testing.T) {
	cases := []struct {
		name         string
		role         enum.Role
		status       enum.UserStatus
		muted        bool
		tenantStatus enum.TenantStatus
		pageStatus   entity.PageStatus
		visibility   entity.PageVisibility
		allowReact   bool
		want         entity.PagePermissions
	}{
		{
			name:       "anonymous public",
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
		},
		{
			name:       "visitor public",
			role:       enum.RoleVisitor,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
			want:       entity.PagePermissions{React: true, Subscribe: true},
		},
		{
			name:       "muted visitor",
			role:       enum.RoleVisitor,
			status:     enum.UserActive,
			muted:      true,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
			want:       entity.PagePermissions{Subscribe: true},
		},
		{
			name:       "reactions disabled",
			role:       enum.RoleVisitor,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			want:       entity.PagePermissions{Subscribe: true},
		},
		{
			name:       "draft visitor",
			role:       enum.RoleVisitor,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusDraft,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
		},
		{
			name:       "private outsider",
			role:       enum.RoleVisitor,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPrivate,
			allowReact: true,
		},
		{
			name:       "private permitted helper",
			role:       enum.RoleHelper,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPrivate,
			allowReact: true,
			want:       entity.PagePermissions{React: true, Subscribe: true},
		},
		{
			name:       "moderator public",
			role:       enum.RoleModerator,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
			want:       entity.PagePermissions{React: true, Subscribe: true},
		},
		{
			name:       "collaborator draft",
			role:       enum.RoleCollaborator,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusDraft,
			visibility: entity.PageVisibilityPrivate,
			allowReact: true,
			want:       entity.PagePermissions{Edit: true, Delete: true, React: true, Subscribe: true},
		},
		{
			name:       "administrator public",
			role:       enum.RoleAdministrator,
			status:     enum.UserActive,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
			want:       entity.PagePermissions{Edit: true, Delete: true, React: true, Subscribe: true},
		},
		{
			name:       "blocked administrator",
			role:       enum.RoleAdministrator,
			status:     enum.UserBlocked,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
		},
		{
			name:       "deleted administrator",
			role:       enum.RoleAdministrator,
			status:     enum.UserDeleted,
			pageStatus: entity.PageStatusPublished,
			visibility: entity.PageVisibilityPublic,
			allowReact: true,
		},
		{
			name:         "locked tenant",
			role:         enum.RoleAdministrator,
			status:       enum.UserActive,
			tenantStatus: enum.TenantLocked,
			pageStatus:   entity.PageStatusPublished,
			visibility:   entity.PageVisibilityPublic,
			allowReact:   true,
		},
		{
			name:         "disabled tenant",
			role:         enum.RoleAdministrator,
			status:       enum.UserActive,
			tenantStatus: enum.TenantDisabled,
			pageStatus:   entity.PageStatusPublished,
			visibility:   entity.PageVisibilityPublic,
			allowReact:   true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			page := &entity.Page{
				ID:             9,
				Status:         test.pageStatus,
				Visibility:     test.visibility,
				AllowedRoles:   []string{"helper"},
				AllowReactions: test.allowReact,
			}
			tenant := &entity.Tenant{ID: 1, Status: test.tenantStatus}
			var user *entity.User
			if test.role != 0 {
				user = &entity.User{ID: 4, Role: test.role, Status: test.status, Muted: test.muted}
			}

			if got := page.AllowedActions(user, tenant); got != test.want {
				t.Fatalf("page permissions %+v; want %+v", got, test.want)
			}
		})
	}
}
