package handlers

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func Health() web.HandlerFunc {
	return func(c *web.Context) error {
		err := dbx.Ping()
		if err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{"status": "Healthy"})
	}
}

func UpdateMessageBanner() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(cmd.UpdateMessageBanner)
		err := c.Bind(action)
		if err != nil {
			return c.Failure(err)
		}
		action.TenantID = c.Tenant().ID
		err = bus.Dispatch(c, action)
		if err != nil {
			return c.Failure(err)
		}
		middlewares.InvalidateTenantCache()
		return c.Ok(web.Map{})
	}
}

func LegalPage(title, file string) web.HandlerFunc {
	content, err := fs.ReadFile(assets.FS, "etc/"+file)
	if err != nil {
		return func(c *web.Context) error {
			return c.NotFound()
		}
	}

	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Legal/Legal.page",
			Title: title,
			Data: web.Map{
				"content": string(content),
			},
		})
	}
}

func Sitemap() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.Tenant().IsPrivate {
			return c.NotFound()
		}

		if env.Config.Environment == "development" {
			return c.NotFound()
		}

		allPosts := &query.GetAllPosts{}
		if err := bus.Dispatch(c, allPosts); err != nil {
			return c.Failure(err)
		}

		baseURL := c.BaseURL()
		text := strings.Builder{}
		text.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		text.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		text.WriteString(fmt.Sprintf("<url> <loc>%s</loc> </url>", baseURL))
		for _, post := range allPosts.Result {
			text.WriteString(fmt.Sprintf("<url> <loc>%s/posts/%d/%s</loc> </url>", baseURL, post.Number, post.Slug))
		}
		text.WriteString(`</urlset>`)

		c.Response.Header().Del("Content-Security-Policy")
		return c.XML(http.StatusOK, text.String())
	}
}

func RobotsTXT() web.HandlerFunc {
	robotsFile := "robots.txt"
	if env.IsDevelopment() {
		robotsFile = "robots-dev.txt"
	}

	robotsBytes, err := fs.ReadFile(assets.FS, robotsFile)
	if err != nil {
		return func(c *web.Context) error {
			return c.NotFound()
		}
	}

	return func(c *web.Context) error {
		sitemapURL := c.BaseURL() + "/sitemap.xml"
		content := fmt.Sprintf("%s\nSitemap: %s", string(robotsBytes), sitemapURL)
		return c.String(http.StatusOK, content)
	}
}

func Page(title, description, page string) web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:        page,
			Title:       title,
			Description: description,
		})
	}
}

type NewLogError struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func LogError() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(NewLogError)
		err := c.Bind(action)
		if err != nil {
			return c.Failure(err)
		}
		log.Warnf(c, action.Message, dto.Props{
			"Data": action.Data,
		})
		return c.Ok(web.Map{})
	}
}

func validateKey(kind enum.EmailVerificationKind, key string, c *web.Context) (*entity.EmailVerification, error) {
	findByKey := &query.GetVerificationByKey{Kind: kind, Key: key}
	err := bus.Dispatch(c, findByKey)
	if err != nil {
		if errors.Cause(err) == app.ErrNotFound {
			return nil, c.NotFound()
		}
		return nil, c.Failure(err)
	}

	now := time.Now()
	res := findByKey.Result

	if res.VerifiedAt != nil && now.Sub(*res.VerifiedAt) > 5*time.Minute {
		return nil, c.Gone()
	}

	if now.After(res.ExpiresAt) {
		err = bus.Dispatch(c, &cmd.SetKeyAsVerified{Key: key})
		if err != nil {
			return nil, c.Failure(err)
		}
		return nil, c.Gone()
	}

	return res, nil
}

func between(n, min, max int) int {
	if n > max {
		return max
	} else if n < min {
		return min
	}
	return n
}

var imageSizeBuckets = []int{0, 64, 100, 200, 512, 1500, 2000}

func snapToSize(size int, buckets []int) int {
	if size <= 0 {
		return 0
	}
	for _, b := range buckets {
		if b >= size {
			return b
		}
	}
	return buckets[len(buckets)-1]
}
