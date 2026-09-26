package cmd

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type SchedulePostNotification struct {
	Post    *entity.Post
	BaseURL string
}

type PostNotificationRecipient struct {
	Channel string
	ID      int
}

type ProcessPostNotification struct {
	Prepare        func(context.Context, *entity.Post) ([]PostNotificationRecipient, error)
	Send           func(context.Context, *entity.Post, []PostNotificationRecipient) error
	EmailBatchSize int
	Found          bool
}
