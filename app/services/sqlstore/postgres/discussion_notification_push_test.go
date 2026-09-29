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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webpush"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestDiscussionNotificationPushRecovery(t *testing.T) {
	previousPush := env.Config.WebPush
	previousEmail := env.Config.Email
	publicKey, privateKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	env.Config.WebPush.VAPIDPublicKey = publicKey
	env.Config.WebPush.VAPIDPrivateKey = privateKey
	env.Config.Email.DisableEmailNotifications = true
	t.Cleanup(func() {
		env.Config.WebPush = previousPush
		env.Config.Email = previousEmail
	})

	for _, hideBeforeRetry := range []bool{false, true} {
		name := "recover"
		if hideBeforeRetry {
			name = "hidden before retry"
		}

		t.Run(name, func(t *testing.T) {
			f := newPostWorkflow(t)
			var requests, status atomic.Int32
			status.Store(http.StatusServiceUnavailable)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Content-Encoding") != "aes128gcm" || r.ContentLength == 0 {
					t.Error("push provider did not receive encrypted content")
				}

				w.WriteHeader(int(status.Load()))
			}))
			t.Cleanup(provider.Close)

			recipient := &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive}
			recipientContext := context.WithValue(f.ctx, app.UserCtxKey, recipient)
			if err := bus.Dispatch(recipientContext,
				&cmd.SavePushSubscription{
					Endpoint:  provider.URL,
					KeyP256dh: publicKey,
					KeyAuth:   base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
				},
				&cmd.UpdateCurrentUserSettings{Settings: map[string]string{"event_notification_mention": "4"}},
			); err != nil {
				t.Fatal(err)
			}

			bus.AddHandler(func(context.Context, *query.ListActiveWebhooksByType) error { return nil })
			post := &cmd.AddNewPost{Title: "Push delivery", Description: "Private content must be rechecked before retry"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}

			comment := &cmd.CreateComment{
				PostNumber:   post.Result.Number,
				Content:      `A reply for @{"id":2,"name":"Arya"}`,
				BaseURL:      "http://localhost:3000",
				SubmissionID: "reply-push",
			}
			if err := bus.Dispatch(f.ctx, comment); err != nil {
				t.Fatal(err)
			}

			if found, err := tasks.DeliverPendingNotification(f.ctx); !found || err != nil {
				t.Fatalf("prepare: found=%v err=%v", found, err)
			}

			if found, err := tasks.DeliverPendingNotification(f.ctx); !found || err == nil {
				t.Fatalf("provider failure: found=%v err=%v", found, err)
			}

			if hideBeforeRetry {
				if err := bus.Dispatch(f.ctx, &cmd.SetModerationPending{
					ContentType: "comment",
					ContentID:   comment.Result.ID,
					Pending:     true,
				}); err != nil {
					t.Fatal(err)
				}
			}

			status.Store(http.StatusCreated)
			if _, err := dbx.Connection().Exec("UPDATE notification_recipients SET available_at=now()"); err != nil {
				t.Fatal(err)
			}

			if found, err := tasks.DeliverPendingNotification(f.ctx); !found || err != nil {
				t.Fatalf("retry: found=%v err=%v", found, err)
			}

			if hideBeforeRetry {
				if requests.Load() != 1 || workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries") != 1 {
					t.Fatalf("hidden content was sent or discarded: requests=%d", requests.Load())
				}

				if err := bus.Dispatch(f.ctx, &cmd.SetModerationPending{
					ContentType: "comment",
					ContentID:   comment.Result.ID,
					Pending:     false,
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := dbx.Connection().Exec("UPDATE notification_deliveries SET available_at = NOW()"); err != nil {
					t.Fatal(err)
				}
				if found, err := tasks.DeliverPendingNotification(f.ctx); !found || err != nil {
					t.Fatalf("approved retry: found=%v err=%v", found, err)
				}
			}
			if requests.Load() != 2 || workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries") != 0 {
				t.Fatalf("retry did not deliver approved content: requests=%d", requests.Load())
			}
		})
	}
}
