package web_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func pageTestEngine() *web.Engine {
	bus.AddHandler(func(ctx context.Context, q *query.ListActiveOAuthProviders) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetNavigationLinks) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	return web.New()
}

func TestPageDataMatchesDocument(t *testing.T) {
	engine := pageTestEngine()
	serverData := regexp.MustCompile(`(?s)<script[^>]*id="server-data"[^>]*>(.*?)</script>`)
	props := web.Props{
		Page:        "Home/Home.page",
		Title:       "A <page>",
		Description: "Description\nwith a newline",
		Data:        web.Map{"title": "<script>alert(1)</script>", "count": 3},
	}

	for role := 0; role <= 5; role++ {
		request := httptest.NewRequest(http.MethodGet, "https://demo.test.fider.io/", nil)
		response := httptest.NewRecorder()
		ctx, err := web.NewContext(engine, request, response, nil)
		if err != nil {
			t.Fatal(err)
		}

		ctx.SetTenant(&entity.Tenant{ID: 1, Name: "Review", Locale: "en"})
		if role != 0 {
			ctx.SetUser(&entity.User{ID: 7, Name: "Reviewer", Role: enum.Role(role), Status: enum.UserActive, Muted: true})
		}

		if err := ctx.Page(http.StatusOK, props); err != nil {
			t.Fatal(err)
		}

		match := serverData.FindSubmatch(response.Body.Bytes())
		if len(match) != 2 {
			t.Fatal("document did not contain its server data")
		}

		var document map[string]any
		if err := json.Unmarshal(match[1], &document); err != nil {
			t.Fatal(err)
		}

		response = httptest.NewRecorder()
		ctx.Response.Writer = response
		request.Header.Set("Accept", web.PageDataContentType)
		if err := ctx.Page(http.StatusOK, props); err != nil {
			t.Fatal(err)
		}

		var data map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(document, data) {
			t.Fatalf("role %d: navigation differs from the document", role)
		}
		if data["settings"].(map[string]any)["version"] == nil {
			t.Fatal("page data omitted its UI build identity")
		}
		if role != 0 && data["user"].(map[string]any)["isMuted"] != true {
			t.Fatal("page data discarded the authenticated user's mute status")
		}
		if response.Header().Get("Content-Type") != web.UTF8JSONContentType || response.Header().Get("Vary") != "Accept" {
			t.Fatalf("unexpected representation headers: %v", response.Header())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("personalized page data must not be cached")
		}
	}
}

func TestPageDataVersionTracksAssets(t *testing.T) {
	original := assets.FS
	t.Cleanup(func() { assets.FS = original })
	ssr, err := fs.ReadFile(original, "ssr.js")
	if err != nil {
		t.Fatal(err)
	}

	versionFor := func(script string) string {
		t.Helper()
		manifest, err := json.Marshal(map[string]any{
			"public/index.tsx": map[string]any{"file": script, "isEntry": true},
		})
		if err != nil {
			t.Fatal(err)
		}
		assets.FS = fstest.MapFS{
			"ssr.js":                   &fstest.MapFile{Data: ssr},
			"dist/.vite/manifest.json": &fstest.MapFile{Data: manifest},
		}

		request := httptest.NewRequest(http.MethodGet, "https://demo.test.fider.io/", nil)
		request.Header.Set("Accept", web.PageDataContentType)
		response := httptest.NewRecorder()
		ctx, err := web.NewContext(pageTestEngine(), request, response, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ctx.Page(http.StatusOK, web.Props{Page: "Home/Home.page"}); err != nil {
			t.Fatal(err)
		}

		var data struct {
			Settings struct {
				Version string `json:"version"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}

		return data.Settings.Version
	}

	first := versionFor("js/main.first.js")
	if first == "" || first != versionFor("js/main.first.js") {
		t.Fatal("identical builds did not share their UI identity")
	}
	if first == versionFor("js/main.second.js") {
		t.Fatal("a changed UI build kept its old identity")
	}
}

func TestPageDataPreservesErrorStatus(t *testing.T) {
	engine := pageTestEngine()
	for _, status := range []int{401, 403, 404, 410, 500} {
		request := httptest.NewRequest(http.MethodGet, "https://demo.test.fider.io/", nil)
		request.Header.Set("Accept", web.PageDataContentType)
		response := httptest.NewRecorder()
		ctx, err := web.NewContext(engine, request, response, nil)
		if err != nil {
			t.Fatal(err)
		}

		if err := ctx.Page(status, web.Props{Page: "Error/Error403.page"}); err != nil {
			t.Fatal(err)
		}

		if response.Code != status || !strings.Contains(response.Body.String(), `"page":"Error/Error403.page"`) {
			t.Fatalf("status %d: got %d %s", status, response.Code, response.Body)
		}
	}
}

func BenchmarkPageRepresentation(b *testing.B) {
	engine := pageTestEngine()
	props := web.Props{Page: "Home/Home.page", Data: web.Map{"description": strings.Repeat("A post description. ", 1000)}}

	for _, accept := range []string{"text/html", web.PageDataContentType} {
		b.Run(accept, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				request := httptest.NewRequest(http.MethodGet, "https://demo.test.fider.io/", nil)
				request.Header.Set("Accept", accept)
				response := httptest.NewRecorder()
				ctx, err := web.NewContext(engine, request, response, nil)
				if err != nil {
					b.Fatal(err)
				}

				if err := ctx.Page(http.StatusOK, props); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
