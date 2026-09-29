package web_test

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestReactRenderer_TreatsRequestURLAsData(t *testing.T) {
	previous := assets.FS
	assets.FS = fstest.MapFS{
		"renderer.js": &fstest.MapFile{Data: []byte(`function ssrRender(url) { return url; }`)},
	}
	t.Cleanup(func() { assets.FS = previous })

	renderer, err := web.NewReactRenderer("renderer.js")
	if err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{
		`https://demo.test.fider.io/?value="+("evaluated")+"`,
		`https://demo.test.fider.io/?value=\&text=日本語`,
		`https://demo.test.fider.io/?value=</script>`,
	} {
		requestURL, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}

		rendered, err := renderer.Render(requestURL, web.Map{})
		if err != nil {
			t.Fatal(err)
		}

		if rendered != requestURL.String() {
			t.Fatalf("request URL changed during rendering: got %q, want %q", rendered, requestURL.String())
		}
	}
}

func TestReactRenderer_RendersUntrustedMarkdownWithoutBrowserAPIs(t *testing.T) {
	useReactTestAssets(t)
	renderer, err := web.NewReactRenderer("ssr.js")
	if err != nil {
		t.Fatal(err)
	}
	requestURL, _ := url.Parse("https://demo.test.fider.io/posts/1/security")
	tenant := &entity.Tenant{ID: 1, Locale: "en", Status: enum.TenantActive}
	author := &entity.User{ID: 1, Name: "Author", Role: enum.RoleVisitor, Status: enum.UserActive}
	description := strings.Join([]string{
		`@{"name":"\u003cimg src=x onerror=globalThis.auditMarker=1\u003e"}`,
		`[**formatted**](/posts/2 'title @{"name":"Alice"}')`,
		`[unsafe](javascript&colon;alert(1))`,
		`![image](/static/images/example.png '" onload="alert(1)')`,
		`[video](https://www.youtube.com/watch?v=abc_123-xyz&t=30s)`,
		`[video](https://vk.com/video-123_456?t=30)`,
		`[malformed](https://youtube.com/watch?v=%E0%A4%A)`,
	}, "\n\n")
	post := &entity.Post{
		ID:          1,
		Number:      1,
		Title:       "Markdown security",
		Slug:        "security",
		Description: description,
		CreatedAt:   time.Now(),
		Status:      enum.PostOpen,
		User:        author,
		Tags:        []string{},
	}
	author.Permissions = author.AllowedActions(nil, tenant)
	post.Permissions = post.AllowedActions(nil, tenant, post.CreatedAt)
	post.DiscussionPermissions = entity.PostDiscussion(post).Permissions(nil, tenant)

	html, err := renderer.Render(requestURL, web.Map{
		"page":        "ShowPost/ShowPost.page",
		"tenant":      tenant,
		"settings":    web.Map{"locale": "en", "environment": "production"},
		"user":        nil,
		"permissions": entity.PermissionsFor(nil, tenant),
		"props": web.Map{
			"post":        post,
			"comments":    []web.Map{},
			"tags":        []web.Map{},
			"votes":       []web.Map{},
			"attachments": []string{},
			"subscribed":  false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, unsafe := range []string{`<img src=x`, ` onload="`, `href="javascript:`} {
		if strings.Contains(html, unsafe) {
			t.Errorf("SSR rendered unsafe Markdown fragment %q", unsafe)
		}
	}

	for _, expected := range []string{
		`@&lt;img src=x onerror=globalThis.auditMarker=1&gt;`,
		`<strong>formatted</strong>`,
		`title="title @{&quot;name&quot;:&quot;Alice&quot;}"`,
		`src="https://www.youtube.com/embed/abc_123-xyz?start=30"`,
		`src="https://vk.com/video_ext.php?oid=-123&amp;id=456&amp;t=30"`,
		`>malformed</a>`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("SSR omitted Markdown fragment %q", expected)
		}
	}
}

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
	postAuthor := &entity.User{
		ID:     1,
		Name:   "SSR author",
		Role:   enum.RoleVisitor,
		Status: enum.UserActive,
	}
	post := &entity.Post{
		ID:             1,
		Number:         1,
		Slug:           "ssr-test",
		Title:          "SSR café 日本語 🎯",
		Description:    "SSR **markdown** content.",
		CreatedAt:      time.Date(2026, time.March, 24, 12, 0, 0, 0, time.UTC),
		LastActivityAt: time.Date(2026, time.March, 24, 12, 0, 0, 0, time.UTC),
		Status:         enum.PostOpen,
		User:           postAuthor,
		Tags:           []string{},
	}

	tests := []struct {
		name   string
		page   string
		locale string
		props  web.Map
		user   *entity.User
		want   []string
	}{
		{
			name: "English home", page: "Home/Home.page", locale: "en", props: homeProps,
			want: []string{`id="p-home"`, "What can we do better? This is the place for you to vote, discuss and share ideas.", "No posts have been created yet."},
		},
		{
			name: "Authenticated home", page: "Home/Home.page", locale: "en", props: homeProps,
			user: &entity.User{ID: 1, Name: "SSR user", Role: enum.RoleAdministrator, Status: enum.UserActive},
			want: []string{`id="p-home"`, `id="input-title"`},
		},
		{
			name: "Portuguese home", page: "Home/Home.page", locale: "pt-BR", props: homeProps,
			want: []string{`id="p-home"`, "O que podemos fazer melhor? Este é o lugar para você votar, discutir e compartilhar ideias.", "Nenhuma postagem foi criada ainda."},
		},
		{
			name: "Post with Unicode and markdown", page: "ShowPost/ShowPost.page", locale: "en",
			props: web.Map{
				"post": post,
				"comments": []web.Map{}, "tags": []web.Map{}, "votes": []web.Map{},
				"attachments": []string{}, "subscribed": false,
			},
			want: []string{"SSR café 日本語 🎯", "<strong>markdown</strong>", "SSR author"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("https://demo.test.fider.io")
			tenant := &entity.Tenant{ID: 1, Locale: tt.locale, Status: enum.TenantActive}
			if tt.user != nil {
				tt.user.Permissions = tt.user.AllowedActions(tt.user, tenant)
			}
			if post, ok := tt.props["post"].(*entity.Post); ok {
				post.User.Permissions = post.User.AllowedActions(tt.user, tenant)
				post.Permissions = post.AllowedActions(tt.user, tenant, post.CreatedAt)
				post.DiscussionPermissions = entity.PostDiscussion(post).Permissions(tt.user, tenant)
			}

			html, err := r.Render(u, web.Map{
				"page":        tt.page,
				"tenant":      tenant,
				"settings":    web.Map{"locale": tt.locale, "environment": "production"},
				"props":       tt.props,
				"user":        tt.user,
				"permissions": entity.PermissionsFor(tt.user, tenant),
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
