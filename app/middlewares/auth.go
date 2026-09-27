package middlewares

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// IsAuthenticated blocks non-authenticated requests
func IsAuthenticated() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			if !c.IsAuthenticated() {
				return c.Unauthorized()
			}
			return next(c)
		}
	}
}

func RequirePermission(permission entity.Permission) web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			if !c.IsAuthenticated() {
				return c.Unauthorized()
			}
			if !entity.Can(c.User(), c.Tenant(), permission) {
				return c.Forbidden()
			}
			return next(c)
		}
	}
}
