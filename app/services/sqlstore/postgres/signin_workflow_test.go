package postgres_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func TestSignInWorkflowCompletesProfileWithNameReview(t *testing.T) {
	f := newPostWorkflow(t)
	previous := env.Config.OpenAI
	env.Config.OpenAI.APIKey = "test"
	env.Config.OpenAI.ModerationEnabled = true
	defer func() { env.Config.OpenAI = previous }()

	email := "new.member@example.invalid"
	request := &actions.SignInByEmail{Email: email}
	if err := bus.Dispatch(f.ctx, &cmd.SaveVerificationKey{Key: "signup-review", Duration: time.Hour, Request: request}); err != nil {
		t.Fatal(err)
	}

	anonymous := f
	anonymous.user = nil
	response, err := anonymous.requestWithParams(handlers.CompleteSignInProfile(), http.MethodPost, "http://localhost:3000/api/signin/complete",
		fmt.Sprintf(`{ "kind": %d, "key": "signup-review", "name": "New Member" }`, enum.EmailVerificationKindSignIn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("sign-up completion returned %d: %s", response.Code, response.Body.String())
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM users WHERE email = $1 AND role = $2", email, enum.RoleVisitor); count != 1 {
		t.Fatalf("expected the new member to be registered, found %d", count)
	}
	if count := workflowCount(t, `
		SELECT COUNT(*) FROM moderation_checks m JOIN users u ON u.id = m.content_id
		WHERE u.email = $1 AND m.content_type = 'name' AND m.text_content = 'New Member'`, email); count != 1 {
		t.Fatalf("expected the chosen name to wait for review, found %d checks", count)
	}
}
