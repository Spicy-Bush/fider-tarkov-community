package middlewares_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestSecureWithoutCDN(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	server.Use(middlewares.Secure())

	var ctxID string
	status, response := server.Execute(func(c *web.Context) error {
		ctxID = c.ContextID()
		return c.NoContent(http.StatusOK)
	})

	expectedPolicy := expectedCSP(ctxID, "")

	Expect(status).Equals(http.StatusOK)
	Expect(response.Header().Get("Content-Security-Policy")).Equals(expectedPolicy)
	Expect(response.Header().Get("X-XSS-Protection")).Equals("1; mode=block")
	Expect(response.Header().Get("X-Content-Type-Options")).Equals("nosniff")
	Expect(response.Header().Get("Referrer-Policy")).Equals("no-referrer-when-downgrade")
}

func TestSecureWithCDN(t *testing.T) {
	RegisterT(t)

	env.Config.CDN.Host = "test.fider.io"

	server := mock.NewServer()
	server.Use(middlewares.Secure())

	var ctxID string
	status, response := server.Execute(func(c *web.Context) error {
		ctxID = c.ContextID()
		return c.NoContent(http.StatusOK)
	})

	expectedPolicy := expectedCSP(ctxID, "*.test.fider.io")

	Expect(status).Equals(http.StatusOK)
	Expect(response.Header().Get("Content-Security-Policy")).Equals(expectedPolicy)
	Expect(response.Header().Get("X-XSS-Protection")).Equals("1; mode=block")
	Expect(response.Header().Get("X-Content-Type-Options")).Equals("nosniff")
	Expect(response.Header().Get("Referrer-Policy")).Equals("no-referrer-when-downgrade")
}

func TestSecureWithCDN_SingleHost(t *testing.T) {
	RegisterT(t)

	env.Config.CDN.Host = "test.fider.io"

	server := mock.NewSingleTenantServer()
	server.Use(middlewares.Secure())

	var ctxID string
	status, response := server.WithURL("http://test.fider.io").Execute(func(c *web.Context) error {
		ctxID = c.ContextID()
		return c.NoContent(http.StatusOK)
	})

	expectedPolicy := expectedCSP(ctxID, "test.fider.io")

	Expect(status).Equals(http.StatusOK)
	Expect(response.Header().Get("Content-Security-Policy")).Equals(expectedPolicy)
	Expect(response.Header().Get("X-XSS-Protection")).Equals("1; mode=block")
	Expect(response.Header().Get("X-Content-Type-Options")).Equals("nosniff")
	Expect(response.Header().Get("Referrer-Policy")).Equals("no-referrer-when-downgrade")
}

func TestSecureAllowsViteReconnectOnlyInDevelopment(t *testing.T) {
	previous := env.Config
	t.Cleanup(func() { env.Config = previous })

	for _, environment := range []string{"development", "production"} {
		for _, devUI := range []bool{false, true} {
			server := mock.NewServer()
			server.Use(middlewares.Secure())
			env.Config.Environment = environment
			env.Config.DevUI = devUI

			_, response := server.Execute(func(c *web.Context) error {
				return c.NoContent(http.StatusOK)
			})

			policy := response.Header().Get("Content-Security-Policy")
			allowsWorker := strings.Contains(policy, "worker-src 'self' blob:")
			if allowsWorker != (environment == "development" && devUI) {
				t.Fatalf("environment %s, dev UI %t: unexpected worker policy %q", environment, devUI, policy)
			}
		}
	}
}

func expectedCSP(nonce, cdnHost string) string {
	return strings.Join([]string{
		"base-uri 'self'",
		"default-src 'self'",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://*.paddle.com " + cdnHost,
		"script-src 'self' 'unsafe-inline' 'nonce-" + nonce + "' 'strict-dynamic' " +
			"https://ep1.adtrafficquality.google https://www.google-analytics.com https://*.paddle.com " +
			"https://*.googletagmanager.com https://pagead2.googlesyndication.com " +
			"https://static.cloudflareinsights.com https://*.cloudflare.com " + cdnHost,
		"img-src 'self' https: data: blob: https://*.google-analytics.com https://*.analytics.google.com " +
			"https://*.googletagmanager.com https://*.g.doubleclick.net " + cdnHost,
		"font-src 'self' https://fonts.gstatic.com data: " + cdnHost,
		"object-src 'none'",
		"media-src 'none'",
		"connect-src 'self' https://*.google-analytics.com https://*.analytics.google.com " +
			"https://*.googletagmanager.com https://*.g.doubleclick.net " +
			"https://*.googlesyndication.com https://ep1.adtrafficquality.google https://ep2.adtrafficquality.google https://www.google.com " +
			"https://cloudflareinsights.com https://*.cloudflare.com " + cdnHost,
		"frame-src 'self' https://*.paddle.com https://*.doubleclick.net https://*.googlesyndication.com https://www.google.com https://www.googletagmanager.com " +
			"https://www.youtube.com/ https://vk.com/ https://vkvideo.ru/",
		"frame-ancestors 'none'",
	}, "; ")
}
