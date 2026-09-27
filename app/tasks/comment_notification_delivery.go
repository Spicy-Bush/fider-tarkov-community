package tasks

import (
	"context"
	"fmt"
	"html"
	"slices"

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

func prepareCommentNotifications(ctx context.Context, comment *entity.CommentNotification) ([]cmd.NotificationRecipient, error) {
	author := ctx.Value(app.UserCtxKey).(*entity.User)
	link := fmt.Sprintf("%s#comment-%d", comment.Owner.URL, comment.CommentID)
	var recipients []cmd.NotificationRecipient

	channels := []enum.NotificationChannel{enum.NotificationChannelWeb}
	if !env.Config.Email.DisableEmailNotifications {
		channels = append(channels, enum.NotificationChannelEmail)
	}

	if env.IsWebPushEnabled() {
		channels = append(channels, enum.NotificationChannelPush)
	}

	for _, channel := range channels {
		users := &query.GetCommentNotificationUsers{Notification: comment, Channel: channel}
		if err := bus.Dispatch(ctx, users); err != nil {
			return nil, err
		}

		switch channel {
		case enum.NotificationChannelWeb:
			for _, recipient := range users.Result {
				postID, pageID := 0, 0
				if comment.Owner.Kind == "post" {
					postID = comment.Owner.ID
				} else {
					pageID = comment.Owner.ID
				}

				messageKey := "web.new_comment.text"
				if slices.Contains(comment.MentionIDs, recipient.ID) {
					messageKey = "web.new_mention.text"
				}

				title := i18n.T(ctx, messageKey, i18n.Params{
					"userName": author.Name,
					"title":    comment.Owner.Title,
				})

				if err := bus.Dispatch(ctx, &cmd.AddNewNotification{
					User:      recipient,
					Title:     title,
					Link:      link,
					PostID:    postID,
					PageID:    pageID,
					CommentID: comment.CommentID,
				}); err != nil {
					return nil, err
				}
			}

		case enum.NotificationChannelEmail:
			for _, recipient := range users.Result {
				recipients = append(recipients, cmd.NotificationRecipient{Channel: "email", ID: recipient.ID})
			}

		case enum.NotificationChannelPush:
			ids := make([]int, len(users.Result))
			for index, recipient := range users.Result {
				ids[index] = recipient.ID
			}

			if len(ids) == 0 {
				continue
			}

			subscriptions := &query.GetPushSubscriptionsByUsers{UserIDs: ids}
			if err := bus.Dispatch(ctx, subscriptions); err != nil {
				return nil, err
			}

			for _, subscription := range subscriptions.Result {
				recipients = append(recipients, cmd.NotificationRecipient{Channel: "push", ID: subscription.ID})
			}
		}
	}

	if !comment.Edited {
		hooks := &query.ListActiveWebhooksByType{Type: enum.WebhookNewComment}
		if err := bus.Dispatch(ctx, hooks); err != nil {
			return nil, err
		}

		for _, hook := range hooks.Result {
			recipients = append(recipients, cmd.NotificationRecipient{Channel: "webhook", ID: hook.ID})
		}
	}

	return recipients, nil
}

func sendCommentNotification(ctx context.Context, comment *entity.CommentNotification, recipients []cmd.NotificationRecipient) error {
	recipient := recipients[0]
	author := ctx.Value(app.UserCtxKey).(*entity.User)
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	baseURL, logoURL := web.BaseURL(ctx), web.LogoURL(ctx)
	link := fmt.Sprintf("%s#comment-%d", comment.Owner.URL, comment.CommentID)

	switch recipient.Channel {
	case "email":
		if env.Config.Email.DisableEmailNotifications {
			return nil
		}

		ids := make([]int, len(recipients))
		for index, recipient := range recipients {
			ids[index] = recipient.ID
		}

		users := &query.GetCommentNotificationUsers{
			Notification: comment,
			Channel:      enum.NotificationChannelEmail,
			UserIDs:      ids,
		}
		if err := bus.Dispatch(ctx, users); err != nil {
			return err
		}

		if len(users.Result) == 0 {
			return nil
		}

		to := make([]dto.Recipient, len(users.Result))
		for index, recipient := range users.Result {
			messageKey := "email.new_comment.text"
			if slices.Contains(comment.MentionIDs, recipient.ID) {
				messageKey = "email.new_mention.text"
			}

			message := i18n.T(ctx, messageKey, i18n.Params{
				"userName": html.EscapeString(author.Name),
				"title":    html.EscapeString(comment.Owner.Title),
				"postLink": linkWithText(html.EscapeString(comment.Owner.Title), baseURL, "%s", link),
			})
			to[index] = dto.NewRecipient(recipient.Name, recipient.Email, dto.Props{"message": message})
		}

		mail := &cmd.SendMail{
			From:         dto.Recipient{Name: author.Name},
			To:           to,
			TemplateName: "new_comment",
			Props: dto.Props{
				"title":       comment.Owner.Title,
				"siteName":    tenant.Name,
				"content":     markdown.Full(markdown.StripMentionMetaData(comment.Content)),
				"view":        linkWithText(i18n.T(ctx, "email.subscription.view"), baseURL, "%s", link),
				"unsubscribe": linkWithText(i18n.T(ctx, "email.subscription.unsubscribe"), baseURL, "%s", comment.Owner.URL),
				"change":      linkWithText(i18n.T(ctx, "email.subscription.change"), baseURL, "/profile#settings"),
				"logo":        logoURL,
			},
		}

		err := bus.Dispatch(ctx, mail)
		if len(users.Result) == 1 && email.IsRecipientRejected(err) {
			return bus.Dispatch(ctx, &cmd.SupressEmail{EmailAddresses: []string{users.Result[0].Email}})
		}

		return err

	case "push":
		if !env.IsWebPushEnabled() {
			return nil
		}

		subscription := &query.GetPushSubscription{ID: recipient.ID}
		if err := bus.Dispatch(ctx, subscription); err != nil {
			if errors.Cause(err) == app.ErrNotFound {
				return nil
			}

			return err
		}

		users := &query.GetCommentNotificationUsers{
			Notification: comment,
			Channel:      enum.NotificationChannelPush,
			UserIDs:      []int{subscription.Result.UserID},
		}
		if err := bus.Dispatch(ctx, users); err != nil {
			return err
		}

		if len(users.Result) == 0 {
			return nil
		}

		target := &webpush.Subscription{Endpoint: subscription.Result.Endpoint}
		target.Keys.P256dh = subscription.Result.KeyP256dh
		target.Keys.Auth = subscription.Result.KeyAuth

		message := &webpush.Notification{
			Title: fmt.Sprintf("%s commented on %s", author.Name, comment.Owner.Title),
			Body:  truncateText(markdown.PlainText(comment.Content), 100),
			Icon:  baseURL + "/static/favicon?size=200",
			URL:   baseURL + link,
			Tag:   fmt.Sprintf("comment-%d", comment.CommentID),
		}
		if slices.Contains(comment.MentionIDs, subscription.Result.UserID) {
			message.Title = fmt.Sprintf("%s mentioned you", author.Name)
		}

		err := webpush.SendNotification(ctx, target, message, 86400)
		if webpush.IsSubscriptionExpired(err) {
			return bus.Dispatch(ctx, &cmd.DeletePushSubscriptionByEndpoint{Endpoint: subscription.Result.Endpoint})
		}

		return err

	case "webhook":
		props := webhook.Props{
			"comment": markdown.StripMentionMetaData(comment.Content),
			"comment_id": comment.CommentID,
			"comment_url": baseURL + link,
		}
		props.SetUser(author, "author")
		props.SetTenant(tenant, "tenant", baseURL, logoURL)

		if comment.Owner.Kind == "post" {
			post := &query.GetPostByID{PostID: comment.Owner.ID}
			if err := bus.Dispatch(ctx, post); err != nil {
				if errors.Cause(err) == app.ErrNotFound {
					return nil
				}

				return err
			}

			props.SetPost(post.Result, "post", baseURL, true, true)
		} else {
			props["page_id"] = comment.Owner.ID
			props["page_title"] = comment.Owner.Title
			props["page_url"] = baseURL + comment.Owner.URL
		}

		return bus.Dispatch(ctx, &cmd.DeliverWebhook{ID: recipient.ID, Props: props})

	default:
		return errors.New("unknown notification channel: %s", recipient.Channel)
	}
}
