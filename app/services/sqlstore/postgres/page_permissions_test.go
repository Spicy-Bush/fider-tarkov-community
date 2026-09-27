package postgres_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

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
