package web_test

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func compareRendererResponse(buf *bytes.Buffer, fileName string, ctx *web.Context) {
	// os.WriteFile(env.Path(fileName), []byte(strings.Replace(buf.String(), ctx.ContextID(), "CONTEXT_ID", -1)), 0744)
	bytes, err := os.ReadFile(env.Path(fileName))
	Expect(err).IsNil()
	Expect(strings.Replace(buf.String(), ctx.ContextID(), "CONTEXT_ID", -1)).Equals(string(bytes))
}

func TestRenderer_Basic(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{}, ctx)
	compareRendererResponse(buf, "/app/pkg/web/testdata/basic.html", ctx)
}

func TestRenderer_WithChunkPreload(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{Page: "Test.page"}, ctx)
	compareRendererResponse(buf, "/app/pkg/web/testdata/chunk.html", ctx)
}

func TestRenderer_Tenant(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	ctx.SetTenant(&entity.Tenant{Name: "Game of Thrones"})
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{}, ctx)
	compareRendererResponse(buf, "/app/pkg/web/testdata/tenant.html", ctx)
}

func TestRenderer_WithCanonicalURL(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	ctx.SetCanonicalURL("http://feedback.demo.org")
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{}, ctx)
	compareRendererResponse(buf, "/app/pkg/web/testdata/canonical.html", ctx)
}

func TestRenderer_Home(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{
		Title:       "My Page Title",
		Description: "My Page Description",
		Data: web.Map{
			"posts":          make([]web.Map, 0),
			"tags":           make([]web.Map, 0),
			"countPerStatus": web.Map{},
		},
	}, ctx)

	compareRendererResponse(buf, "/app/pkg/web/testdata/home.html", ctx)
}

func TestRenderer_Home_SSR(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", map[string]string{
		"User-Agent": "Googlebot",
	})
	ctx.SetTenant(&entity.Tenant{
		Locale: "en",
	})
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{
		Page:        "Test.page",
		Title:       "My Page Title",
		Description: "My Page Description",
		Data: web.Map{
			"posts":          make([]web.Map, 0),
			"tags":           make([]web.Map, 0),
			"countPerStatus": web.Map{},
		},
	}, ctx)

	compareRendererResponse(buf, "/app/pkg/web/testdata/home_ssr.html", ctx)
}

func TestRenderer_AuthenticatedUser(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	ctx.SetUser(&entity.User{
		ID:         5,
		Name:       "Jon Snow",
		Email:      "jon.snow@got.com",
		Status:     enum.UserActive,
		Role:       enum.RoleAdministrator,
		AvatarType: enum.AvatarTypeGravatar,
		AvatarURL:  "https://demo.test.fider.io:3000/static/avatars/gravatar/5/Jon%20Snow",
	})
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{
		Title:       "My Page Title",
		Description: "My Page Description",
	}, ctx)

	compareRendererResponse(buf, "/app/pkg/web/testdata/user.html", ctx)
}

func TestRenderer_WithOAuth(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		q.Result = []*dto.OAuthProviderOption{
			{
				Provider:         app.GoogleProvider,
				DisplayName:      "Google",
				ClientID:         "1234",
				URL:              "https://demo.test.fider.io:3000/oauth/google",
				CallbackURL:      "https://demo.test.fider.io:3000/oauth/google/callback",
				LogoBlobKey:      "google.png",
				IsCustomProvider: false,
				IsEnabled:        true,
			},
		}
		return nil
	})

	buf := new(bytes.Buffer)
	ctx := newGetContext("https://demo.test.fider.io:3000/", nil)
	renderer := web.NewRenderer()
	renderer.Render(buf, http.StatusOK, web.Props{}, ctx)
	compareRendererResponse(buf, "/app/pkg/web/testdata/oauth.html", ctx)
}
