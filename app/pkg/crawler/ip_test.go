package crawler_test

import (
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crawler"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func TestClientIPRequiresTrustedPeer(t *testing.T) {
	previous := env.Config
	env.Config.Environment = "development"
	env.Config.TrustedProxies = []string{"127.0.0.1/32", "::1/128"}
	t.Cleanup(func() { env.Config = previous })

	for _, test := range []struct {
		peer   string
		header []string
		want   string
	}{
		{"192.0.2.4:1234", []string{"66.249.66.1"}, "192.0.2.4"},
		{"127.0.0.1:1234", []string{"66.249.66.1"}, "66.249.66.1"},
		{"[::1]:1234", []string{"2001:db8::1"}, "2001:db8::1"},
		{"[::ffff:127.0.0.1]:1234", []string{"192.0.2.4"}, "192.0.2.4"},
		{"127.0.0.1:1234", []string{"bad"}, "127.0.0.1"},
		{"127.0.0.1:1234", []string{"192.0.2.4", "192.0.2.5"}, "127.0.0.1"},
		{"[2001:db8::2]:1234", nil, "2001:db8::2"},
	} {
		request := &http.Request{RemoteAddr: test.peer, Header: make(http.Header)}
		request.Header["Cf-Connecting-Ip"] = test.header
		request.Header.Set("X-Real-IP", "66.249.66.1")
		if got := crawler.GetRealIP(request).String(); got != test.want {
			t.Errorf("peer %s: got %s, wanted %s", test.peer, got, test.want)
		}
	}
}
