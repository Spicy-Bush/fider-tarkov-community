package handlers

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webpush"
)

var (
	serviceWorkerContent []byte
	serviceWorkerOnce    sync.Once
)

type webAppManifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description,omitempty"`
	StartURL        string         `json:"start_url"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Orientation     string         `json:"orientation"`
	Icons           []manifestIcon `json:"icons"`
	Scope           string         `json:"scope"`
	ID              string         `json:"id"`
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

func Manifest() web.HandlerFunc {
	return func(c *web.Context) error {
		tenant := c.Tenant()
		if tenant == nil {
			return c.NotFound()
		}

		baseURL := c.BaseURL()
		faviconBase := "/static/favicon"
		if tenant.LogoBlobKey != "" {
			faviconBase = "/static/favicon/" + tenant.LogoBlobKey
		}

		manifest := webAppManifest{
			Name:            tenant.Name,
			ShortName:       truncateName(tenant.Name, 12),
			StartURL:        "/?utm_source=pwa",
			Display:         "standalone",
			BackgroundColor: "#ffffff",
			ThemeColor:      "#1a202c",
			Orientation:     "any",
			Scope:           "/",
			ID:              baseURL,
			Icons: []manifestIcon{
				{
					Src:   faviconBase + "?size=192",
					Sizes: "192x192",
					Type:  "image/png",
				},
				{
					Src:     faviconBase + "?size=192",
					Sizes:   "192x192",
					Type:    "image/png",
					Purpose: "maskable",
				},
				{
					Src:   faviconBase + "?size=512",
					Sizes: "512x512",
					Type:  "image/png",
				},
			},
		}

		c.Response.Header().Set("Content-Type", "application/manifest+json")
		c.Response.Header().Set("Cache-Control", "public, max-age=86400")
		return c.JSON(http.StatusOK, manifest)
	}
}

func truncateName(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

func ServiceWorker() web.HandlerFunc {
	return func(c *web.Context) error {
		serviceWorkerOnce.Do(func() {
			content, err := os.ReadFile(env.Path("/views/sw.js"))
			if err != nil {
				serviceWorkerContent = []byte("'use strict';")
			} else {
				serviceWorkerContent = content
			}
		})

		c.Response.Header().Set("Content-Type", "application/javascript")
		c.Response.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		c.Response.Header().Set("Service-Worker-Allowed", "/")
		return c.Blob(http.StatusOK, "application/javascript", serviceWorkerContent)
	}
}

var allowedPushServiceHosts = []string{
	"fcm.googleapis.com",
	"google.com",
	"updates.push.services.mozilla.com",
	"updates-autopush.stage.mozaws.net",
	"updates-autopush.dev.mozaws.net",
	"push.apple.com",
	"web.push.apple.com",
	"windows.com",
	"notify.windows.com",
	"push.services.mozilla.com",
}

func isValidPushEndpoint(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return false
	}

	if parsed.Scheme != "https" {
		return false
	}

	host := strings.ToLower(parsed.Host)
	for _, allowed := range allowedPushServiceHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}

type pushSubscribeInput struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func SavePushSubscription() web.HandlerFunc {
	return func(c *web.Context) error {
		if !env.IsWebPushEnabled() {
			return c.BadRequest(web.Map{"error": "Push notifications are not enabled"})
		}

		input := new(pushSubscribeInput)
		if err := c.Bind(input); err != nil {
			return c.BadRequest(web.Map{"error": "Invalid request body"})
		}

		if input.Endpoint == "" || input.Keys.P256dh == "" || input.Keys.Auth == "" {
			return c.BadRequest(web.Map{"error": "Missing required fields"})
		}

		if !isValidPushEndpoint(input.Endpoint) {
			return c.BadRequest(web.Map{"error": "Invalid push service endpoint"})
		}

		saveCmd := &cmd.SavePushSubscription{
			Endpoint:  input.Endpoint,
			KeyP256dh: input.Keys.P256dh,
			KeyAuth:   input.Keys.Auth,
		}

		if err := bus.Dispatch(c, saveCmd); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{"status": "subscribed"})
	}
}

type pushUnsubscribeInput struct {
	Endpoint string `json:"endpoint"`
}

func DeletePushSubscription() web.HandlerFunc {
	return func(c *web.Context) error {
		input := new(pushUnsubscribeInput)
		if err := c.Bind(input); err != nil {
			return c.BadRequest(web.Map{"error": "Invalid request body"})
		}

		if input.Endpoint == "" {
			return c.BadRequest(web.Map{"error": "Missing endpoint"})
		}

		deleteCmd := &cmd.DeletePushSubscription{
			Endpoint: input.Endpoint,
		}

		if err := bus.Dispatch(c, deleteCmd); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{"status": "unsubscribed"})
	}
}

func GetVAPIDPublicKey() web.HandlerFunc {
	return func(c *web.Context) error {
		if !env.IsWebPushEnabled() {
			return c.Ok(web.Map{"enabled": false})
		}

		return c.Ok(web.Map{
			"enabled":   true,
			"publicKey": webpush.GetPublicKeyBase64(),
		})
	}
}

func HasPushSubscription() web.HandlerFunc {
	return func(c *web.Context) error {
		if !env.IsWebPushEnabled() {
			return c.Ok(web.Map{"enabled": false, "subscribed": false})
		}

		user := c.User()
		if user == nil {
			return c.Ok(web.Map{"enabled": true, "subscribed": false})
		}

		q := &query.HasPushSubscription{UserID: user.ID}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"enabled":    true,
			"subscribed": q.Result,
		})
	}
}
