package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
)

func TestRequestsCannotReachPrivateServices(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("private service data"))
	}))
	defer server.Close()

	request := &cmd.HTTPRequest{Method: http.MethodGet, URL: server.URL}
	if err := requestHandler(context.Background(), request); err == nil {
		t.Fatal("a tenant-configured URL reached a private service")
	}
	if requests.Load() != 0 || len(request.ResponseBody) != 0 {
		t.Fatal("the private service was contacted or its data was returned")
	}
}

func TestResponseLimitAndRecovery(t *testing.T) {
	var responseBytes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", maxResponseBytes+1)), responseBytes.Load())
	}))
	defer server.Close()

	previous := client
	client = server.Client()
	t.Cleanup(func() { client = previous })

	for _, size := range []int64{maxResponseBytes, maxResponseBytes + 1, 5} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			responseBytes.Store(size)
			request := &cmd.HTTPRequest{Method: http.MethodGet, URL: server.URL}
			err := requestHandler(context.Background(), request)
			if size > maxResponseBytes {
				if err == nil || len(request.ResponseBody) != 0 || request.ResponseStatusCode != 0 {
					t.Fatalf("oversized response returned data: error=%v, bytes=%d, HTTP %d", err, len(request.ResponseBody), request.ResponseStatusCode)
				}
				return
			}

			if err != nil || int64(len(request.ResponseBody)) != size || request.ResponseStatusCode != http.StatusOK {
				t.Fatalf("valid response failed: error=%v, bytes=%d, HTTP %d", err, len(request.ResponseBody), request.ResponseStatusCode)
			}
		})
	}
}
