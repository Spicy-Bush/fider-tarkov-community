package mailgun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
)

func sendMail(ctx context.Context, c *cmd.SendMail) error {
	if len(c.To) == 0 {
		return nil
	}

	if c.Props == nil {
		c.Props = dto.Props{}
	}

	if c.From.Address == "" {
		c.From.Address = email.NoReply
	}

	isBatch := len(c.To) > 1

	var message *email.Message
	if isBatch {
		// Replace recipient specific Go templates variables with Mailgun template variables
		if c.To[0].Props != nil {
			for k := range c.To[0].Props {
				c.Props[k] = fmt.Sprintf("%%recipient.%s%%", k)
			}
		}
		message = email.RenderMessage(ctx, c.TemplateName, c.From.Address, c.Props)
	} else {
		message = email.RenderMessage(ctx, c.TemplateName, c.From.Address, c.Props.Merge(c.To[0].Props))
	}

	form := url.Values{}
	form.Add("from", c.From.String())
	form.Add("h:Reply-To", c.From.Address)
	form.Add("subject", message.Subject)
	form.Add("html", message.Body)
	form.Add("o:tag", fmt.Sprintf("template:%s", c.TemplateName))

	tenant, ok := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if ok && !env.IsSingleHostMode() {
		form.Add("o:tag", fmt.Sprintf("tenant:%s", tenant.Subdomain))
	}

	// Set Mailgun's var based on each recipient's variables
	recipientVariables := make(map[string]dto.Props)
	for _, r := range c.To {
		if r.Address != "" {
			if email.CanSendTo(r.Address) {
				form.Add("to", r.String())
				recipientVariables[r.Address] = r.Props
			} else {
				log.Warnf(ctx, "Skipping email to '@{Name} <@{Address}>'.", dto.Props{
					"Name":    r.Name,
					"Address": r.Address,
				})
			}
		}
	}

	if len(recipientVariables) == 0 {
		return nil
	}

	if isBatch {
		json, err := json.Marshal(recipientVariables)
		if err != nil {
			return errors.Wrap(err, "failed to marshal recipient variables")
		}

		form.Add("recipient-variables", string(json))
	}

	if isBatch {
		log.Debugf(ctx, "Sending email to @{CountRecipients} recipients with template @{TemplateName}.", dto.Props{
			"CountRecipients": len(recipientVariables),
			"TemplateName":    c.TemplateName,
		})
	} else {
		log.Debugf(ctx, "Sending email to @{Address} with template @{TemplateName}.", dto.Props{
			"Address":      c.To[0].Address,
			"TemplateName": c.TemplateName,
		})
	}

	req := &cmd.HTTPRequest{
		Method: "POST",
		URL:    getEndpoint(ctx, env.Config.Email.Mailgun.Domain, "/messages"),
		Body:   strings.NewReader(form.Encode()),
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
		BasicAuth: &dto.BasicAuth{
			User:     "api",
			Password: env.Config.Email.Mailgun.APIKey,
		},
	}
	err := bus.Dispatch(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to send email with template %s", c.TemplateName)
	}
	if req.ResponseStatusCode < 200 || req.ResponseStatusCode >= 300 {
		err := errors.New("mailgun rejected email with template %s: HTTP %d", c.TemplateName, req.ResponseStatusCode)
		if req.ResponseStatusCode == 400 {
			var response struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(req.ResponseBody, &response) == nil &&
				strings.HasPrefix(response.Message, "'to' parameter is not a valid address") {
				return &email.RecipientRejected{Cause: err}
			}
		}
		return err
	}
	log.Debugf(ctx, "Email sent with response code @{StatusCode}.", dto.Props{
		"StatusCode": req.ResponseStatusCode,
	})
	return nil
}
