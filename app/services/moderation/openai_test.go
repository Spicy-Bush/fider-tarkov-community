package moderation

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func TestProviderDiagnostics(t *testing.T) {
	bus.Reset()
	old := env.Config.OpenAI.APIKey
	env.Config.OpenAI.APIKey = "test-secret"
	t.Cleanup(func() { env.Config.OpenAI.APIKey = old })
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		c.ResponseStatusCode = 429
		c.ResponseHeader = http.Header{
			"Date":                           {"Fri, 25 Sep 2026 14:20:51 GMT"},
			"X-Request-Id":                   {"req_fixture"},
			"Retry-After":                    {"120"},
			"X-Ratelimit-Remaining-Requests": {"0"},
			"Set-Cookie":                     {"private-cookie"},
		}

		c.ResponseBody = []byte(`{"error":{"type":"invalid_request_error","code":null,"message":"echoed private submission"}}`)
		return nil
	})

	_, err := CallOpenAIModeration(context.Background(), "private submission", nil)
	var failure *ProviderError

	if !errors.As(err, &failure) {
		t.Fatalf("error=%v", err)
	}

	if failure.Status != 429 || failure.Type != "invalid_request_error" || failure.RequestID != "req_fixture" || failure.RetryAfter != 120*time.Second {
		t.Fatalf("diagnostics: %+v", failure)
	}

	if failure.Headers["x-ratelimit-remaining-requests"] != "0" || failure.Headers["date"] != "Fri, 25 Sep 2026 14:20:51 GMT" {
		t.Fatalf("headers=%v", failure.Headers)
	}

	if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "test-secret") {
		t.Fatal("diagnostic leaked credentials or content")
	}
}

func TestProviderNullScoresRemainUnreviewed(t *testing.T) {
	bus.Reset()
	old := env.Config.OpenAI.APIKey
	env.Config.OpenAI.APIKey = "test-key"
	t.Cleanup(func() { env.Config.OpenAI.APIKey = old })
	body := `{"results":[{"category_scores":{"sexual":null,"sexual/minors":0,"self-harm":0,"self-harm/intent":0,"self-harm/instructions":0}}]}`
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		c.ResponseStatusCode = 200
		c.ResponseBody = []byte(body)
		return nil
	})

	_, err := CallOpenAIModeration(context.Background(), "name", nil)
	var failure *ProviderError

	if !errors.As(err, &failure) || failure.Code != "incomplete_response" {
		t.Fatalf("null score: %v", err)
	}

	body = strings.Replace(body, "null", "0", 1)
	response, err := CallOpenAIModeration(context.Background(), "name", nil)

	if err != nil || len(CheckThresholds(response)) != 0 {
		t.Fatalf("valid zero scores rejected: %v", err)
	}
}
