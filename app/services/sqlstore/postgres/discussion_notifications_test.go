package postgres_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
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
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestDiscussionNotificationRecipients(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		t.Run(kind, func(t *testing.T) {
			SetupDatabaseTest(t)
			defer TeardownDatabaseTest()

			users := make(map[string]*entity.User)
			ids := make([]int, 0)
			for _, name := range []string{"author", "parent", "subscriber", "mentioned", "disabled", "unsubscribed", "default"} {
				user := &entity.User{Name: name, Email: name + "@discussion.test", Role: enum.RoleVisitor}
				if err := bus.Dispatch(jonSnowCtx, &cmd.RegisterUser{User: user}); err != nil {
					t.Fatal(err)
				}

				users[name] = user
				ids = append(ids, user.ID)
				if name == "default" {
					continue
				}

				settings := map[string]string{
					"event_notification_new_comment": "7",
					"event_notification_mention":     "7",
				}
				if name == "disabled" {
					settings["event_notification_new_comment"] = "0"
					settings["event_notification_mention"] = "0"
				}

				ctx := context.WithValue(jonSnowCtx, app.UserCtxKey, user)
				if err := bus.Dispatch(ctx, &cmd.UpdateCurrentUserSettings{Settings: settings}); err != nil {
					t.Fatal(err)
				}
			}

			create := &cmd.CreateComment{
				Content:      "A reply with several notification reasons",
				SubmissionID: "notification-reasons",
			}
			var owner entity.DiscussionOwner
			if kind == "post" {
				post := &cmd.AddNewPost{Title: "Notification recipients", Description: "Description"}
				if err := bus.Dispatch(jonSnowCtx, post); err != nil {
					t.Fatal(err)
				}

				create.PostNumber = post.Result.Number
				owner = entity.PostDiscussion(post.Result).Owner
				for _, name := range []string{"parent", "subscriber", "disabled", "default"} {
					if err := bus.Dispatch(jonSnowCtx, &cmd.AddSubscriber{Post: post.Result, User: users[name]}); err != nil {
						t.Fatal(err)
					}
				}

				if err := bus.Dispatch(jonSnowCtx, &cmd.RemoveSubscriber{Post: post.Result, User: users["unsubscribed"]}); err != nil {
					t.Fatal(err)
				}
			} else {
				page := &cmd.CreatePage{
					Title:         "Notification recipients",
					Slug:          "notification-recipients",
					Content:       "Page",
					Status:        entity.PageStatusPublished,
					Visibility:    entity.PageVisibilityPublic,
					AllowComments: true,
				}
				if err := bus.Dispatch(jonSnowCtx, page); err != nil {
					t.Fatal(err)
				}

				create.PageID = page.Result.ID
				owner = entity.PageDiscussion(page.Result).Owner
				for _, name := range []string{"parent", "subscriber", "disabled", "default"} {
					ctx := context.WithValue(jonSnowCtx, app.UserCtxKey, users[name])
					if err := bus.Dispatch(ctx, &cmd.TogglePageSubscription{PageID: page.Result.ID}); err != nil {
						t.Fatal(err)
					}
				}
			}

			authorCtx := context.WithValue(jonSnowCtx, app.UserCtxKey, users["author"])
			if err := bus.Dispatch(authorCtx, create); err != nil {
				t.Fatal(err)
			}

			event := &entity.CommentNotification{
				CommentID:      create.Result.ID,
				Owner:          owner,
				ParentAuthorID: users["parent"].ID,
				MentionIDs: []int{
					users["parent"].ID, users["mentioned"].ID, users["mentioned"].ID,
					users["disabled"].ID, users["author"].ID, tonyStark.ID,
				},
			}
			ids = append(ids, tonyStark.ID)

			check := func(channel enum.NotificationChannel, names ...string) {
				t.Helper()
				request := &query.GetCommentNotificationUsers{Notification: event, Channel: channel, UserIDs: ids}
				if err := bus.Dispatch(authorCtx, request); err != nil {
					t.Fatal(err)
				}

				actual := make([]string, 0, len(request.Result))
				for _, recipient := range request.Result {
					actual = append(actual, recipient.Name)
				}

				slices.Sort(actual)
				slices.Sort(names)
				if !slices.Equal(actual, names) {
					t.Fatalf("channel %d: expected %v, got %v", channel, names, actual)
				}
			}

			for _, channel := range []enum.NotificationChannel{enum.NotificationChannelWeb, enum.NotificationChannelEmail, enum.NotificationChannelPush} {
				check(channel, "parent", "subscriber", "mentioned")
			}

			if err := bus.Dispatch(authorCtx, &cmd.SupressEmail{EmailAddresses: []string{users["mentioned"].Email}}); err != nil {
				t.Fatal(err)
			}

			check(enum.NotificationChannelEmail, "parent", "subscriber")
			check(enum.NotificationChannelWeb, "parent", "subscriber", "mentioned")

			event.Edited = true
			check(enum.NotificationChannelWeb, "parent", "mentioned")
			event.Edited = false

			if kind == "page" {
				if _, err := trx.Execute("UPDATE pages SET visibility = 'private', allowed_roles = '[]' WHERE id = $1", owner.ID); err != nil {
					t.Fatal(err)
				}

				check(enum.NotificationChannelWeb)
				if _, err := trx.Execute("UPDATE pages SET allowed_roles = '[\"visitor\"]' WHERE id = $1", owner.ID); err != nil {
					t.Fatal(err)
				}

				check(enum.NotificationChannelWeb, "parent", "subscriber", "mentioned")
			}

			if err := bus.Dispatch(jonSnowCtx, &cmd.SetModerationPending{ContentType: "comment", ContentID: create.Result.ID, Pending: true}); err != nil {
				t.Fatal(err)
			}

			check(enum.NotificationChannelWeb)
		})
	}
}

func TestDiscussionNotificationDeliveryRecoversWithoutRepeatingHealthyRecipients(t *testing.T) {
	f := newPostWorkflow(t)
	previousEmail := env.Config.Email
	env.Config.Email.Type = "smtp"
	env.Config.Email.DisableEmailNotifications = false
	t.Cleanup(func() { env.Config.Email = previousEmail })

	page := &cmd.CreatePage{
		Title:         "Reliable discussion",
		Slug:          "reliable-discussion",
		Content:       "Page",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	parent := &entity.User{Name: "Parent", Email: "parent@discussion.test", Role: enum.RoleVisitor}
	subscriber := &entity.User{Name: "Subscriber", Email: "subscriber@discussion.test", Role: enum.RoleVisitor}
	for _, user := range []*entity.User{parent, subscriber} {
		if err := bus.Dispatch(f.ctx, &cmd.RegisterUser{User: user}); err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(f.ctx, app.UserCtxKey, user)
		if err := bus.Dispatch(ctx,
			&cmd.TogglePageSubscription{PageID: page.Result.ID},
			&cmd.UpdateCurrentUserSettings{Settings: map[string]string{
				"event_notification_new_comment": "3",
				"event_notification_mention":     "3",
			}},
		); err != nil {
			t.Fatal(err)
		}
	}

	var parentID int
	if err := dbx.Connection().QueryRow(`
		INSERT INTO comments (tenant_id, page_id, user_id, content, created_at)
		VALUES ($1, $2, $3, 'Original comment', NOW()) RETURNING id
	`, f.tenant.ID, page.Result.ID, parent.ID).Scan(&parentID); err != nil {
		t.Fatal(err)
	}

	create := &cmd.CreateComment{
		PageID:       page.Result.ID,
		ParentID:     &parentID,
		Content:      fmt.Sprintf(`Reply to @{"id":%d,"name":"Parent","isNew":false}`, parent.ID),
		SubmissionID: "notification-recovery",
		BaseURL:      "http://localhost:3000",
	}
	if err := bus.Dispatch(f.ctx, create); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, create); err != nil || create.Created {
		t.Fatalf("submission replay: created=%v error=%v", create.Created, err)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries"); count != 1 {
		t.Fatalf("retry scheduled %d deliveries", count)
	}

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveWebhooksByType) error {
		q.Result = []*entity.Webhook{{ID: 100}}
		return nil
	})

	unavailable := true
	mailAttempts := make(map[string]int)
	bus.AddHandler(func(ctx context.Context, mail *cmd.SendMail) error {
		if len(mail.To) != 1 || mail.TemplateName != "new_comment" {
			t.Fatalf("unexpected mail: %+v", mail)
		}

		recipient := mail.To[0]
		mailAttempts[recipient.Address]++
		message := fmt.Sprint(recipient.Props["message"])
		if recipient.Address == parent.Email && !strings.Contains(message, "mentioned") {
			t.Fatalf("parent mention lost its notification message: %q", message)
		}

		if !strings.Contains(fmt.Sprint(mail.Props["view"]), fmt.Sprintf("/pages/%s#comment-%d", page.Result.Slug, create.Result.ID)) {
			t.Fatalf("mail points at the wrong discussion: %v", mail.Props["view"])
		}

		if unavailable && recipient.Address == parent.Email {
			return errors.New("SMTP unavailable")
		}

		return nil
	})

	hookAttempts := 0
	bus.AddHandler(func(ctx context.Context, hook *cmd.DeliverWebhook) error {
		hookAttempts++
		if hook.Props["page_id"] != page.Result.ID || hook.Props["comment_id"] != create.Result.ID {
			t.Fatalf("webhook lost Page/comment identity: %v", hook.Props)
		}

		if unavailable {
			return errors.New("webhook unavailable")
		}

		return nil
	})

	deliver := func() {
		t.Helper()
		for attempt := 0; attempt < 20; attempt++ {
			found, err := tasks.DeliverPendingNotification(context.Background())
			if err != nil && !unavailable {
				t.Fatal(err)
			}

			if !found {
				return
			}
		}

		t.Fatal("notification queue did not settle")
	}

	deliver()
	if mailAttempts[parent.Email] != 1 || mailAttempts[subscriber.Email] != 1 || hookAttempts != 1 {
		t.Fatalf("first delivery attempts: mail=%v webhook=%d", mailAttempts, hookAttempts)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM notifications"); count != 2 {
		t.Fatalf("parent/subscriber/mention overlap created %d web notifications", count)
	}

	unavailable = false
	if _, err := dbx.Connection().Exec("UPDATE notification_recipients SET available_at = NOW()"); err != nil {
		t.Fatal(err)
	}

	deliver()
	if mailAttempts[parent.Email] != 2 || mailAttempts[subscriber.Email] != 1 || hookAttempts != 2 {
		t.Fatalf("recovery repeated a healthy recipient: mail=%v webhook=%d", mailAttempts, hookAttempts)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries"); count != 0 {
		t.Fatalf("recovery left %d deliveries", count)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM notifications"); count != 2 {
		t.Fatalf("recovery duplicated web notifications: %d", count)
	}

	edit := &cmd.UpdateComment{
		CommentID:    create.Result.ID,
		Content:      create.Content + fmt.Sprintf(` and @{"id":%d,"name":"Subscriber"}`, subscriber.ID),
		SubmissionID: "new-mention-edit",
		BaseURL:      "http://localhost:3000",
	}
	if err := bus.Dispatch(f.ctx, edit); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, edit); err != nil {
		t.Fatal(err)
	}

	deliver()
	if mailAttempts[parent.Email] != 2 || mailAttempts[subscriber.Email] != 2 || hookAttempts != 2 {
		t.Fatalf("edit notified users other than the new mention: mail=%v webhook=%d", mailAttempts, hookAttempts)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM notifications"); count != 3 {
		t.Fatalf("edit replay did not preserve one new notification: %d", count)
	}
}

func TestDiscussionNotificationInboxVisibility(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	page := &cmd.CreatePage{
		Title:         "Inbox visibility",
		Slug:          "inbox-visibility",
		Content:       "Page",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(jonSnowCtx, page); err != nil {
		t.Fatal(err)
	}

	comment := &cmd.CreateComment{
		PageID:       page.Result.ID,
		Content:      "A Page comment",
		SubmissionID: "page-comment",
	}
	if err := bus.Dispatch(jonSnowCtx, comment); err != nil {
		t.Fatal(err)
	}

	notification := &cmd.AddNewNotification{
		User:      aryaStark,
		Title:     "New Page comment",
		PageID:    page.Result.ID,
		CommentID: comment.Result.ID,
		Link: fmt.Sprintf("/pages/%s#comment-%d", page.Result.Slug, comment.Result.ID),
	}
	if err := bus.Dispatch(jonSnowCtx, notification); err != nil {
		t.Fatal(err)
	}

	check := func(want int) {
		t.Helper()
		count := &query.CountUnreadNotifications{}
		list := &query.GetActiveNotifications{Type: "unread", PerPage: 100}
		if err := bus.Dispatch(aryaStarkCtx, count, list); err != nil {
			t.Fatal(err)
		}

		if count.Result != want || list.TotalCount != want || len(list.Result) != want {
			t.Fatalf("expected %d visible notifications, got count=%d total=%d rows=%d", want, count.Result, list.TotalCount, len(list.Result))
		}

		get := &query.GetNotificationByID{ID: notification.Result.ID}
		err := bus.Dispatch(aryaStarkCtx, get)
		if (want == 0 && errors.Cause(err) != app.ErrNotFound) || (want == 1 && err != nil) {
			t.Fatalf("direct notification lookup disagrees with list: %v", err)
		}
	}

	check(1)
	if _, err := trx.Execute("UPDATE pages SET visibility = 'private', allowed_roles = '[]' WHERE id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}

	check(0)
	if _, err := trx.Execute("UPDATE pages SET visibility = 'public' WHERE id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}

	check(1)
	if err := bus.Dispatch(jonSnowCtx, &cmd.SetModerationPending{ContentType: "comment", ContentID: comment.Result.ID, Pending: true}); err != nil {
		t.Fatal(err)
	}

	check(0)
	if err := bus.Dispatch(jonSnowCtx, &cmd.SetModerationPending{ContentType: "comment", ContentID: comment.Result.ID, Pending: false}); err != nil {
		t.Fatal(err)
	}

	check(1)
	if err := bus.Dispatch(jonSnowCtx, &cmd.DeleteComment{CommentID: comment.Result.ID}); err != nil {
		t.Fatal(err)
	}

	check(0)
}

func TestDiscussionNotificationUsesEventPolicy(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	previousComment := enum.NotificationEventNewComment
	previousMention := enum.NotificationEventMention
	t.Cleanup(func() {
		enum.NotificationEventNewComment = previousComment
		enum.NotificationEventMention = previousMention
	})

	post := &cmd.AddNewPost{Title: "Notification policy", Description: "Shared event definitions"}
	if err := bus.Dispatch(jonSnowCtx, post); err != nil {
		t.Fatal(err)
	}
	comment := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		Content:      "Policy test",
		SubmissionID: "policy-test",
	}
	if err := bus.Dispatch(jonSnowCtx, comment); err != nil {
		t.Fatal(err)
	}

	event := &entity.CommentNotification{
		CommentID: comment.Result.ID,
		Owner: entity.PostDiscussion(post.Result).Owner,
		MentionIDs: []int{aryaStark.ID},
	}
	check := func(channel enum.NotificationChannel, want int) {
		t.Helper()
		users := &query.GetCommentNotificationUsers{Notification: event, Channel: channel, UserIDs: []int{aryaStark.ID}}
		if err := bus.Dispatch(jonSnowCtx, users); err != nil {
			t.Fatal(err)
		}
		if len(users.Result) != want {
			t.Fatalf("channel %d: got %d recipients, want %d", channel, len(users.Result), want)
		}
	}

	event.Edited = true
	enum.NotificationEventMention.DefaultSettingValue = "4"
	check(enum.NotificationChannelWeb, 0)
	check(enum.NotificationChannelPush, 1)
	enum.NotificationEventMention.DefaultEnabledUserRoles = []enum.Role{enum.RoleAdministrator}
	check(enum.NotificationChannelPush, 0)

	event.Edited = false
	event.MentionIDs = nil
	enum.NotificationEventNewComment.DefaultSettingValue = "4"
	enum.NotificationEventNewComment.DefaultEnabledUserRoles = []enum.Role{enum.RoleVisitor}
	enum.NotificationEventNewComment.RequiresSubscriptionUserRoles = []enum.Role{enum.RoleHelper}
	check(enum.NotificationChannelPush, 1)
	check(enum.NotificationChannelEmail, 0)
	enum.NotificationEventNewComment.RequiresSubscriptionUserRoles = []enum.Role{enum.RoleVisitor}
	check(enum.NotificationChannelPush, 0)

	if err := bus.Dispatch(jonSnowCtx, &cmd.AddSubscriber{Post: post.Result, User: aryaStark}); err != nil {
		t.Fatal(err)
	}
	check(enum.NotificationChannelPush, 1)
}
