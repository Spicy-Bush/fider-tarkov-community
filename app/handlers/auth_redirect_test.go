package handlers_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestAuthenticationRedirectsStayOnSite(t *testing.T) {
	for _, route := range []struct {
		path    string
		handler web.HandlerFunc
	}{
		{path: "/signout", handler: handlers.SignOut()},
		{path: "/oauth/example/token", handler: handlers.OAuthToken()},
	} {
		for _, target := range []string{"https://outside.invalid/", "//outside.invalid/", `/\outside.invalid/`, `\outside.invalid`, "/%5coutside.invalid/", "javascript:alert(1)"} {
			t.Run(route.path+"/"+target, func(t *testing.T) {
				server := mock.NewServer().WithURL("http://demo.test.fider.io" + route.path + "?redirect=" + url.QueryEscape(target))
				status, response := server.Execute(route.handler)
				location := response.Header().Get("Location")
				if status != http.StatusTemporaryRedirect || location != "/" {
					t.Fatalf("unsafe redirect %q returned HTTP %d, Location %q", target, status, location)
				}
			})
		}

		for _, target := range []string{"/posts/1?sort=latest#comment-2", "/", "/pages/news"} {
			server := mock.NewServer().WithURL("http://demo.test.fider.io" + route.path + "?redirect=" + url.QueryEscape(target))
			status, response := server.Execute(route.handler)
			if status != http.StatusTemporaryRedirect || response.Header().Get("Location") != target {
				t.Fatalf("safe redirect %q was changed: HTTP %d, Location %q", target, status, response.Header().Get("Location"))
			}
		}
	}
}
