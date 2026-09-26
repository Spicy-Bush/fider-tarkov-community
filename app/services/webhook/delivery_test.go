package webhook_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/webhook"
)

func TestDeliveryFailureAndRecovery(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusGone} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			bus.Init(webhook.Service{})
			hook := &entity.Webhook{ID: 1, Status: enum.WebhookEnabled, Url: "https://example.com/events", HttpMethod: "POST", Content: "{}"}
			bus.AddHandler(func(ctx context.Context, q *query.GetWebhook) error {
				copy := *hook
				q.Result = &copy
				return nil
			})
			bus.AddHandler(func(ctx context.Context, q *query.MarkWebhookAsFailed) error {
				hook.Status = enum.WebhookFailed
				return nil
			})
			calls, responseStatus := 0, status
			bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
				calls++
				c.ResponseStatusCode = responseStatus
				return nil
			})
			ctx := context.Background()
			err := bus.Dispatch(ctx, &cmd.DeliverWebhook{ID: hook.ID})
			if status == http.StatusGone {
				if err != nil || hook.Status != enum.WebhookFailed {
					t.Fatalf("permanent failure: status=%v error=%v", hook.Status, err)
				}
			} else if err == nil || hook.Status != enum.WebhookEnabled {
				t.Fatalf("temporary failure: status=%v error=%v", hook.Status, err)
			}
			responseStatus = http.StatusOK
			if err := bus.Dispatch(ctx, &cmd.DeliverWebhook{ID: hook.ID}); err != nil {
				t.Fatalf("provider recovered: %v", err)
			}
			wantCalls := 2
			if status == http.StatusGone {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("provider calls=%d, want %d", calls, wantCalls)
			}
		})
	}
}
