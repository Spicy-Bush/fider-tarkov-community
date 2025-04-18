package tasks

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webhook"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

// NotifyAboutDeletedPost sends a notification (web and email) to subscribers of the post that has been deleted
func NotifyAboutDeletedPost(post *entity.Post, deleteCommentAdded bool) worker.Task {
	return describe("Notify about deleted post", func(c *worker.Context) error {

		tenant := c.Tenant()
		baseURL, logoURL := web.BaseURL(c), web.LogoURL(c)
		author := c.User()

		// Webhook
		webhookProps := webhook.Props{}
		webhookProps.SetPost(post, "post", baseURL, true, true)
		webhookProps.SetUser(author, "author")
		webhookProps.SetTenant(tenant, "tenant", baseURL, logoURL)

		err := bus.Dispatch(c, &cmd.TriggerWebhooks{
			Type:  enum.WebhookDeletePost,
			Props: webhookProps,
		})
		if err != nil {
			return c.Failure(err)
		}

		// If no comment was added, we can stop here
		// (I'm not sure about the rational of this business rule, but this is how it currently works)
		if !deleteCommentAdded {
			return nil
		}

		// Web notification
		users, err := getActiveSubscribers(c, post, enum.NotificationChannelWeb, enum.NotificationEventChangeStatus)
		if err != nil {
			return c.Failure(err)
		}

		// Get localized title after database operations
		title := i18n.T(c, "web.delete_post.text", i18n.Params{
			"userName": author.Name,
			"title":    post.Title,
		})

		for _, user := range users {
			if user.ID != author.ID {
				err = bus.Dispatch(c, &cmd.AddNewNotification{
					User:   user,
					Title:  title,
					PostID: post.ID,
				})
				if err != nil {
					return c.Failure(err)
				}
			}
		}

		// Email notification
		if !env.Config.Email.DisableEmailNotifications {
			users, err = getActiveSubscribers(c, post, enum.NotificationChannelEmail, enum.NotificationEventChangeStatus)
			if err != nil {
				return c.Failure(err)
			}

			to := make([]dto.Recipient, 0)
			for _, user := range users {
				if user.ID != author.ID {
					to = append(to, dto.NewRecipient(user.Name, user.Email, dto.Props{}))
				}
			}

			props := dto.Props{
				"title":    post.Title,
				"siteName": tenant.Name,
				"content":  markdown.Full(post.Response.Text),
				"change":   linkWithText(i18n.T(c, "email.subscription.change"), baseURL, "/settings"),
				"logo":     logoURL,
			}

			bus.Publish(c, &cmd.SendMail{
				From:         dto.Recipient{Name: c.User().Name},
				To:           to,
				TemplateName: "delete_post",
				Props:        props,
			})
		}

		return nil
	})
}
