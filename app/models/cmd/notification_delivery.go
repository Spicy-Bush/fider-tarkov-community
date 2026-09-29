package cmd

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type ScheduleNotification struct {
	PostID     int
	CommentID  int
	MentionIDs []int
	Edited     bool
	BaseURL    string
}

type NotificationRecipient struct {
	Channel string
	ID      int
}

type ProcessNotification struct {
	Prepare        func(context.Context, *entity.NotificationDelivery) ([]NotificationRecipient, error)
	Send           func(context.Context, *entity.NotificationDelivery, []NotificationRecipient) error
	EmailBatchSize int
	Found          bool
}
