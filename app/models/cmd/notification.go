package cmd

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type MarkAllNotificationsAsRead struct{}

type PurgeExpiredNotifications struct {
	NumOfDeletedNotifications int
}

type PurgeReadNotifications struct {
	NumOfPurgedNotifications int
}

type MarkNotificationAsRead struct {
	ID int
}

type AddNewNotification struct {
	User   *entity.User
	Title  string
	Link   string
	PostID int

	Result *entity.Notification
}

type AddSubscriber struct {
	Post *entity.Post
	User *entity.User
}

type RemoveSubscriber struct {
	Post *entity.Post
	User *entity.User
}

type SupressEmail struct {
	EmailAddresses []string

	//Output
	NumOfSupressedEmailAddresses int
}
