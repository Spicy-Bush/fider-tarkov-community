package middlewares_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestCache_StatusOK(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.ClientCache(5 * time.Minute))
	handler := func(c *web.Context) error {
		return c.NoContent(http.StatusOK)
	}

	status, response := server.Execute(handler)

	Expect(status).Equals(http.StatusOK)
	Expect(response.Header().Get("Cache-Control")).Equals("public, max-age=300")
}

func TestCache_StatusNotFound(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.ClientCache(5 * time.Minute))
	handler := func(c *web.Context) error {
		return c.NotFound()
	}

	status, response := server.Execute(handler)

	Expect(status).Equals(http.StatusNotFound)
	Expect(response.Header().Get("Cache-Control")).Equals("no-cache, no-store")
}
