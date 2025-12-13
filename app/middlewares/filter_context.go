package middlewares

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func FilterContext() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			tenant := c.Tenant()
			if tenant == nil {
				return next(c)
			}

			// TODO: move this to a route instead of a middleware
			// Will also need to migrate to a new table in the database instead of using the tenant

			user := c.User()
			if user == nil || (!user.IsAdministrator() && !user.IsCollaborator()) {
				tenantCopy := *tenant
				tenantCopy.ProfanityWords = ""
				c.Set(app.TenantCtxKey, &tenantCopy)
			}

			return next(c)
		}
	}
}
