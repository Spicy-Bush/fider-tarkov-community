package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestPageVisibilitySQLMatchesPolicy(t *testing.T) {
	for _, role := range []enum.Role{0, enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		for _, granted := range []bool{false, true} {
			tenant := &entity.Tenant{
				ID: 1, Status: enum.TenantActive,
				RolePermissions: entity.RolePermissions{role: {entity.ManagePages: granted}},
			}
			var user *entity.User
			roleName := ""
			if role != 0 {
				user = &entity.User{ID: 1, Role: role, Status: enum.UserActive, Tenant: tenant}
				roleName = role.String()
			}

			for _, status := range []entity.PageStatus{entity.PageStatusDraft, entity.PageStatusPublished, entity.PageStatusUnpublished, entity.PageStatusScheduled} {
				for _, visibility := range []entity.PageVisibility{entity.PageVisibilityPublic, entity.PageVisibilityPrivate, entity.PageVisibilityUnlisted} {
					for _, allowedRoles := range [][]string{nil, {}, {""}, {"helper"}, {"visitor", "moderator"}} {
						page := &entity.Page{Status: status, Visibility: visibility, AllowedRoles: allowedRoles}
						encodedRoles, err := json.Marshal(allowedRoles)
						if err != nil {
							t.Fatal(err)
						}

						var visible bool
						err = dbx.Connection().QueryRow(`
							SELECT page_is_visible($1, $2, $3, $4, $5)
						`, status, visibility, string(encodedRoles), roleName, entity.Can(user, tenant, entity.ManagePages)).Scan(&visible)
						if err != nil || visible != page.CanView(user, tenant) {
							t.Fatalf("role=%s manage=%t status=%s visibility=%s allowed=%v: SQL=%t Go=%t error=%v",
								roleName, granted, status, visibility, allowedRoles, visible, page.CanView(user, tenant), err)
						}
					}
				}
			}
		}
	}
}

func TestPagePermissionsProjectionAndMutations(t *testing.T) {
	f := newPostWorkflow(t)
	create := &cmd.CreatePage{
		Title:          "Page permission projection",
		Slug:           "page-permission-projection",
		Content:        "A page with reactions",
		Status:         entity.PageStatusPublished,
		Visibility:     entity.PageVisibilityPublic,
		AllowReactions: true,
	}
	if err := bus.Dispatch(f.ctx, create); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name         string
		role         enum.Role
		status       enum.UserStatus
		tenantStatus enum.TenantStatus
		want         entity.PagePermissions
	}{
		{
			name:         "visitor",
			role:         enum.RoleVisitor,
			status:       enum.UserActive,
			tenantStatus: enum.TenantActive,
			want:         entity.PagePermissions{React: true, Subscribe: true},
		},
		{
			name:         "administrator",
			role:         enum.RoleAdministrator,
			status:       enum.UserActive,
			tenantStatus: enum.TenantActive,
			want:         entity.PagePermissions{Edit: true, Delete: true, React: true, Subscribe: true},
		},
		{
			name:         "blocked administrator",
			role:         enum.RoleAdministrator,
			status:       enum.UserBlocked,
			tenantStatus: enum.TenantActive,
		},
		{
			name:         "deleted administrator",
			role:         enum.RoleAdministrator,
			status:       enum.UserDeleted,
			tenantStatus: enum.TenantActive,
		},
		{
			name:         "locked tenant",
			role:         enum.RoleAdministrator,
			status:       enum.UserActive,
			tenantStatus: enum.TenantLocked,
		},
		{
			name:         "disabled tenant",
			role:         enum.RoleAdministrator,
			status:       enum.UserActive,
			tenantStatus: enum.TenantDisabled,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			user := *f.user
			user.Role = test.role
			user.Status = test.status
			tenant := *f.tenant
			tenant.Status = test.tenantStatus
			ctx := context.WithValue(f.ctx, app.UserCtxKey, &user)
			ctx = context.WithValue(ctx, app.TenantCtxKey, &tenant)

			byID := &query.GetPageByID{ID: create.Result.ID}
			bySlug := &query.GetPageBySlug{Slug: create.Slug}
			list := &query.ListPages{Limit: 20}
			if err := bus.Dispatch(ctx, byID, bySlug, list); err != nil {
				t.Fatal(err)
			}
			if len(list.Result) != 1 {
				t.Fatalf("listed %d pages; want one", len(list.Result))
			}
			for _, page := range []*entity.Page{byID.Result, bySlug.Result, list.Result[0]} {
				if page.Permissions != test.want {
					t.Fatalf("projected %+v; want %+v", page.Permissions, test.want)
				}
			}

			transaction, err := dbx.Connection().Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer transaction.Rollback()
			ctx = context.WithValue(ctx, app.TransactionCtxKey, transaction)
			react := &cmd.TogglePageReaction{Page: byID.Result, Emoji: "like"}
			if err := bus.Dispatch(ctx, react); (err == nil) != test.want.React {
				t.Fatalf("reaction error %v; allowed %v", err, test.want.React)
			}
			subscribe := &cmd.TogglePageSubscription{PageID: create.Result.ID}
			if err := bus.Dispatch(ctx, subscribe); (err == nil) != test.want.Subscribe {
				t.Fatalf("subscription error %v; allowed %v", err, test.want.Subscribe)
			}
		})
	}
}
