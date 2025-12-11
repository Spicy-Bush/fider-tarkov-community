package middlewares

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

// cachedTenant holds the cached tenant for single-tenant mode
var (
	cachedTenant atomic.Pointer[entity.Tenant]
	tenantOnce   sync.Once
)

// InvalidateTenantCache clears the cached tenant (call after tenant updates)
func InvalidateTenantCache() {
	cachedTenant.Store(nil)
	tenantOnce = sync.Once{}
}

// Tenant adds either SingleTenant or MultiTenant to the pipeline
func Tenant() web.MiddlewareFunc {
	if env.IsSingleHostMode() {
		return SingleTenant()
	}
	return MultiTenant()
}

// SingleTenant inject default tenant into current context
// Caches the tenant after first load to avoid DB queries on every request
func SingleTenant() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			// Try to get cached tenant first
			tenant := cachedTenant.Load()
			if tenant != nil && !tenant.IsDisabled() {
				c.SetTenant(tenant)
				return next(c)
			}

			// Load tenant from DB (only happens once due to sync.Once)
			var loadErr error
			tenantOnce.Do(func() {
				firstTenant := &query.GetFirstTenant{}
				err := bus.Dispatch(c, firstTenant)
				if err != nil && errors.Cause(err) != app.ErrNotFound {
					loadErr = err
					return
				}
				if firstTenant.Result != nil {
					cachedTenant.Store(firstTenant.Result)
				}
			})

			if loadErr != nil {
				return c.Failure(loadErr)
			}

			tenant = cachedTenant.Load()
			if tenant != nil && !tenant.IsDisabled() {
				c.SetTenant(tenant)
			}

			return next(c)
		}
	}
}

// MultiTenant extract tenant information from hostname and inject it into current context
func MultiTenant() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			hostname := c.Request.URL.Hostname()

			byDomain := &query.GetTenantByDomain{Domain: hostname}
			err := bus.Dispatch(c, byDomain)
			if err != nil && errors.Cause(err) != app.ErrNotFound {
				return c.Failure(err)
			}

			if byDomain.Result != nil && !byDomain.Result.IsDisabled() {
				c.SetTenant(byDomain.Result)

				if byDomain.Result.CNAME != "" && !c.IsAjax() {
					baseURL := web.TenantBaseURL(c, byDomain.Result)
					if baseURL != c.BaseURL() {
						link := baseURL + c.Request.URL.RequestURI()
						c.SetCanonicalURL(link)
					}
				}
			}

			return next(c)
		}
	}
}

// RequireTenant returns 404 if tenant is not available
func RequireTenant() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			tenant := c.Tenant()
			if tenant == nil {
				if env.IsSingleHostMode() {
					return c.Redirect("/signup")
				}
				return c.NotFound()
			}

			return next(c)
		}
	}
}

// BlockPendingTenants blocks requests for pending tenants
func BlockPendingTenants() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			if c.Tenant().Status == enum.TenantPending {
				return c.Page(http.StatusOK, web.Props{
					Page:        "SignUp/PendingActivation.page",
					Title:       "Pending Activation",
					Description: "We sent you a confirmation email with a link to activate your site. Please check your inbox to activate it.",
				})
			}
			return next(c)
		}
	}
}

// CheckTenantPrivacy blocks requests of unauthenticated users for private tenants
func CheckTenantPrivacy() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			if c.Tenant().IsPrivate && !c.IsAuthenticated() {
				return c.Redirect("/signin")
			}
			return next(c)
		}
	}
}

// BlockLockedTenants blocks requests on locked tenants as they are in read-only mode
func BlockLockedTenants() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			if c.Tenant().Status == enum.TenantLocked {

				// Only API operations are blocked, so it's ok to always return a JSON
				return c.JSON(http.StatusPaymentRequired, web.Map{})
			}
			return next(c)
		}
	}
}
