package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestCSRFRequiresNonSimpleContentType(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data", "text/plain; application/json", "application/json", "application/json; charset=utf-8"} {
			t.Run(method+"/"+contentType, func(t *testing.T) {
				request := httptest.NewRequest(method, "http://localhost/api/action", nil)
				request.Header.Set("Accept", "application/json")
				request.Header.Set("Content-Type", contentType)
				response := httptest.NewRecorder()
				ctx, err := web.NewContext(web.New(), request, response, nil)
				if err != nil {
					t.Fatal(err)
				}

				called := false
				err = middlewares.CSRF()(func(c *web.Context) error {
					called = true
					return c.NoContent(http.StatusNoContent)
				})(ctx)
				if err != nil {
					t.Fatal(err)
				}

				allowed := contentType == "application/json" || contentType == "application/json; charset=utf-8"
				want := http.StatusForbidden
				if allowed {
					want = http.StatusNoContent
				}
				if called != allowed || response.Code != want {
					t.Fatalf("handler called=%v, HTTP %d; want called=%v, HTTP %d", called, response.Code, allowed, want)
				}
			})
		}
	}
}
