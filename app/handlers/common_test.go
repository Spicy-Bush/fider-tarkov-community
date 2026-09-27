package handlers_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
)

func TestHealthHandler(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	code, _ := server.
		Execute(handlers.Health())

	Expect(code).Equals(http.StatusOK)
}

func TestPageHandler(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	code, _ := server.
		Execute(handlers.Page("The Title", "The Description", "TheChunk.Page"))

	Expect(code).Equals(http.StatusOK)
}

func TestLegalPageHandler(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	code, _ := server.
		Execute(handlers.LegalPage("Terms of Service", "terms.md"))

	Expect(code).Equals(http.StatusOK)
}

func TestLegalPageHandler_Invalid(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	code, _ := server.
		Execute(handlers.LegalPage("Some Page", "somepage.md"))

	Expect(code).Equals(http.StatusNotFound)
}

func TestRobotsTXT(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	code, response := server.
		WithURL("https://demo.test.fider.io/robots.txt").
		Execute(handlers.RobotsTXT())
	content, _ := io.ReadAll(response.Body)
	Expect(code).Equals(http.StatusOK)
	Expect(string(content)).ContainsSubstring("User-agent: *")
	Expect(string(content)).ContainsSubstring("Disallow: /api/")
	Expect(string(content)).ContainsSubstring("Disallow: /admin/")
	Expect(string(content)).ContainsSubstring("Disallow: /oauth/")
	Expect(string(content)).ContainsSubstring("Disallow: /terms")
	Expect(string(content)).ContainsSubstring("Disallow: /privacy")
	Expect(string(content)).ContainsSubstring("Disallow: /_design")
	Expect(string(content)).ContainsSubstring("Sitemap: https://demo.test.fider.io/sitemap.xml")
}

func TestSitemap(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetAllPosts) error {
		q.Result = []*entity.Post{}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllPublishedPages) error {
		q.Result = []*entity.Page{}
		return nil
	})

	server := mock.NewServer()
	code, response := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io:3000/sitemap.xml").
		Execute(handlers.Sitemap())

	bytes, _ := io.ReadAll(response.Body)
	Expect(code).Equals(http.StatusOK)
	Expect(string(bytes)).Equals(`<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url> <loc>http://demo.test.fider.io:3000</loc> </url></urlset>`)
}

func TestSitemap_WithPostsAndPublicPages(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetAllPosts) error {
		q.Result = []*entity.Post{
			{Number: 1, Slug: "my-new-idea-1", Title: "My new idea 1"},
			{Number: 2, Slug: "the-other-idea", Title: "The other idea"},
		}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllPublishedPages) error {
		q.Result = []*entity.Page{
			{
				Slug:       "community-guide",
				Visibility: entity.PageVisibilityPublic,
				UpdatedAt:  time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
			},
			{Slug: "staff-guide", Visibility: entity.PageVisibilityPrivate},
			{Slug: "unlisted-guide", Visibility: entity.PageVisibilityUnlisted},
		}
		return nil
	})

	server := mock.NewServer()

	code, response := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io:3000/sitemap.xml").
		Execute(handlers.Sitemap())

	bytes, _ := io.ReadAll(response.Body)
	Expect(code).Equals(http.StatusOK)
	expected := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
		`<url> <loc>http://demo.test.fider.io:3000</loc> </url>` +
		`<url> <loc>http://demo.test.fider.io:3000/posts/1/my-new-idea-1</loc> </url>` +
		`<url> <loc>http://demo.test.fider.io:3000/posts/2/the-other-idea</loc> </url>` +
		`<url> <loc>http://demo.test.fider.io:3000/pages/community-guide</loc> <lastmod>2026-09-20</lastmod> </url>` +
		`</urlset>`
	Expect(string(bytes)).Equals(expected)
}

func TestSitemap_PrivateTenant_WithPosts(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()

	mock.DemoTenant.IsPrivate = true

	code, _ := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io:3000/sitemap.xml").
		Execute(handlers.Sitemap())

	Expect(code).Equals(http.StatusNotFound)
}
