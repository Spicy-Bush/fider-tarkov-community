package middlewares

import (
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// Secure middleware is responsible for
// 1. Setting the HTTP Security Headers
// 2. Protecting from Host attacks
func Secure() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			cdnHost := env.Config.CDN.Host
			if cdnHost != "" && !env.IsSingleHostMode() {
				cdnHost = "*." + cdnHost
			}
			csp := fmt.Sprintf(web.CspPolicyTemplate, c.ContextID(), cdnHost)
			if env.IsDevelopment() && env.Config.DevUI {
				// Vite uses a worker to reconnect after a server restart.
				csp += "; worker-src 'self' blob:"
			}

			c.Response.Header().Set("Content-Security-Policy", strings.TrimSpace(csp))
			c.Response.Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response.Header().Set("X-Content-Type-Options", "nosniff")
			c.Response.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
			c.Response.Header().Set("X-Frame-Options", "DENY")
			return next(c)
		}
	}
}

func CSRF() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}

			// Cross-origin browsers must preflight this content type, including empty writes.
			contentType, _, err := mime.ParseMediaType(c.Request.GetHeader("Content-Type"))
			if err != nil || contentType != web.JSONContentType {
				return c.Forbidden()
			}

			return next(c)
		}
	}
}
