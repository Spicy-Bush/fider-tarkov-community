package web_test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func useReactTestAssets(t *testing.T) {
	t.Helper()
	previous := assets.FS
	assets.FS = os.DirFS("../../..")
	t.Cleanup(func() { assets.FS = previous })
}

func TestReactRenderer_FileNotFound(t *testing.T) {
	useReactTestAssets(t)
	r, err := web.NewReactRenderer("unknown.js")
	if err == nil || r != nil {
		t.Fatalf("expected missing script error, got renderer %v and error %v", r, err)
	}
}

func TestReactRenderer_EmptyFile(t *testing.T) {
	useReactTestAssets(t)
	r, err := web.NewReactRenderer("app/pkg/web/testdata/empty.js")
	if err != nil {
		t.Fatal(err)
	}

	u, _ := url.Parse("https://demo.test.fider.io")
	html, err := r.Render(u, web.Map{})
	if err != nil || html != "" {
		t.Fatalf("expected empty output, got %q and error %v", html, err)
	}
}

// Running this in Node would hide accidental dependencies on Node built ins
func TestReactRenderer_RenderPages(t *testing.T) {
	useReactTestAssets(t)
	r, err := web.NewReactRenderer("ssr.js")
	if err != nil {
		t.Fatalf("build the SSR bundle with make build-ssr before testing: %v", err)
	}

	homeProps := web.Map{
		"posts":          []web.Map{},
		"tags":           []web.Map{},
		"countPerStatus": web.Map{},
	}
	tests := []struct {
		name, page, locale string
		props              web.Map
		want               []string
	}{
		{
			name: "English home", page: "Home/Home.page", locale: "en", props: homeProps,
			want: []string{`id="p-home"`, "What can we do better? This is the place for you to vote, discuss and share ideas.", "No posts have been created yet."},
		},
		{
			name: "Portuguese home", page: "Home/Home.page", locale: "pt-BR", props: homeProps,
			want: []string{`id="p-home"`, "O que podemos fazer melhor? Este é o lugar para você votar, discutir e compartilhar ideias.", "Nenhuma postagem foi criada ainda."},
		},
		{
			name: "Post with Unicode and markdown", page: "ShowPost/ShowPost.page", locale: "en",
			props: web.Map{
				"post": web.Map{
					"id": 1, "number": 1, "slug": "ssr-test",
					"title": "SSR café 日本語 🎯", "description": "SSR **markdown** content.",
					"createdAt": "2026-03-24T12:00:00Z", "lastActivityAt": "2026-03-24T12:00:00Z",
					"status": "open", "user": web.Map{"id": 1, "name": "SSR author", "role": "visitor"},
					"tags": []string{}, "votesCount": 0, "commentsCount": 0, "response": nil,
				},
				"comments": []web.Map{}, "tags": []web.Map{}, "votes": []web.Map{},
				"attachments": []string{}, "subscribed": false,
			},
			want: []string{"SSR café 日本語 🎯", "<strong>markdown</strong>", "SSR author"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("https://demo.test.fider.io")
			html, err := r.Render(u, web.Map{
				"page":     tt.page,
				"tenant":   &entity.Tenant{Locale: tt.locale},
				"settings": web.Map{"locale": tt.locale, "environment": "production"},
				"props":    tt.props,
			})
			if err != nil {
				t.Fatalf("SSR render failed: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered HTML does not contain %q", want)
				}
			}
		})
	}
}
