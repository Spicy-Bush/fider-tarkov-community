package tasks

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func DeliverPendingNotification(ctx context.Context) (bool, error) {
	operation := &cmd.ProcessNotification{
		Prepare: prepareNotifications,
		Send:    sendNotification,
	}

	if env.Config.Email.Type == "mailgun" {
		operation.EmailBatchSize = 1000
	}

	err := bus.Dispatch(ctx, operation)
	return operation.Found, err
}

func prepareNotifications(ctx context.Context, event *entity.NotificationDelivery) ([]cmd.NotificationRecipient, error) {
	if event.Post != nil {
		return preparePostNotifications(ctx, event.Post)
	}

	return prepareCommentNotifications(ctx, event.Comment)
}

func sendNotification(ctx context.Context, event *entity.NotificationDelivery, recipients []cmd.NotificationRecipient) error {
	if event.Post != nil {
		return sendPostNotification(ctx, event.Post, recipients)
	}

	return sendCommentNotification(ctx, event.Comment, recipients)
}
