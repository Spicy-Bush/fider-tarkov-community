package tasks_test

import (
	"context"
	"html/template"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/webhook"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email/emailmock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestNotifyAboutDeletePostTask(t *testing.T) {
	RegisterT(t)
	bus.Init(emailmock.Service{})

	var addNewNotification *cmd.AddNewNotification
	bus.AddHandler(func(ctx context.Context, c *cmd.AddNewNotification) error {
		addNewNotification = c
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetActiveSubscribers) error {
		q.Result = []*entity.User{
			mock.AryaStark,
		}
		return nil
	})

	var triggerWebhooks *cmd.TriggerWebhooks
	bus.AddHandler(func(ctx context.Context, c *cmd.TriggerWebhooks) error {
		triggerWebhooks = c
		return nil
	})

	worker := mock.NewWorker()
	post := &entity.Post{
		ID:          1,
		Number:      1,
		Title:       "Add support for TypeScript",
		Slug:        "add-support-for-typescript",
		Description: "TypeScript is great, please add support for it",
		User:        mock.AryaStark,
		Status:      enum.PostDeleted,
		Response: &entity.PostResponse{
			RespondedAt: time.Now(),
			Text:        "Invalid post!",
			User:        mock.JonSnow,
		},
	}

	task := tasks.NotifyAboutDeletedPost(post, true)

	err := worker.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		WithBaseURL("http://domain.com").
		Execute(task)

	Expect(err).IsNil()
	Expect(emailmock.MessageHistory).HasLen(1)
	Expect(emailmock.MessageHistory[0].TemplateName).Equals("delete_post")
	Expect(emailmock.MessageHistory[0].Tenant).Equals(mock.DemoTenant)
	Expect(emailmock.MessageHistory[0].Props).Equals(dto.Props{
		"title":    "Add support for TypeScript",
		"siteName": "Demonstration",
		"content":  template.HTML("<p>Invalid post!</p>"),
		"change":   "<a href='http://domain.com/settings'>change your notification preferences</a>",
		"logo":     "https://fider.io/images/logo-100x100.png",
	})
	Expect(emailmock.MessageHistory[0].From).Equals(dto.Recipient{
		Name: "Jon Snow",
	})
	Expect(emailmock.MessageHistory[0].To).HasLen(1)
	Expect(emailmock.MessageHistory[0].To[0]).Equals(dto.Recipient{
		Name:    "Arya Stark",
		Address: "arya.stark@got.com",
		Props:   dto.Props{},
	})

	Expect(addNewNotification).IsNotNil()
	Expect(addNewNotification.PostID).Equals(post.ID)
	Expect(addNewNotification.Link).Equals("")
	Expect(addNewNotification.Title).Equals("**Jon Snow** deleted **Add support for TypeScript**")
	Expect(addNewNotification.User).Equals(mock.AryaStark)

	Expect(triggerWebhooks).IsNotNil()
	Expect(triggerWebhooks.Type).Equals(enum.WebhookDeletePost)
	Expect(triggerWebhooks.Props).ContainsProps(webhook.Props{
		"post_id":                    post.ID,
		"post_number":                post.Number,
		"post_title":                 post.Title,
		"post_slug":                  post.Slug,
		"post_description":           post.Description,
		"post_status":                post.Status.Name(),
		"post_url":                   "http://domain.com/posts/1/add-support-for-typescript",
		"post_author_id":             mock.AryaStark.ID,
		"post_author_name":           mock.AryaStark.Name,
		"post_author_email":          mock.AryaStark.Email,
		"post_author_role":           mock.AryaStark.Role.String(),
		"post_response":              true,
		"post_response_text":         post.Response.Text,
		"post_response_responded_at": post.Response.RespondedAt,
		"post_response_author_id":    mock.JonSnow.ID,
		"post_response_author_name":  mock.JonSnow.Name,
		"post_response_author_email": mock.JonSnow.Email,
		"post_response_author_role":  mock.JonSnow.Role.String(),
		"author_id":                  mock.JonSnow.ID,
		"author_name":                mock.JonSnow.Name,
		"author_email":               mock.JonSnow.Email,
		"author_role":                mock.JonSnow.Role.String(),
		"tenant_id":                  mock.DemoTenant.ID,
		"tenant_name":                mock.DemoTenant.Name,
		"tenant_subdomain":           mock.DemoTenant.Subdomain,
		"tenant_status":              mock.DemoTenant.Status.String(),
		"tenant_url":                 "http://domain.com",
	})
}

func TestNotifyAboutDeletePostTask_NoComment(t *testing.T) {
	RegisterT(t)
	bus.Init(emailmock.Service{})

	var addNewNotification *cmd.AddNewNotification
	bus.AddHandler(func(ctx context.Context, c *cmd.AddNewNotification) error {
		addNewNotification = c
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetActiveSubscribers) error {
		q.Result = []*entity.User{
			mock.AryaStark,
		}
		return nil
	})

	var triggerWebhooks *cmd.TriggerWebhooks
	bus.AddHandler(func(ctx context.Context, c *cmd.TriggerWebhooks) error {
		triggerWebhooks = c
		return nil
	})

	worker := mock.NewWorker()
	post := &entity.Post{
		ID:          1,
		Number:      1,
		Title:       "Add support for TypeScript",
		Slug:        "add-support-for-typescript",
		Description: "TypeScript is great, please add support for it",
		User:        mock.AryaStark,
		Status:      enum.PostDeleted,
		Response: &entity.PostResponse{
			RespondedAt: time.Now(),
			Text:        "Invalid post!",
			User:        mock.JonSnow,
		},
	}

	task := tasks.NotifyAboutDeletedPost(post, false)

	err := worker.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		WithBaseURL("http://domain.com").
		Execute(task)

	Expect(err).IsNil()
	Expect(emailmock.MessageHistory).HasLen(0)

	Expect(addNewNotification).IsNil()

	Expect(triggerWebhooks).IsNotNil()
	Expect(triggerWebhooks.Type).Equals(enum.WebhookDeletePost)
	Expect(triggerWebhooks.Props).ContainsProps(webhook.Props{
		"post_id":                    post.ID,
		"post_number":                post.Number,
		"post_title":                 post.Title,
		"post_slug":                  post.Slug,
		"post_description":           post.Description,
		"post_status":                post.Status.Name(),
		"post_url":                   "http://domain.com/posts/1/add-support-for-typescript",
		"post_author_id":             mock.AryaStark.ID,
		"post_author_name":           mock.AryaStark.Name,
		"post_author_email":          mock.AryaStark.Email,
		"post_author_role":           mock.AryaStark.Role.String(),
		"post_response":              true,
		"post_response_text":         post.Response.Text,
		"post_response_responded_at": post.Response.RespondedAt,
		"post_response_author_id":    mock.JonSnow.ID,
		"post_response_author_name":  mock.JonSnow.Name,
		"post_response_author_email": mock.JonSnow.Email,
		"post_response_author_role":  mock.JonSnow.Role.String(),
		"author_id":                  mock.JonSnow.ID,
		"author_name":                mock.JonSnow.Name,
		"author_email":               mock.JonSnow.Email,
		"author_role":                mock.JonSnow.Role.String(),
		"tenant_id":                  mock.DemoTenant.ID,
		"tenant_name":                mock.DemoTenant.Name,
		"tenant_subdomain":           mock.DemoTenant.Subdomain,
		"tenant_status":              mock.DemoTenant.Status.String(),
		"tenant_url":                 "http://domain.com",
	})
}
