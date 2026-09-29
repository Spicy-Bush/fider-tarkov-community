package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestNotificationRecoveryUsesCurrentContent(t *testing.T) {
	for _, kind := range []string{"post", "comment", "page-comment"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			post := &cmd.AddNewPost{Title: "Original title", Description: "Original text"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}

			commentID := 0
			pageID := 0
			if kind == "post" {
				if err := bus.Dispatch(f.ctx, &cmd.ScheduleNotification{PostID: post.Result.ID, BaseURL: "http://localhost:3000"}); err != nil {
					t.Fatal(err)
				}
			} else {
				create := &cmd.CreateComment{
					PostNumber:   post.Result.Number,
					Content:      "Original text",
					SubmissionID: "current-content",
					BaseURL:      "http://localhost:3000",
				}
				if kind == "page-comment" {
					page := &cmd.CreatePage{
						Title:         "Original title",
						Slug:          "current-notification",
						Status:        entity.PageStatusPublished,
						Visibility:    entity.PageVisibilityPrivate,
						AllowComments: true,
					}
					if err := bus.Dispatch(f.ctx, page); err != nil {
						t.Fatal(err)
					}

					pageID = page.Result.ID
					create.PageID = pageID
					create.PostNumber = 0
				}

				if err := bus.Dispatch(f.ctx, create); err != nil {
					t.Fatal(err)
				}
				commentID = create.Result.ID
			}

			prepared := 0
			attempts := 0
			unavailable := true
			operation := &cmd.ProcessNotification{
				Prepare: func(context.Context, *entity.NotificationDelivery) ([]cmd.NotificationRecipient, error) {
					prepared++
					return []cmd.NotificationRecipient{{Channel: "email", ID: 2}}, nil
				},
				Send: func(_ context.Context, event *entity.NotificationDelivery, _ []cmd.NotificationRecipient) error {
					attempts++
					if unavailable {
						return errors.New("provider unavailable")
					}

					if kind == "post" {
						if event.Post.Title != "Current title" || event.Post.Description != "Current text" {
							t.Fatalf("replayed stale post: %+v", event.Post)
						}
					} else if event.Comment.Content != "Current text" || event.Comment.Owner.Title != "Current title" {
						t.Fatalf("replayed stale comment: %+v", event.Comment)
					}

					return nil
				},
			}
			if _, err := mediaFixtureSQL("UPDATE posts SET moderation_pending = TRUE WHERE id = $1", post.Result.ID); err != nil {
				t.Fatal(err)
			}
			if commentID != 0 {
				if _, err := mediaFixtureSQL("UPDATE comments SET moderation_pending = TRUE WHERE id = $1", commentID); err != nil {
					t.Fatal(err)
				}
			}
			if err := bus.Dispatch(f.ctx, operation); err != nil || prepared != 0 {
				t.Fatalf("hidden work was prepared: count=%d error=%v", prepared, err)
			}

			if _, err := mediaFixtureSQL("UPDATE posts SET moderation_pending = FALSE WHERE id = $1", post.Result.ID); err != nil {
				t.Fatal(err)
			}
			if commentID != 0 {
				if _, err := mediaFixtureSQL("UPDATE comments SET moderation_pending = FALSE WHERE id = $1", commentID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := dbx.Connection().Exec("UPDATE notification_deliveries SET available_at = NOW()"); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(f.ctx, operation); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(f.ctx, operation); err == nil {
				t.Fatal("provider failure was discarded")
			}

			if _, err := mediaFixtureSQL("UPDATE posts SET title = 'Current title', description = 'Current text', moderation_pending = TRUE WHERE id = $1", post.Result.ID); err != nil {
				t.Fatal(err)
			}
			if commentID != 0 {
				if _, err := mediaFixtureSQL("UPDATE comments SET content = 'Current text', moderation_pending = TRUE WHERE id = $1", commentID); err != nil {
					t.Fatal(err)
				}
			}
			if pageID != 0 {
				if _, err := mediaFixtureSQL("UPDATE pages SET title = 'Current title' WHERE id = $1", pageID); err != nil {
					t.Fatal(err)
				}
			}

			unavailable = false
			if _, err := dbx.Connection().Exec("UPDATE notification_recipients SET available_at = NOW()"); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(f.ctx, operation); err != nil {
				t.Fatal(err)
			}
			if attempts != 1 || prepared != 1 || workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries") != 1 {
				t.Fatalf("hidden work was sent or discarded: attempts=%d prepared=%d", attempts, prepared)
			}
			if err := bus.Dispatch(f.ctx, operation); err != nil || operation.Found {
				t.Fatalf("hidden work was immediately reclaimed: found=%v error=%v", operation.Found, err)
			}

			if _, err := mediaFixtureSQL("UPDATE posts SET moderation_pending = FALSE WHERE id = $1", post.Result.ID); err != nil {
				t.Fatal(err)
			}
			if commentID != 0 {
				if _, err := mediaFixtureSQL("UPDATE comments SET moderation_pending = FALSE WHERE id = $1", commentID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := dbx.Connection().Exec("UPDATE notification_deliveries SET available_at = NOW()"); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(f.ctx, operation); err != nil {
				t.Fatal(err)
			}
			if attempts != 2 || prepared != 1 || workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries") != 0 {
				t.Fatalf("approval did not complete the pending work: attempts=%d prepared=%d", attempts, prepared)
			}
		})
	}
}

func TestEditedMentionRetryChecksCurrentTextAndAccess(t *testing.T) {
	previousEmail := env.Config.Email
	env.Config.Email.Type = "smtp"
	env.Config.Email.DisableEmailNotifications = false
	t.Cleanup(func() { env.Config.Email = previousEmail })

	for _, change := range []string{"mention removed", "Page access revoked"} {
		t.Run(change, func(t *testing.T) {
			f := newPostWorkflow(t)
			page := &cmd.CreatePage{
				Title:         "Mention recovery",
				Slug:          "mention-recovery",
				Status:        entity.PageStatusPublished,
				Visibility:    entity.PageVisibilityPublic,
				AllowComments: true,
			}
			if err := bus.Dispatch(f.ctx, page); err != nil {
				t.Fatal(err)
			}

			comment := &cmd.CreateComment{
				PageID:       page.Result.ID,
				Content:      "Original reply",
				SubmissionID: "mention-original",
				BaseURL:      "http://localhost:3000",
			}
			if err := bus.Dispatch(f.ctx, comment); err != nil {
				t.Fatal(err)
			}
			if _, err := dbx.Connection().Exec("DELETE FROM notification_deliveries"); err != nil {
				t.Fatal(err)
			}

			recipient := &query.GetUserByID{UserID: 2}
			if err := bus.Dispatch(f.ctx, recipient); err != nil {
				t.Fatal(err)
			}
			recipientCtx := context.WithValue(f.ctx, app.UserCtxKey, recipient.Result)
			if err := bus.Dispatch(recipientCtx, &cmd.UpdateCurrentUserSettings{
				Settings: map[string]string{"event_notification_mention": "3"},
			}); err != nil {
				t.Fatal(err)
			}

			edit := &cmd.UpdateComment{
				CommentID:    comment.Result.ID,
				Content:      fmt.Sprintf(`Now mentioning @{"id":%d,"name":"Recipient"}`, recipient.Result.ID),
				SubmissionID: "mention-added",
				BaseURL:      "http://localhost:3000",
			}
			if err := bus.Dispatch(f.ctx, edit); err != nil {
				t.Fatal(err)
			}

			attempts := 0
			bus.AddHandler(func(context.Context, *cmd.SendMail) error {
				attempts++
				return errors.New("provider unavailable")
			})
			bus.AddHandler(func(context.Context, *query.ListActiveWebhooksByType) error {
				t.Fatal("edited mention attempted a new-comment webhook")
				return nil
			})

			if _, err := tasks.DeliverPendingNotification(f.ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := tasks.DeliverPendingNotification(f.ctx); err == nil || attempts != 1 {
				t.Fatalf("provider failure: attempts=%d error=%v", attempts, err)
			}

			if change == "mention removed" {
				if _, err := mediaFixtureSQL("UPDATE comments SET content = 'Mention removed' WHERE id = $1", comment.Result.ID); err != nil {
					t.Fatal(err)
				}
			} else if _, err := mediaFixtureSQL("UPDATE pages SET visibility = 'private', allowed_roles = '[]' WHERE id = $1", page.Result.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := dbx.Connection().Exec("UPDATE notification_recipients SET available_at = NOW()"); err != nil {
				t.Fatal(err)
			}

			if found, err := tasks.DeliverPendingNotification(f.ctx); !found || err != nil {
				t.Fatalf("retry: found=%v error=%v", found, err)
			}
			if attempts != 1 || workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries") != 0 {
				t.Fatalf("obsolete mention was sent or retained: attempts=%d", attempts)
			}
			if count := workflowCount(t, "SELECT COUNT(*) FROM notifications"); count != 1 {
				t.Fatalf("retry duplicated the delivered inbox notification: %d", count)
			}
		})
	}
}
