package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

// SendInvites sends one email to each invited recipient
func SendInvites(subject, message string, invitations []*actions.UserInvitation) worker.Task {
	return describe("Send invites", func(c *worker.Context) error {
		to := make([]dto.Recipient, len(invitations))
		for i, invite := range invitations {
			err := bus.Dispatch(c, &cmd.SaveVerificationKey{
				Key:      invite.VerificationKey,
				Duration: 15 * 24 * time.Hour,
				Request:  invite,
			})
			if err != nil {
				return c.Failure(err)
			}

			url := fmt.Sprintf("%s/invite/verify?k=%s", web.BaseURL(c), invite.VerificationKey)
			toMessage := strings.Replace(message, app.InvitePlaceholder, url, -1)
			to[i] = dto.NewRecipient("", invite.Email, dto.Props{
				"message": markdown.Full(toMessage),
			})
		}

		bus.Publish(c, &cmd.SendMail{
			From: dto.Recipient{
				Name: c.User().Name,
			},
			To:           to,
			TemplateName: "invite_email",
			Props: dto.Props{
				"subject": subject,
				"logo":    web.LogoURL(c),
			},
		})

		return nil
	})
}
