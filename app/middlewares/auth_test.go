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

func TestIsAuthorized_WithAllowedRole(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.IsAuthorized(enum.RoleAdministrator, enum.RoleCollaborator))
	status, _ := server.AsUser(mock.JonSnow).Execute(func(c *web.Context) error {
		return c.NoContent(http.StatusOK)
	})

	Expect(status).Equals(http.StatusOK)
}

func TestIsAuthorized_WithForbiddenRole(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()
	server.Use(middlewares.IsAuthorized(enum.RoleAdministrator, enum.RoleCollaborator))
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
				Use(middlewares.IsAuthorized(enum.RoleAdministrator))

			if test.role != 0 {
				server.AsUser(&entity.User{ID: 7, Name: "Reviewer", Role: test.role})
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
