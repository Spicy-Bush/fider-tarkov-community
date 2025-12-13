package tasks

import (
	"fmt"
	"strconv"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webhook"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

// NotifyAboutNewComment sends a notification (web and email) to subscribers
func NotifyAboutNewComment(comment *entity.Comment, post *entity.Post) worker.Task {
	return describe("Notify about new comment", func(c *worker.Context) error {
		comment.ParseMentions()

		// Web notification
		users, err := getActiveSubscribers(c, post, enum.NotificationChannelWeb, enum.NotificationEventNewComment)
		if err != nil {
			return c.Failure(err)
		}

		author := c.User()
		title := i18n.T(c, "web.new_comment.text", i18n.Params{
			"userName": author.Name,
			"title":    post.Title,
			"postLink": fmt.Sprintf("#%d", post.Number),
		})
		link := fmt.Sprintf("/posts/%d/%s#comment-%d", post.Number, post.Slug, comment.ID)
		for _, user := range users {
			if user.ID != author.ID {
				err = bus.Dispatch(c, &cmd.AddNewNotification{
					User:   user,
					Title:  title,
					Link:   link,
					PostID: post.ID,
				})
				if err != nil {
					return c.Failure(err)
				}
			}
		}

		// Web notification - mentions
		if comment.Mentions != nil {
			title = i18n.T(c, "web.new_mention.text", i18n.Params{
				"userName": author.Name,
				"title":    post.Title,
				"postLink": fmt.Sprintf("#%d", post.Number),
			})

			q := &query.GetUsersToNotify{
				Event:   enum.NotificationEventMention,
				Channel: enum.NotificationChannelWeb,
			}
			err = bus.Dispatch(c, q)
			if err != nil {
				return c.Failure(err)
			}

			// Iterate the mentions
			for _, mention := range comment.Mentions {
				// Check if the user is in the list of mention subscribers (users)
				for _, u := range q.Result {
					if u.ID == mention.ID && mention.IsNew {
						err = bus.Dispatch(c, &cmd.AddNewNotification{
							User:   u,
							Title:  title,
							Link:   link,
							PostID: post.ID,
						})
						if err != nil {
							return c.Failure(err)
						}
					}
				}
			}
		}

		strippedContent := markdown.StripMentionMetaData(comment.Content)

		// Standard email notitifications
		users, err = getActiveSubscribers(c, post, enum.NotificationChannelEmail, enum.NotificationEventNewComment)
		if err != nil {
			return c.Failure(err)
		}

		to := make([]dto.Recipient, 0)
		for _, user := range users {
			if user.ID != author.ID {
				to = append(to, dto.NewRecipient(user.Name, user.Email, dto.Props{}))
			}
		}

		sendEmailNotifications(c, post, to, strippedContent, enum.NotificationEventNewComment, comment.ID)

		// Mentions
		to = make([]dto.Recipient, 0)
		if comment.Mentions != nil {
			q := &query.GetUsersToNotify{
				Event:   enum.NotificationEventMention,
				Channel: enum.NotificationChannelEmail,
			}
			err = bus.Dispatch(c, q)
			if err != nil {
				return c.Failure(err)
			}

			// Iterate the mentions
			for _, mention := range comment.Mentions {
				// Check if the user is in the list of users with mention notifications enabled
				for _, u := range q.Result {
					if u.ID == mention.ID && mention.IsNew {
						to = append(to, dto.NewRecipient(u.Name, u.Email, dto.Props{}))
						break
					}
				}
			}
		}

		sendEmailNotifications(c, post, to, strippedContent, enum.NotificationEventMention, comment.ID)

		tenant := c.Tenant()
		baseURL, logoURL := web.BaseURL(c), web.LogoURL(c)

		webhookProps := webhook.Props{"comment": strippedContent}
		webhookProps.SetPost(post, "post", baseURL, true, true)
		webhookProps.SetUser(author, "author")
		webhookProps.SetTenant(tenant, "tenant", baseURL, logoURL)

		err = bus.Dispatch(c, &cmd.TriggerWebhooks{
			Type:  enum.WebhookNewComment,
			Props: webhookProps,
		})
		if err != nil {
			return c.Failure(err)
		}

		return nil
	})
}

func NotifyAboutUpdatedComment(content string, post *entity.Post, commentID int) worker.Task {
	return describe("Notify about updated comment", func(c *worker.Context) error {
		contentString := entity.CommentString(content)
		mentions := contentString.ParseMentions()

		log.Infof(c, "Comment updated: @{Comment:Yellow}. Mentions @{MentionsCount}", dto.Props{
			"Comment":       contentString,
			"MentionsCount": len(mentions),
		})

		author := c.User()
		title := i18n.T(c, "web.new_mention.text", i18n.Params{
			"userName": author.Name,
			"title":    post.Title,
			"postLink": fmt.Sprintf("#%d", post.Number),
		})
		link := fmt.Sprintf("/posts/%d/%s#comment-%d", post.Number, post.Slug, commentID)
		if mentions != nil {
			q := &query.GetUsersToNotify{
				Event:   enum.NotificationEventMention,
				Channel: enum.NotificationChannelWeb,
			}
			err := bus.Dispatch(c, q)
			if err != nil {
				return c.Failure(err)
			}

			// Iterate the mentions
			for _, mention := range mentions {
				// Check if the user is in the list of users with mention notifications enabled
				for _, u := range q.Result {
					if u.ID == mention.ID && mention.IsNew {
						err = bus.Dispatch(c, &cmd.AddNewNotification{
							User:   u,
							Title:  title,
							Link:   link,
							PostID: post.ID,
						})
						if err != nil {
							return c.Failure(err)
						}
					}
				}
			}
		}

		strippedContent := markdown.StripMentionMetaData(content)

		to := make([]dto.Recipient, 0)
		if mentions != nil {
			q := &query.GetUsersToNotify{
				Event:   enum.NotificationEventMention,
				Channel: enum.NotificationChannelEmail,
			}
			err := bus.Dispatch(c, q)
			if err != nil {
				return c.Failure(err)
			}

			for _, mention := range mentions {
				// Check if the user is in the list of mention subscribers (users)
				for _, u := range q.Result {
					if u.ID == mention.ID && mention.IsNew {
						to = append(to, dto.NewRecipient(u.Name, u.Email, dto.Props{}))
						break
					}
				}
			}
		}

		sendEmailNotifications(c, post, to, strippedContent, enum.NotificationEventMention, commentID)

		return nil
	})
}

func sendEmailNotifications(c *worker.Context, post *entity.Post, to []dto.Recipient, comment string, event enum.NotificationEvent, commentID int) {
	if env.Config.Email.DisableEmailNotifications {
		return
	}

	// Short circuit if there is no one to notify
	if len(to) == 0 {
		return
	}

	author := c.User()
	tenant := c.Tenant()
	baseURL, logoURL := web.BaseURL(c), web.LogoURL(c)
	messaleLocaleString := "email.new_comment.text"
	if event.UserSettingsKeyName == enum.NotificationEventMention.UserSettingsKeyName {
		messaleLocaleString = "email.new_mention.text"
	}

	mailProps := dto.Props{
		"title":               post.Title,
		"messageLocaleString": messaleLocaleString,
		"siteName":            tenant.Name,
		"userName":            author.Name,
		"content":             markdown.Full(comment),
	}

	path := "/posts/%d/%s"
	if commentID > 0 {
		commentFragment := "#comment-" + strconv.Itoa(commentID)
		path = "/posts/%d/%s" + commentFragment
	}

	mailProps["postLink"] = linkWithText(fmt.Sprintf("#%d", post.Number), baseURL, path, post.Number, post.Slug)
	mailProps["view"] = linkWithText(i18n.T(c, "email.subscription.view"), baseURL, path, post.Number, post.Slug)
	mailProps["unsubscribe"] = linkWithText(i18n.T(c, "email.subscription.unsubscribe"), baseURL, path, post.Number, post.Slug)
	mailProps["change"] = linkWithText(i18n.T(c, "email.subscription.change"), baseURL, "/profile#settings")
	mailProps["logo"] = logoURL

	bus.Publish(c, &cmd.SendMail{
		From:         dto.Recipient{Name: author.Name},
		To:           to,
		TemplateName: "new_comment",
		Props:        mailProps,
	})
}
