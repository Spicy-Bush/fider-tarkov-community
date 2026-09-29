package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestPostSubmissionIdentityRequired(t *testing.T) {
	f := newPostWorkflow(t)

	for _, test := range []struct {
		name   string
		value  any
		status int
	}{
		{"omitted", nil, http.StatusBadRequest},
		{"null", nil, http.StatusBadRequest},
		{"empty", "", http.StatusBadRequest},
		{"NUL", "a\x00b", http.StatusBadRequest},
		{"boolean", true, http.StatusBadRequest},
		{"number", 1, http.StatusBadRequest},
		{"array", []string{"id"}, http.StatusBadRequest},
		{"object", map[string]string{"id": "id"}, http.StatusBadRequest},
		{"too long", strings.Repeat("x", 129), http.StatusBadRequest},
		{"minimum", "x", http.StatusOK},
		{"maximum", strings.Repeat("x", 128), http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			created := workflowCount(t, "SELECT COUNT(*) FROM posts")
			input := map[string]any{
				"title":       "Submission identity " + test.name,
				"description": strings.Repeat("A useful description for the community. ", 6),
			}
			if test.name != "omitted" {
				input["submissionId"] = test.value
			}

			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}

			response, err := f.request(api.CreatePost(), http.MethodPost, 0, string(body))
			if err != nil || response.Code != test.status {
				t.Fatalf("submission: want %d, got %d, error=%v body=%s", test.status, response.Code, err, response.Body)
			}

			if test.status == http.StatusOK {
				created++
				replay, err := f.request(api.CreatePost(), http.MethodPost, 0, string(body))
				if err != nil || replay.Code != http.StatusOK || replay.Body.String() != response.Body.String() {
					t.Fatalf("replay changed receipt: %v %d %s", err, replay.Code, replay.Body)
				}

				count := workflowCount(t, `SELECT COUNT(*) FROM command_receipts
					WHERE kind='post' AND submission_id = $1 AND fingerprint IS NOT NULL AND result IS NOT NULL`, test.value)
				if count != 1 {
					t.Fatalf("expected one persisted receipt, got %d", count)
				}
			}

			if count := workflowCount(t, "SELECT COUNT(*) FROM posts"); count != created {
				t.Fatalf("expected %d posts, got %d", created, count)
			}
		})
	}
}

func TestSubmitPostRejectsInvalidIdentityBeforeCreation(t *testing.T) {
	f := newPostWorkflow(t)

	for _, id := range []string{"", strings.Repeat("x", 129)} {
		created := false
		err := bus.Dispatch(f.ctx, &cmd.SubmitPost{
			SubmissionID: id,
			Create: func(context.Context) (*entity.Post, error) {
				created = true
				return nil, nil
			},
		})

		var validation *validate.Result
		if !errors.As(err, &validation) || validation.Ok || created {
			t.Errorf("invalid identity reached creation: length=%d created=%v error=%v", len(id), created, err)
		}
	}
}
