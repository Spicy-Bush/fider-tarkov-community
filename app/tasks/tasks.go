package tasks

import (
	"context"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webpush"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

func describe(name string, job worker.Job) worker.Task {
	return worker.Task{Name: name, Job: job}
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func link(baseURL, path string, args ...any) string {
	return fmt.Sprintf("<a href='%[1]s%[2]s'>%[1]s%[2]s</a>", baseURL, fmt.Sprintf(path, args...))
}

func linkWithText(text, baseURL, path string, args ...any) string {
	return fmt.Sprintf("<a href='%s%s'>%s</a>", baseURL, fmt.Sprintf(path, args...), text)
}

func getActiveSubscribers(ctx context.Context, post *entity.Post, channel enum.NotificationChannel, event enum.NotificationEvent) ([]*entity.User, error) {
	q := &query.GetActiveSubscribers{
		Number:  post.Number,
		Channel: channel,
		Event:   event,
	}
	err := bus.Dispatch(ctx, q)
	return q.Result, err
}

func sendPushNotifications(ctx context.Context, users []*entity.User, excludeUserID int, title, body, url, icon, tag string) {
	if !env.IsWebPushEnabled() || len(users) == 0 {
		return
	}

	userIDs := make([]int, 0, len(users))
	for _, user := range users {
		if user.ID != excludeUserID {
			userIDs = append(userIDs, user.ID)
		}
	}

	if len(userIDs) == 0 {
		return
	}

	q := &query.GetPushSubscriptionsByUsers{UserIDs: userIDs}
	if err := bus.Dispatch(ctx, q); err != nil {
		log.Error(ctx, err)
		return
	}

	if len(q.Result) == 0 {
		return
	}

	notification := &webpush.Notification{
		Title: title,
		Body:  body,
		Icon:  icon,
		URL:   url,
		Tag:   tag,
	}

	for _, sub := range q.Result {
		pushSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
		}
		pushSub.Keys.P256dh = sub.KeyP256dh
		pushSub.Keys.Auth = sub.KeyAuth

		err := webpush.SendNotification(ctx, pushSub, notification, 86400)
		if err != nil {
			if webpush.IsSubscriptionExpired(err) {
				bus.Dispatch(ctx, &cmd.DeletePushSubscriptionByEndpoint{
					Endpoint: sub.Endpoint,
				})
			} else {
				log.Warn(ctx, fmt.Sprintf("failed to send push notification: %s", err.Error()))
			}
		}
	}
}
