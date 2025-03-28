package email

import (
	"bytes"
	"context"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/tpl"
)

// Message represents what is sent by email
type Message struct {
	Subject string
	Body    string
}

// RenderMessage returns the HTML of an email based on template and params
func RenderMessage(ctx context.Context, templateName string, fromAddress string, params dto.Props) *Message {
	noreply := false
	if fromAddress == NoReply {
		noreply = true
	}

	tmpl := tpl.GetTemplate("/views/email/base_email.html", "/views/email/"+templateName+".html")
	var bf bytes.Buffer
	if err := tpl.Render(ctx, tmpl, &bf, params.Merge(dto.Props{
		"logo":    params["logo"],
		"noreply": noreply,
	})); err != nil {
		panic(err)
	}

	content := strings.ReplaceAll(strings.ReplaceAll(bf.String(), "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(content, "\n")
	subject := strings.TrimLeft(lines[0], "subject: ")
	body := strings.Join(lines[2:], "\n")

	return &Message{
		Subject: subject,
		Body:    body,
	}
}
