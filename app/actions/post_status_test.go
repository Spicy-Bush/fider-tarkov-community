package actions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestSetResponseRejectsUnknownStatus(t *testing.T) {
	status := enum.PostStarted
	action := actions.SetResponse{Status: &status}
	if err := json.Unmarshal([]byte(`{"status":"not-a-status"}`), &action); err == nil {
		t.Fatal("unknown response status was accepted")
	}
	if action.Status == nil || *action.Status != enum.PostStarted {
		t.Fatalf("invalid response status became %v", action.Status)
	}
}

func TestSetResponseRequiresStatus(t *testing.T) {
	for _, body := range []string{`{}`, `{"status":null}`} {
		t.Run(body, func(t *testing.T) {
			var action actions.SetResponse
			if err := json.Unmarshal([]byte(body), &action); err != nil {
				t.Fatal(err)
			}
			user := &entity.User{Role: enum.RoleAdministrator, Status: enum.UserActive}
			result := action.Validate(context.Background(), user)
			if result.Ok || len(result.Errors) != 1 || result.Errors[0].Field != "status" {
				t.Fatalf("missing status validation: %+v", result)
			}
		})
	}
}
