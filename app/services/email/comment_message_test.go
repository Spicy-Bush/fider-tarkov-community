package email_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
)

func TestCommentMessageUsesRecipientText(t *testing.T) {
	for _, text := range []string{
		"<strong>Alice</strong> mentioned you in <strong>A &amp; B</strong>.",
		"<strong>Alice</strong> left a comment.",
		"%recipient.message%",
	} {
		message := email.RenderMessage(context.Background(), "new_comment", email.NoReply, dto.Props{
			"siteName": "Community",
			"title": "Discussion",
			"message": text,
			"content": "Comment body",
			"view": "View",
			"unsubscribe": "Unsubscribe",
			"change": "Settings",
		})
		if !strings.Contains(message.Body, text) || strings.Contains(message.Body, "<no value>") {
			t.Fatalf("recipient text changed during rendering: %s", message.Body)
		}
	}
}
