package middlewares_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestRequirePermission_WithAllowedRole(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.RequirePermission(entity.ManagePages))
	status, _ := server.AsUser(mock.JonSnow).Execute(func(c *web.Context) error {
		return c.NoContent(http.StatusOK)
	})

	Expect(status).Equals(http.StatusOK)
}

func TestRequirePermission_WithForbiddenRole(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()
	server.Use(middlewares.RequirePermission(entity.ManagePages))
	status, _ := server.AsUser(mock.AryaStark).Execute(func(c *web.Context) error {
		return c.NoContent(http.StatusOK)
	})

	Expect(status).Equals(http.StatusForbidden)
}

func TestIsAuthenticated_WithUser(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.IsAuthenticated())
	status, _ := server.AsUser(mock.AryaStark).Execute(func(c *web.Context) error {
		return c.NoContent(http.StatusOK)
	})

	Expect(status).Equals(http.StatusOK)
}

func TestIsAuthenticated_WithoutUser(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.IsAuthenticated())

	status, _ := server.Execute(func(c *web.Context) error {
		return c.NoContent(http.StatusOK)
	})

	Expect(status).Equals(http.StatusUnauthorized)
}

func TestPageDataRequiresAdministrator(t *testing.T) {
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	cases := []struct {
		name   string
		role   enum.Role
		status int
	}{
		{name: "anonymous", status: http.StatusUnauthorized},
		{name: "visitor", role: enum.RoleVisitor, status: http.StatusForbidden},
		{name: "helper", role: enum.RoleHelper, status: http.StatusForbidden},
		{name: "moderator", role: enum.RoleModerator, status: http.StatusForbidden},
		{name: "collaborator", role: enum.RoleCollaborator, status: http.StatusForbidden},
		{name: "administrator", role: enum.RoleAdministrator, status: http.StatusOK},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := mock.NewServer().
				AddHeader("Accept", web.PageDataContentType).
				Use(middlewares.IsAuthenticated()).
				Use(middlewares.RequirePermission(entity.ManageSettings))

			if test.role != 0 {
				server.AsUser(&entity.User{ID: 7, Name: "Reviewer", Role: test.role, Status: enum.UserActive})
			}

			called := false
			status, response := server.Execute(func(c *web.Context) error {
				called = true
				return c.Page(http.StatusOK, web.Props{
					Page: "Administration/General.page",
					Data: web.Map{"private": "administrator settings"},
				})
			})

			if status != test.status || called != (test.status == http.StatusOK) {
				t.Fatalf("status %d, handler called %v; wanted %d", status, called, test.status)
			}

			var data struct {
				Props map[string]any `json:"props"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}

			if _, containsSettings := data.Props["private"]; containsSettings != called {
				t.Fatalf("private settings present: %v, authorized: %v", containsSettings, called)
			}
		})
	}
}

func TestRequirePermission_AgreesWithPageProjection(t *testing.T) {
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	features := []struct {
		permission entity.Permission
		roles      []enum.Role
	}{
		{entity.ManageSettings, []enum.Role{enum.RoleAdministrator}},
		{entity.ManagePages, []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator}},
		{entity.ManageReports, []enum.Role{enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}},
		{entity.ManageQueue, []enum.Role{enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}},
		{entity.Permission("unknown"), nil},
	}
	viewers := []struct {
		name   string
		role   enum.Role
		status enum.UserStatus
	}{
		{name: "anonymous"},
		{name: "visitor", role: enum.RoleVisitor, status: enum.UserActive},
		{name: "helper", role: enum.RoleHelper, status: enum.UserActive},
		{name: "moderator", role: enum.RoleModerator, status: enum.UserActive},
		{name: "collaborator", role: enum.RoleCollaborator, status: enum.UserActive},
		{name: "administrator", role: enum.RoleAdministrator, status: enum.UserActive},
		{name: "zero-status administrator", role: enum.RoleAdministrator},
		{name: "unknown-status administrator", role: enum.RoleAdministrator, status: enum.UserStatus(99)},
		{name: "blocked administrator", role: enum.RoleAdministrator, status: enum.UserBlocked},
		{name: "deleted administrator", role: enum.RoleAdministrator, status: enum.UserDeleted},
	}

	for _, feature := range features {
		for _, viewer := range viewers {
			for _, tenantStatus := range []enum.TenantStatus{enum.TenantActive, enum.TenantLocked, enum.TenantDisabled} {
				t.Run(string(feature.permission)+"/"+viewer.name+"/"+tenantStatus.String(), func(t *testing.T) {
					tenant := &entity.Tenant{ID: 1, Name: "Permissions", Status: tenantStatus}
					var user *entity.User
					if viewer.role != 0 {
						user = &entity.User{ID: 7, Name: "Viewer", Role: viewer.role, Status: viewer.status}
					}

					allowed := false
					for _, role := range feature.roles {
						if viewer.role == role && viewer.status == enum.UserActive && tenantStatus == enum.TenantActive {
							allowed = true
						}
					}

					server := mock.NewServer().OnTenant(tenant).
						AddHeader("Accept", web.PageDataContentType).
						Use(middlewares.RequirePermission(feature.permission))
					if user != nil {
						server.AsUser(user)
					}
					called := false
					status, _ := server.Execute(func(c *web.Context) error {
						called = true
						return c.NoContent(http.StatusOK)
					})

					wantStatus := http.StatusForbidden
					if user == nil {
						wantStatus = http.StatusUnauthorized
					} else if allowed {
						wantStatus = http.StatusOK
					}
					if status != wantStatus || called != allowed {
						t.Fatalf("status %d, handler called %v; want %d, %v", status, called, wantStatus, allowed)
					}

					projection := mock.NewServer().OnTenant(tenant).
						AddHeader("Accept", web.PageDataContentType)
					if user != nil {
						projection.AsUser(user)
					}
					_, response := projection.Execute(func(c *web.Context) error {
						return c.Page(http.StatusOK, web.Props{Page: "Home/Index.page"})
					})
					var page struct {
						Permissions map[entity.Permission]bool `json:"permissions"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
						t.Fatal(err)
					}
					if page.Permissions[feature.permission] != allowed {
						t.Fatalf("projected permission %v; want %v", page.Permissions[feature.permission], allowed)
					}
				})
			}
		}
	}
}
