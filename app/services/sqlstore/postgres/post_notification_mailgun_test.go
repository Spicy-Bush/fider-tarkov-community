package postgres_test

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email/mailgun"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestPostNotificationMailgunRecovery(t *testing.T) {
	scenarios := []struct {
		name      string
		status    int
		body      string
		isolate   bool
		transport bool
	}{
		{
			name:   "service unavailable",
			status: 503,
			body:   `{"message":"Service unavailable"}`,
		},
		{
			name:   "rate limited",
			status: 429,
			body:   `{"message":"Too many requests"}`,
		},
		{
			name:      "transport interrupted",
			transport: true,
		},
		{
			name:   "generic bad request",
			status: 400,
			body:   `{"message":"Invalid request"}`,
		},
		{
			name:   "invalid sender",
			status: 400,
			body:   `{"message":"'from' parameter is not a valid address"}`,
		},
		{
			name:    "invalid recipient",
			status:  400,
			body:    `{"message":"'to' parameter is not a valid address. please check documentation"}`,
			isolate: true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			f := newPostWorkflow(t)

			previousType := env.Config.Email.Type
			previousDisabled := env.Config.Email.DisableEmailNotifications
			env.Config.Email.Type = "mailgun"
			env.Config.Email.DisableEmailNotifications = false
			t.Cleanup(func() {
				env.Config.Email.Type = previousType
				env.Config.Email.DisableEmailNotifications = previousDisabled
			})

			email.SetAllowlist("")
			mailgun.Service{}.Init()

			var users []*entity.User
			for _, id := range []int{2, 3} {
				user := &query.GetUserByID{UserID: id}
				if err := bus.Dispatch(f.ctx, user); err != nil {
					t.Fatal(err)
				}

				users = append(users, user.Result)
			}

			bus.AddHandler(func(_ context.Context, q *query.GetActiveSubscribers) error {
				if q.Channel != enum.NotificationChannelEmail {
					return nil
				}

				for _, user := range users {
					if len(q.UserIDs) == 0 {
						q.Result = append(q.Result, user)
						continue
					}

					for _, id := range q.UserIDs {
						if user.ID == id {
							q.Result = append(q.Result, user)
						}
					}
				}

				return nil
			})

			bus.AddHandler(func(_ context.Context, q *query.ListActiveWebhooksByType) error {
				return nil
			})

			var sizes []int
			accepted := make(map[string]int)
			rejectedAddress := fmt.Sprintf("%q <%s>", users[1].Name, users[1].Email)

			bus.AddHandler(func(_ context.Context, request *cmd.HTTPRequest) error {
				body, err := io.ReadAll(request.Body)
				if err != nil {
					return err
				}

				form, err := url.ParseQuery(string(body))
				if err != nil {
					return err
				}

				to := form["to"]
				sizes = append(sizes, len(to))

				if len(sizes) == 1 {
					if scenario.transport {
						return fmt.Errorf("injected transport interruption")
					}

					request.ResponseStatusCode = scenario.status
					request.ResponseBody = []byte(scenario.body)
					return nil
				}

				if scenario.isolate && len(to) == 1 && to[0] == rejectedAddress {
					request.ResponseStatusCode = 400
					request.ResponseBody = []byte(scenario.body)
					return nil
				}

				for _, recipient := range to {
					accepted[recipient]++
				}

				request.ResponseStatusCode = 200
				return nil
			})

			f.queuePostNotification(t)
			if _, err := tasks.DeliverPendingPostNotification(f.ctx); err != nil {
				t.Fatal(err)
			}

			if _, err := tasks.DeliverPendingPostNotification(f.ctx); err == nil {
				t.Fatal("expected provider rejection")
			}

			if _, err := dbx.Connection().Exec("UPDATE post_notification_recipients SET available_at = NOW()"); err != nil {
				t.Fatal(err)
			}

			for {
				found, err := tasks.DeliverPendingPostNotification(f.ctx)
				if err != nil {
					t.Fatal(err)
				}

				if !found {
					break
				}
			}

			if scenario.isolate {
				if !slices.Equal(sizes, []int{2, 1, 1}) {
					t.Fatalf("recipient rejection did not split the batch: requests=%v", sizes)
				}

				if len(accepted) != 1 {
					t.Fatalf("accepted recipients: got %d, want 1", len(accepted))
				}

				suppressed := workflowCount(t, `SELECT COUNT(*) FROM users
					WHERE id = 3 AND email_supressed_at IS NOT NULL`)
				if suppressed != 1 {
					t.Fatal("rejected recipient was not suppressed")
				}
			} else {
				if !slices.Equal(sizes, []int{2, 2}) {
					t.Fatalf("provider failure fragmented healthy recipients: requests=%v", sizes)
				}

				if len(accepted) != 2 {
					t.Fatalf("accepted recipients: got %d, want 2", len(accepted))
				}

				suppressed := workflowCount(t, `SELECT COUNT(*) FROM users
					WHERE id IN (2, 3) AND email_supressed_at IS NOT NULL`)
				if suppressed != 0 {
					t.Fatal("provider failure suppressed a recipient")
				}
			}

			for _, count := range accepted {
				if count != 1 {
					t.Fatal("acknowledged recipient repeated")
				}
			}

			pendingDeliveries := workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries")
			pendingRecipients := workflowCount(t, "SELECT COUNT(*) FROM post_notification_recipients")
			if pendingDeliveries != 0 || pendingRecipients != 0 {
				t.Fatal("finished deliveries were not reclaimed")
			}
		})
	}
}

func TestPostNotificationConcurrentEmailBatches(t *testing.T) {
	f := newPostWorkflow(t)
	f.queuePostNotification(t)

	prepare := func(context.Context, *entity.Post) ([]cmd.PostNotificationRecipient, error) {
		recipients := make([]cmd.PostNotificationRecipient, 1002)
		for i := range recipients {
			recipients[i] = cmd.PostNotificationRecipient{
				Channel: "email",
				ID:      i + 1,
			}
		}

		return recipients, nil
	}

	if err := bus.Dispatch(f.ctx, &cmd.ProcessPostNotification{Prepare: prepare}); err != nil {
		t.Fatal(err)
	}

	entered := make(chan []cmd.PostNotificationRecipient, 2)
	release := make(chan struct{})
	finished := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			finished <- bus.Dispatch(f.ctx, &cmd.ProcessPostNotification{
				EmailBatchSize: 1000,
				Send: func(_ context.Context, _ *entity.Post, recipients []cmd.PostNotificationRecipient) error {
					entered <- recipients
					<-release
					return nil
				},
			})
		}()
	}

	counts := make(map[int]int)
	for i := 0; i < 2; i++ {
		select {
		case recipients := <-entered:
			if len(recipients) > 1000 {
				close(release)
				t.Fatal("provider batch limit exceeded")
			}

			for _, recipient := range recipients {
				counts[recipient.ID]++
			}

		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("concurrent batches blocked before provider delivery")
		}
	}

	close(release)

	for i := 0; i < 2; i++ {
		if err := <-finished; err != nil {
			t.Fatal(err)
		}
	}

	if len(counts) != 1002 {
		t.Fatalf("concurrent batches delivered %d of 1002 recipients", len(counts))
	}

	for _, count := range counts {
		if count != 1 {
			t.Fatal("concurrent batches repeated a recipient")
		}
	}

	if workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries") != 0 {
		t.Fatal("concurrent batches retained completed work")
	}
}
