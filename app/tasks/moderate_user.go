package tasks

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

// NotifyAboutMute sends notifications when a user is muted
func NotifyAboutMute(mutedUser *entity.User, reason string, expiresAt *time.Time) worker.Task {
	return describe("Notify about user mute", func(c *worker.Context) error {
		// Get users who have notification enabled for mute event
		q := &query.GetUsersToNotify{
			Event:   enum.NotificationEventMute,
			Channel: enum.NotificationChannelWeb,
		}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		title := i18n.T(c, "web.user_muted.text", i18n.Params{
			"userName": mutedUser.Name,
			"reason":   reason,
		})
		link := "/profile#standing"

		err := bus.Dispatch(c, &cmd.AddNewNotification{
			User:   mutedUser,
			Title:  title,
			Link:   link,
			PostID: 0, // No post associated with this notification
		})
		if err != nil {
			return c.Failure(err)
		}

		// TODO: email sending
		/*
			mailProps := dto.Props{
				"userName":  mutedUser.Name,
				"reason":    reason,
				"expiresAt": expiresAt,
				"link":      link,
			}

			err = bus.Dispatch(c, &cmd.SendMail{
				From:         dto.Recipient{Name: c.Tenant().Name},
				To:           []dto.Recipient{{Name: mutedUser.Name, Address: mutedUser.Email}},
				TemplateName: "user_muted",
				Props:        mailProps,
			})
			if err != nil {
				return c.Failure(err)
			}
		*/

		return nil
	})
}

// NotifyAboutWarning sends notifications when a user is warned
func NotifyAboutWarning(warnedUser *entity.User, reason string, expiresAt *time.Time) worker.Task {
	return describe("Notify about user warning", func(c *worker.Context) error {
		// Get users who have notification enabled for warning event
		q := &query.GetUsersToNotify{
			Event:   enum.NotificationEventWarning,
			Channel: enum.NotificationChannelWeb,
		}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		title := i18n.T(c, "web.user_warned.text", i18n.Params{
			"userName": warnedUser.Name,
			"reason":   reason,
		})
		link := "/profile#standing"

		err := bus.Dispatch(c, &cmd.AddNewNotification{
			User:   warnedUser,
			Title:  title,
			Link:   link,
			PostID: 0, // No post associated with this notification
		})
		if err != nil {
			return c.Failure(err)
		}

		// TODO: email sending
		/*
			mailProps := dto.Props{
				"userName":  warnedUser.Name,
				"reason":    reason,
				"expiresAt": expiresAt,
				"link":      link,
			}

			err = bus.Dispatch(c, &cmd.SendMail{
				From:         dto.Recipient{Name: c.Tenant().Name},
				To:           []dto.Recipient{{Name: warnedUser.Name, Address: warnedUser.Email}},
				TemplateName: "user_warned",
				Props:        mailProps,
			})
			if err != nil {
				return c.Failure(err)
			}
		*/

		return nil
	})
}
