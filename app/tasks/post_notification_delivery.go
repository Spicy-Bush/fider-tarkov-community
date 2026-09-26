package tasks

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func DeliverPendingPostNotification(ctx context.Context) (bool, error) {
	operation := &cmd.ProcessPostNotification{
		Prepare: preparePostNotifications,
		Send:    sendPostNotification,
	}

	if env.Config.Email.Type == "mailgun" {
		operation.EmailBatchSize = 1000
	}

	err := bus.Dispatch(ctx, operation)
	return operation.Found, err
}
