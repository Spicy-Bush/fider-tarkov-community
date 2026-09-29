package s3

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
)

func TestS3ReadLimitWithUnknownLengthAndRecovery(t *testing.T) {
	var large atomic.Bool
	large.Store(true)
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		if large.Load() {
			_, _ = io.WriteString(w, strings.Repeat("x", 4096))
		} else {
			_, _ = io.WriteString(w, "small")
		}
	})
	request := &query.GetBlobByKey{Key: "image", MaxBytes: 64}
	if err := backend.Get(ctx, request); !errors.Is(err, readlimit.ErrTooLarge) || request.Result != nil {
		t.Fatalf("unknown-length body bypassed limit: err=%v", err)
	}

	large.Store(false)
	if err := backend.Get(ctx, request); err != nil || string(request.Result.Content) != "small" {
		t.Fatalf("later healthy object failed: err=%v", err)
	}
}

type blobResponseTransport struct{}

func (blobResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Length": []string{"1"}, "Content-Type": []string{"image/png"}},
		ContentLength: 1,
		Body:          io.NopCloser(strings.NewReader(strings.Repeat("x", 4096))),
		Request:       request,
	}, nil
}

func TestS3ReadLimitRejectsFalseContentLength(t *testing.T) {
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("test response transport was not installed")
	})
	DefaultClient.Config.HTTPClient = &http.Client{Transport: blobResponseTransport{}}
	request := &query.GetBlobByKey{Key: "image", MaxBytes: 64}
	if err := backend.Get(ctx, request); !errors.Is(err, readlimit.ErrTooLarge) || request.Result != nil {
		t.Fatalf("false declared size bypassed limit: err=%v", err)
	}
}
