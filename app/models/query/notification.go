package query

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type CountUnreadNotifications struct {
	Result int
}

type GetNotificationByID struct {
	ID     int
	Result *entity.Notification
}

type GetActiveNotifications struct {
	Result     []*entity.Notification
	Type       string
	Page       int
	PerPage    int
	TotalCount int
}

type GetActiveSubscribers struct {
	Number  int
	Channel enum.NotificationChannel
	Event   enum.NotificationEvent

	Result []*entity.User
}

type PurgeReadNotifications struct {
	NumOfPurgedNotifications int
}

// GetUsersToNotify represents a query to get users who should receive notifications
type GetUsersToNotify struct {
	Event   enum.NotificationEvent
	Channel enum.NotificationChannel
	Result  []*entity.User
}
