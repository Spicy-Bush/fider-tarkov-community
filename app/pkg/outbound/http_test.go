package outbound

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPrivateDestinationsAreRejectedBeforeConnecting(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	client := NewClient()
	defer client.CloseIdleConnections()
	for _, target := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1)} {
		response, err := client.Get(target)
		if err == nil {
			response.Body.Close()
			t.Errorf("private URL was accepted: %s", target)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("private service received a request")
	}
}

func TestOutboundAddressPolicy(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1", "10.0.0.1", "172.16.1.1", "192.168.1.1", "169.254.169.254",
		"100.100.100.200", "0.1.2.3", "192.0.0.8", "198.18.0.1", "224.0.0.1", "240.0.0.1",
		"::", "::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "fc00::1", "fe80::1", "ff02::1",
		"64:ff9b::7f00:1", "2002:7f00:1::", "2001:db8::1", "2001::1",
	} {
		if err := publicConnection(context.Background(), "tcp", net.JoinHostPort(address, "443"), nil); err == nil {
			t.Errorf("nonpublic address accepted: %s", address)
		}
	}

	for _, address := range []string{"8.8.8.8", "1.1.1.1", "::ffff:8.8.8.8", "2001:4860:4860::8888", "2606:4700:4700::1111"} {
		if err := publicConnection(context.Background(), "tcp", net.JoinHostPort(address, "443"), nil); err != nil {
			t.Errorf("public address rejected: %s: %v", address, err)
		}
	}
}
