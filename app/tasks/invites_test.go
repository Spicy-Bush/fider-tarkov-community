package tasks_test

import (
	"context"
	"html/template"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email/emailmock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func TestSendInvites(t *testing.T) {
	RegisterT(t)
	bus.Init(emailmock.Service{})

	savedKeys := make([]*cmd.SaveVerificationKey, 0)
	bus.AddHandler(func(ctx context.Context, c *cmd.SaveVerificationKey) error {
		savedKeys = append(savedKeys, c)
		return nil
	})

	worker := mock.NewWorker()
	task := tasks.SendInvites("My Subject", "Click here: %invite%", []*actions.UserInvitation{
		{Email: "user1@domain.com", VerificationKey: "1234"},
		{Email: "user2@domain.com", VerificationKey: "5678"},
	})

	err := worker.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		WithBaseURL("http://domain.com").
		Execute(task)

	Expect(err).IsNil()
	Expect(emailmock.MessageHistory).HasLen(1)
	Expect(emailmock.MessageHistory[0].TemplateName).Equals("invite_email")
	Expect(emailmock.MessageHistory[0].Tenant).Equals(mock.DemoTenant)
	Expect(emailmock.MessageHistory[0].Props).Equals(dto.Props{
		"subject": "My Subject",
		"logo":    "https://fider.io/images/logo-100x100.png",
	})
	Expect(emailmock.MessageHistory[0].From).Equals(dto.Recipient{
		Name: "Jon Snow",
	})
	Expect(emailmock.MessageHistory[0].To).HasLen(2)
	Expect(emailmock.MessageHistory[0].To[0]).Equals(dto.Recipient{
		Address: "user1@domain.com",
		Props: dto.Props{
			"message": template.HTML(`<p>Click here: <a href="http://domain.com/invite/verify?k=1234" rel="nofollow noreferrer">http://domain.com/invite/verify?k=1234</a></p>`),
		},
	})
	Expect(emailmock.MessageHistory[0].To[1]).Equals(dto.Recipient{
		Address: "user2@domain.com",
		Props: dto.Props{
			"message": template.HTML(`<p>Click here: <a href="http://domain.com/invite/verify?k=5678" rel="nofollow noreferrer">http://domain.com/invite/verify?k=5678</a></p>`),
		},
	})

	Expect(savedKeys).HasLen(2)
	Expect(savedKeys[0].Key).Equals("1234")
	Expect(savedKeys[0].Request.GetEmail()).Equals("user1@domain.com")
	Expect(savedKeys[1].Key).Equals("5678")
	Expect(savedKeys[1].Request.GetEmail()).Equals("user2@domain.com")
}
