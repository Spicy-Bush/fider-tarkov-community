package postgres_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webpush"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestPostNotificationPushRecovery(t *testing.T) {
	previous := env.Config.WebPush
	publicKey, privateKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	env.Config.WebPush.VAPIDPublicKey = publicKey
	env.Config.WebPush.VAPIDPrivateKey = privateKey
	t.Cleanup(func() {
		env.Config.WebPush = previous
	})

	scenarios := []struct {
		name          string
		status        int32
		change        string
		requests      int32
		subscriptions int
	}{
		{
			name:          "retry",
			status:        http.StatusServiceUnavailable,
			requests:      2,
			subscriptions: 1,
		},
		{
			name:          "expired",
			status:        http.StatusGone,
			requests:      1,
			subscriptions: 0,
		},
		{
			name:          "unsubscribed",
			status:        http.StatusCreated,
			change:        "unsubscribe",
			requests:      0,
			subscriptions: 1,
		},
		{
			name:          "deleted",
			status:        http.StatusCreated,
			change:        "delete",
			requests:      0,
			subscriptions: 0,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			f := newPostWorkflow(t)

			var status, requests atomic.Int32
			status.Store(scenario.status)

			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)

				if r.Method != http.MethodPost {
					t.Errorf("push method: got %s, want POST", r.Method)
				}

				if r.Header.Get("Content-Encoding") != "aes128gcm" {
					t.Error("push payload was not encrypted with aes128gcm")
				}

				if r.ContentLength == 0 {
					t.Error("push payload was empty")
				}

				w.WriteHeader(int(status.Load()))
			}))
			t.Cleanup(provider.Close)

			recipient := &entity.User{ID: 2}
			recipientCtx := context.WithValue(f.ctx, app.UserCtxKey, recipient)
			subscription := &cmd.SavePushSubscription{
				Endpoint:  provider.URL,
				KeyP256dh: publicKey,
				KeyAuth:   base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
			}

			if err := bus.Dispatch(recipientCtx, subscription); err != nil {
				t.Fatal(err)
			}

			subscriptions := &query.GetPushSubscriptionsByUser{UserID: recipient.ID}
			if err := bus.Dispatch(f.ctx, subscriptions); err != nil {
				t.Fatalf("subscription setup: %v", err)
			}

			if len(subscriptions.Result) != 1 {
				t.Fatalf("subscription setup: got %d subscriptions, want 1", len(subscriptions.Result))
			}

			otherTenant := &entity.Tenant{ID: f.tenant.ID + 1}
			otherTenantCtx := context.WithValue(f.ctx, app.TenantCtxKey, otherTenant)
			lookup := &query.GetPushSubscription{ID: subscriptions.Result[0].ID}

			err := bus.Dispatch(otherTenantCtx, lookup)
			if errors.Cause(err) != app.ErrNotFound {
				t.Fatalf("subscription crossed tenant boundary: %v", err)
			}

			subscribed := true
			bus.AddHandler(func(_ context.Context, q *query.GetActiveSubscribers) error {
				if q.Channel == enum.NotificationChannelPush && subscribed {
					q.Result = []*entity.User{recipient}
				}

				return nil
			})

			bus.AddHandler(func(context.Context, *query.ListActiveWebhooksByType) error {
				return nil
			})

			f.queuePostNotification(t)

			found, err := tasks.DeliverPendingNotification(f.ctx)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}

			if !found {
				t.Fatal("preparation did not find the queued post")
			}

			switch scenario.change {
			case "unsubscribe":
				subscribed = false

			case "delete":
				deletion := &cmd.DeletePushSubscription{Endpoint: provider.URL}
				if err := bus.Dispatch(recipientCtx, deletion); err != nil {
					t.Fatal(err)
				}
			}

			found, err = tasks.DeliverPendingNotification(f.ctx)
			if !found {
				t.Fatal("delivery did not find the queued recipient")
			}

			if scenario.status == http.StatusServiceUnavailable {
				if err == nil {
					t.Fatal("temporary provider failure was not returned")
				}

				pending := workflowCount(t, `SELECT COUNT(*) FROM notification_recipients
					WHERE attempts = 1 AND last_error IS NOT NULL`)
				if pending != 1 {
					t.Fatal("temporary failure did not retain the recipient and cause")
				}

				status.Store(http.StatusCreated)
				if _, err := dbx.Connection().Exec("UPDATE notification_recipients SET available_at = NOW()"); err != nil {
					t.Fatal(err)
				}

				found, err = tasks.DeliverPendingNotification(f.ctx)
				if err != nil {
					t.Fatalf("recover: %v", err)
				}

				if !found {
					t.Fatal("recovery did not find the failed recipient")
				}
			} else if err != nil {
				t.Fatalf("deliver: %v", err)
			}

			if requests.Load() != scenario.requests {
				t.Fatalf("provider requests: got %d, want %d", requests.Load(), scenario.requests)
			}

			remainingSubscriptions := workflowCount(t, "SELECT COUNT(*) FROM push_subscriptions")
			if remainingSubscriptions != scenario.subscriptions {
				t.Fatalf("remaining subscriptions: got %d, want %d", remainingSubscriptions, scenario.subscriptions)
			}

			pendingDeliveries := workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries")
			pendingRecipients := workflowCount(t, "SELECT COUNT(*) FROM notification_recipients")
			if pendingDeliveries != 0 || pendingRecipients != 0 {
				t.Fatal("completed push retained queue data")
			}
		})
	}
}
