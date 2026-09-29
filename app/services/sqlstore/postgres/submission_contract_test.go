package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestCommentSubmissionIdentityRequired(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Submission contract", Description: "A post with comments"}
	page := &cmd.CreatePage{
		Title:         "Submission contract",
		Slug:          "submission-contract",
		Content:       "A Page with comments",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(f.ctx, post, page); err != nil {
		t.Fatal(err)
	}

	comment := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		Content:      "Original comment",
		SubmissionID: "original-comment",
	}
	if err := bus.Dispatch(f.ctx, comment); err != nil {
		t.Fatal(err)
	}

	for _, operation := range []struct {
		name    string
		handler web.HandlerFunc
		method  string
		path    string
		params  web.StringMap
	}{
		{
			name:    "post comment",
			handler: api.CreateDiscussionComment(),
			method:  http.MethodPost,
			path:    "/api/posts/1/comments",
			params:  web.StringMap{"number": fmt.Sprint(post.Result.Number)},
		},
		{
			name:    "Page comment",
			handler: api.CreateDiscussionComment(),
			method:  http.MethodPost,
			path:    "/api/pages/1/comments",
			params:  web.StringMap{"id": fmt.Sprint(page.Result.ID)},
		},
		{
			name:    "comment edit",
			handler: api.EditDiscussionComment(),
			method:  http.MethodPut,
			path:    "/api/comments/1",
			params:  web.StringMap{"id": fmt.Sprint(comment.Result.ID)},
		},
	} {
		for _, identity := range []struct {
			name  string
			value any
		}{
			{"omitted", nil},
			{"null", nil},
			{"empty", ""},
			{"too long", strings.Repeat("x", 129)},
		} {
			t.Run(operation.name+"/"+identity.name, func(t *testing.T) {
				input := map[string]any{
					"content": "Changed comment",
				}
				if identity.name != "omitted" {
					input["submissionId"] = identity.value
				}

				body, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}

				response, err := f.requestWithParams(operation.handler, operation.method, operation.path, string(body), operation.params)
				if err != nil || response.Code != http.StatusBadRequest {
					t.Fatalf("missing or invalid identity accepted: %v %d %s", err, response.Code, response.Body)
				}
			})
		}
	}

	if workflowCount(t, "SELECT COUNT(*) FROM posts") != 1 ||
		workflowCount(t, "SELECT COUNT(*) FROM comments") != 1 ||
		workflowCount(t, "SELECT COUNT(*) FROM command_receipts WHERE kind='comment-edit'") != 0 ||
		workflowCount(t, "SELECT COUNT(*) FROM comments WHERE content = 'Original comment'") != 1 {
		t.Fatal("rejected submissions changed stored content or receipts")
	}
}
