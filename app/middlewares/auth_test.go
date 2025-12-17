package middlewares_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
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
