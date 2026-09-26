package tasks

import (
	"context"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webhook"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webpush"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
)

func preparePostNotifications(ctx context.Context, post *entity.Post) ([]cmd.PostNotificationRecipient, error) {
	author := ctx.Value(app.UserCtxKey).(*entity.User)
	users, err := getActiveSubscribers(ctx, post, enum.NotificationChannelWeb, enum.NotificationEventNewPost)
	if err != nil {
		return nil, err
	}

	title := i18n.T(ctx, "web.new_post.text", i18n.Params{
		"userName": author.Name,
		"title":    post.Title,
		"postLink": fmt.Sprintf("#%d", post.Number),
	})

	for _, user := range users {
		if user.ID != author.ID {
			notification := &cmd.AddNewNotification{
				User:   user,
				Title:  title,
				Link:   fmt.Sprintf("/posts/%d/%s", post.Number, post.Slug),
				PostID: post.ID,
			}

			if err := bus.Dispatch(ctx, notification); err != nil {
				return nil, err
			}
		}
	}

	var recipients []cmd.PostNotificationRecipient
	if !env.Config.Email.DisableEmailNotifications {
		users, err := getActiveSubscribers(ctx, post, enum.NotificationChannelEmail, enum.NotificationEventNewPost)
		if err != nil {
			return nil, err
		}

		for _, user := range users {
			if user.ID != author.ID {
				recipients = append(recipients, cmd.PostNotificationRecipient{
					Channel: "email",
					ID:      user.ID,
				})
			}
		}
	}

	if env.IsWebPushEnabled() {
		users, err := getActiveSubscribers(ctx, post, enum.NotificationChannelPush, enum.NotificationEventNewPost)
		if err != nil {
			return nil, err
		}

		ids := make([]int, 0, len(users))
		for _, user := range users {
			if user.ID != author.ID {
				ids = append(ids, user.ID)
			}
		}

		subscriptions := &query.GetPushSubscriptionsByUsers{UserIDs: ids}
		if err := bus.Dispatch(ctx, subscriptions); err != nil {
			return nil, err
		}

		for _, subscription := range subscriptions.Result {
			recipients = append(recipients, cmd.PostNotificationRecipient{
				Channel: "push",
				ID:      subscription.ID,
			})
		}
	}

	webhooks := &query.ListActiveWebhooksByType{Type: enum.WebhookNewPost}
	if err := bus.Dispatch(ctx, webhooks); err != nil {
		return nil, err
	}

	for _, hook := range webhooks.Result {
		recipients = append(recipients, cmd.PostNotificationRecipient{
			Channel: "webhook",
			ID:      hook.ID,
		})
	}

	return recipients, nil
}

func sendPostNotification(ctx context.Context, post *entity.Post, recipients []cmd.PostNotificationRecipient) error {
	recipient := recipients[0]
	author := ctx.Value(app.UserCtxKey).(*entity.User)
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	baseURL, logoURL := web.BaseURL(ctx), web.LogoURL(ctx)

	switch recipient.Channel {
	case "email":
		if env.Config.Email.DisableEmailNotifications {
			return nil
		}

		ids := make([]int, len(recipients))
		for i, recipient := range recipients {
			ids[i] = recipient.ID
		}

		users := &query.GetActiveSubscribers{
			Number:  post.Number,
			Channel: enum.NotificationChannelEmail,
			Event:   enum.NotificationEventNewPost,
			UserIDs: ids,
		}

		if err := bus.Dispatch(ctx, users); err != nil {
			return err
		}

		if len(users.Result) == 0 {
			return nil
		}

		to := make([]dto.Recipient, len(users.Result))
		for i, user := range users.Result {
			to[i] = dto.NewRecipient(user.Name, user.Email, dto.Props{})
		}

		mail := &cmd.SendMail{
			From:         dto.Recipient{Name: author.Name},
			To:           to,
			TemplateName: "new_post",
			Props: dto.Props{
				"title":    post.Title,
				"siteName": tenant.Name,
				"userName": author.Name,
				"content":  markdown.Full(post.Description),
				"postLink": linkWithText(fmt.Sprintf("#%d", post.Number), baseURL, "/posts/%d/%s", post.Number, post.Slug),
				"view":     linkWithText(i18n.T(ctx, "email.subscription.view"), baseURL, "/posts/%d/%s", post.Number, post.Slug),
				"change":   linkWithText(i18n.T(ctx, "email.subscription.change"), baseURL, "/profile#settings"),
				"logo":     logoURL,
			},
		}

		err := bus.Dispatch(ctx, mail)
		if len(users.Result) == 1 && email.IsRecipientRejected(err) {
			return bus.Dispatch(ctx, &cmd.SupressEmail{
				EmailAddresses: []string{users.Result[0].Email},
			})
		}

		return err

	case "push":
		if !env.IsWebPushEnabled() {
			return nil
		}

		q := &query.GetPushSubscription{ID: recipient.ID}
		err := bus.Dispatch(ctx, q)
		if errors.Cause(err) == app.ErrNotFound {
			return nil
		}

		if err != nil {
			return err
		}

		users := &query.GetActiveSubscribers{
			Number:  post.Number,
			Channel: enum.NotificationChannelPush,
			Event:   enum.NotificationEventNewPost,
			UserIDs: []int{q.Result.UserID},
		}

		if err := bus.Dispatch(ctx, users); err != nil {
			return err
		}

		if len(users.Result) == 0 {
			return nil
		}

		subscription := &webpush.Subscription{Endpoint: q.Result.Endpoint}
		subscription.Keys.P256dh = q.Result.KeyP256dh
		subscription.Keys.Auth = q.Result.KeyAuth

		notification := &webpush.Notification{
			Title: fmt.Sprintf("New post by %s", author.Name),
			Body:  truncateText(post.Title, 100),
			Icon:  baseURL + "/static/favicon?size=200",
			URL:   fmt.Sprintf("%s/posts/%d/%s", baseURL, post.Number, post.Slug),
			Tag:   fmt.Sprintf("post-%d", post.Number),
		}

		err = webpush.SendNotification(ctx, subscription, notification, 86400)
		if webpush.IsSubscriptionExpired(err) {
			return bus.Dispatch(ctx, &cmd.DeletePushSubscriptionByEndpoint{
				Endpoint: q.Result.Endpoint,
			})
		}

		return err

	case "webhook":
		props := webhook.Props{}
		props.SetPost(post, "post", baseURL, false, false)
		props.SetUser(author, "author")
		props.SetTenant(tenant, "tenant", baseURL, logoURL)

		return bus.Dispatch(ctx, &cmd.DeliverWebhook{
			ID:    recipient.ID,
			Props: props,
		})

	default:
		return errors.New("unknown notification channel: %s", recipient.Channel)
	}
}
