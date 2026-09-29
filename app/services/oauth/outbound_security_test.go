package oauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/oauth"
)

func TestCustomOAuthTokenExchangeRejectsPrivateDestination(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer"}`))
	}))
	defer server.Close()

	bus.Init(oauth.Service{})
	bus.AddHandler(func(ctx context.Context, q *query.GetCustomOAuthConfigByProvider) error {
		q.Result = &entity.OAuthConfig{
			Provider:     q.Provider,
			Status:       enum.OAuthConfigEnabled,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			TokenURL:     server.URL + "/token",
			ProfileURL:   server.URL + "/profile",
		}
		return nil
	})

	ctx := newGetContext("http://login.test.fider.io:3000")
	profile := &query.GetOAuthRawProfile{Provider: "_private", Code: "test-code"}
	err := bus.Dispatch(ctx, profile)
	if err == nil || !strings.Contains(err.Error(), "public network destination") {
		t.Fatalf("token exchange did not reject its private destination: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("private token endpoint received %d requests", requests.Load())
	}
	if profile.Result != "" {
		t.Fatalf("rejected exchange returned a profile: %q", profile.Result)
	}
}
