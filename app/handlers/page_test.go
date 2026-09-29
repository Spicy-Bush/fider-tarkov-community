package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestPageViewUsesTenantPermissions(t *testing.T) {
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.UserSubscribedToPage) error {
		return nil
	})

	for _, status := range []entity.PageStatus{entity.PageStatusDraft, entity.PageStatusPublished} {
		for _, candidate := range []struct {
			role    enum.Role
			granted bool
			status  int
		}{
			{enum.RoleCollaborator, false, http.StatusForbidden},
			{enum.RoleHelper, true, http.StatusOK},
		} {
			t.Run(string(status)+"/"+candidate.role.String(), func(t *testing.T) {
				tenant := &entity.Tenant{
					ID:              1,
					Status:          enum.TenantActive,
					RolePermissions: entity.RolePermissions{candidate.role: {entity.ManagePages: candidate.granted}},
				}
				user := &entity.User{ID: 1, Role: candidate.role, Status: enum.UserActive, Tenant: tenant}
				bus.AddHandler(func(ctx context.Context, q *query.GetPageBySlug) error {
					q.Result = &entity.Page{
						ID:         1,
						Title:      "Restricted Page",
						Slug:       "restricted",
						Status:     status,
						Visibility: entity.PageVisibilityPrivate,
					}
					return nil
				})

				server := mock.NewServer().OnTenant(tenant).AsUser(user).
					AddParam("slug", "restricted").AddHeader("Accept", web.PageDataContentType)
				code, _ := server.Execute(handlers.ViewPage())
				if code != candidate.status {
					t.Fatalf("response = %d, want %d", code, candidate.status)
				}
			})
		}
	}
}
