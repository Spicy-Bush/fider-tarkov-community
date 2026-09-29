package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestFileCommandsReceiveValidatedInputs(t *testing.T) {
	f := newPostWorkflow(t)
	f.user.Role = enum.RoleAdministrator
	calls := 0
	bus.AddHandler(func(ctx context.Context, upload *cmd.UploadImageFile) error {
		calls++
		if upload.Name != "Upload name" || upload.Type != enum.FileUploadPublic || (upload.SubmissionID != "file-operation" && upload.SubmissionID != strings.Repeat("x", 128)) {
			t.Errorf("upload input was not normalized: %+v", upload)
		}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, rename *cmd.RenameImageFile) error {
		calls++
		if rename.Name != "Renamed image" {
			t.Errorf("rename input was not normalized: %q", rename.Name)
		}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, deletion *cmd.DeleteFiles) error {
		calls++
		return nil
	})

	request := func(handler web.HandlerFunc, body any, want int) {
		t.Helper()
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		response, err := f.requestWithParams(handler, http.MethodPost, "/api/admin/files", string(encoded), nil)
		if err != nil || response.Code != want {
			t.Fatalf("status=%d want=%d error=%v body=%s", response.Code, want, err, response.Body)
		}
	}

	valid := map[string]any{
		"submissionId": "file-operation", "name": "  Upload name  ",
		"uploadType": "attachment", "file": pngAttachment(t, 4),
	}
	request(handlers.UploadFile(), valid, http.StatusOK)
	valid["submissionId"] = strings.Repeat("x", 128)
	request(handlers.UploadFile(), valid, http.StatusOK)
	request(handlers.RenameFile(), map[string]any{"blobKey": "files/example", "name": "  Renamed image  "}, http.StatusOK)
	if calls != 3 {
		t.Fatalf("valid input reached %d commands", calls)
	}

	for _, invalid := range []struct {
		field string
		value any
	}{
		{"name", "  "}, {"name", strings.Repeat("x", 256)},
		{"submissionId", ""}, {"submissionId", "a\x00b"}, {"submissionId", strings.Repeat("x", 129)},
		{"uploadType", "../../private"}, {"file", nil},
	} {
		body := make(map[string]any, len(valid))
		for key, value := range valid {
			body[key] = value
		}
		body[invalid.field] = invalid.value
		request(handlers.UploadFile(), body, http.StatusBadRequest)
	}
	request(handlers.RenameFile(), map[string]any{"blobKey": "../escape", "name": "A name"}, http.StatusBadRequest)
	request(handlers.RenameFile(), map[string]any{"blobKey": "files/example", "name": " "}, http.StatusBadRequest)
	for _, keys := range [][]string{nil, {"../escape"}, make([]string, 101)} {
		request(handlers.BulkDeleteFiles(), map[string]any{"blobKeys": keys}, http.StatusBadRequest)
	}
	if calls != 3 {
		t.Fatalf("invalid input reached %d storage commands", calls-3)
	}
}
