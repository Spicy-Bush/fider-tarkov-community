package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

type imageSubmission struct {
	handler web.HandlerFunc
	params  web.StringMap
	payload map[string]any
}

func (f postWorkflow) allowImageUploads(t testing.TB) {
	t.Helper()
	f.tenant.GeneralSettings.MaxImagesPerPost = 5
	f.tenant.GeneralSettings.MaxImagesPerComment = 5
	if err := bus.Dispatch(f.ctx, &cmd.UpdateContentSettings{Settings: f.tenant.GeneralSettings}); err != nil {
		t.Fatal(err)
	}
}

func useExternalImageStorage(t testing.TB) {
	t.Helper()
	previous := env.Config.BlobStorage
	t.Cleanup(func() { env.Config.BlobStorage = previous })
	env.Config.BlobStorage.Type = "fs"
	env.Config.BlobStorage.FS.Path = t.TempDir()
}

func (f postWorkflow) imageSubmission(t testing.TB, kind, id string) imageSubmission {
	t.Helper()
	f.allowImageUploads(t)
	post := &cmd.AddNewPost{Title: "Preparation owner", Description: "Original post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	comment := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		Content:      "Original comment",
		SubmissionID: "owner",
	}
	if err := bus.Dispatch(f.ctx, comment); err != nil {
		t.Fatal(err)
	}

	image := pngAttachment(t, 4)
	request := imageSubmission{
		handler: api.CreateDiscussionComment(),
		params:  web.StringMap{"number": fmt.Sprint(post.Result.Number)},
		payload: map[string]any{
			"submissionId": id,
			"content":      "Submitted comment",
			"attachments":  []any{image},
		},
	}
	switch kind {
	case "post":
		request.handler = api.CreatePost()
		if err := json.Unmarshal([]byte(submissionBody(t, id, false)), &request.payload); err != nil {
			t.Fatal(err)
		}
		request.payload["attachments"] = []any{image}
	case "comment edit":
		request.handler = api.EditDiscussionComment()
		request.params = web.StringMap{"id": fmt.Sprint(comment.Result.ID)}
	case "file":
		request.handler = handlers.UploadFile()
		request.payload = map[string]any{
			"submissionId": actions.NewFileUploadID(f.tenant.ID, f.user.ID),
			"name":         "Submitted image",
			"uploadType":   "file",
			"file":         image,
		}
	}
	return request
}

func TestSubmissionPreparationReusesExternalObject(t *testing.T) {
	for _, kind := range []string{"post", "comment", "comment edit", "file"} {
		for _, fault := range []string{"lost provider response", "concurrent preparation", "conflicting preparation"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				f := newPostWorkflow(t)
				submission := f.imageSubmission(t, kind, "recover-external-object")
				body := submission.payload
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}

				useExternalImageStorage(t)
				objects := make(map[string][]byte)
				var mutex sync.Mutex
				var arrivals sync.WaitGroup
				participants := 4
				if fault == "conflicting preparation" {
					participants = 2
				}
				arrivals.Add(participants)
				writes := 0
				bus.AddHandler(func(ctx context.Context, upload *cmd.StoreBlob) error {
					mutex.Lock()
					objects[upload.Key] = append([]byte(nil), upload.Content...)
					writes++
					first := writes == 1
					mutex.Unlock()

					if fault != "lost provider response" {
						arrivals.Done()
						arrivals.Wait()
					}
					if fault == "lost provider response" && first {
						return fmt.Errorf("provider stored object but acknowledgement was lost")
					}
					return nil
				})

				request := func() error {
					response, err := f.requestWithParams(submission.handler, http.MethodPost, "/api/submission", string(encoded), submission.params)
					if err != nil {
						return err
					}
					if response.Code != http.StatusOK {
						return fmt.Errorf("status=%d: %s", response.Code, response.Body)
					}
					return nil
				}

				wantConflict := http.StatusConflict
				expectedObjects := 1
				if fault == "conflicting preparation" {
					body["attachments"] = []any{pngAttachment(t, 7)}
					if kind == "file" {
						body["file"] = pngAttachment(t, 7)
					}
					changed, err := json.Marshal(body)
					if err != nil {
						t.Fatal(err)
					}

					type outcome struct {
						payload []byte
						status  int
						err     error
					}
					results := make(chan outcome, 2)
					for _, payload := range [][]byte{encoded, changed} {
						go func() {
							response, err := f.requestWithParams(submission.handler, http.MethodPost, "/api/submission", string(payload), submission.params)
							results <- outcome{payload: payload, status: response.Code, err: err}
						}()
					}
					accepted, rejected := 0, 0
					for range 2 {
						result := <-results
						if result.err != nil {
							t.Fatal(result.err)
						}
						if result.status == http.StatusOK {
							accepted++
							encoded = result.payload
						} else if result.status == wantConflict {
							rejected++
						} else {
							t.Fatalf("conflicting preparation returned HTTP %d", result.status)
						}
					}
					if accepted != 1 || rejected != 1 {
						t.Fatalf("conflicting preparation accepted=%d rejected=%d", accepted, rejected)
					}
					expectedObjects = 2
				} else if fault == "lost provider response" {
					if err := request(); err == nil {
						t.Fatal("lost acknowledgement reported success")
					}
					if err := request(); err != nil {
						t.Fatal(err)
					}
				} else {
					results := make(chan error, 4)
					for range 4 {
						go func() { results <- request() }()
					}
					for range 4 {
						if err := <-results; err != nil {
							t.Fatal(err)
						}
					}
				}

				if len(objects) != expectedObjects {
					t.Errorf("preparation produced %d external objects; want %d after %d writes", len(objects), expectedObjects, writes)
				}
				before := writes
				if err := request(); err != nil || writes != before {
					t.Fatalf("committed replay wrote externally: error=%v writes=%d", err, writes-before)
				}

				body["attachments"] = []any{pngAttachment(t, 9)}
				if kind == "file" {
					body["file"] = pngAttachment(t, 9)
				}
				changed, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				response, err := f.requestWithParams(submission.handler, http.MethodPost, "/api/submission", string(changed), submission.params)
				if err != nil || response.Code != wantConflict || writes != before {
					t.Fatalf("changed payload reused an accepted image: status=%d error=%v writes=%d", response.Code, err, writes-before)
				}
			})
		}
	}
}

func TestSubmissionPreparationSeparatesAccounts(t *testing.T) {
	f := newPostWorkflow(t)
	otherTenant := &query.GetTenantByDomain{Domain: "avengers"}
	secondUser := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, otherTenant, secondUser); err != nil {
		t.Fatal(err)
	}
	otherUser := &query.GetUserByEmail{Email: "tony.stark@avengers.com"}
	if err := bus.Dispatch(withTenant(f.ctx, otherTenant.Result), otherUser); err != nil {
		t.Fatal(err)
	}

	useExternalImageStorage(t)

	providerFailure := fmt.Errorf("provider interrupted before publication")
	keys := make(map[string]bool)
	bus.AddHandler(func(ctx context.Context, upload *cmd.StoreBlob) error {
		if keys[upload.Key] {
			t.Errorf("distinct account or operation reused external key %q", upload.Key)
		}
		keys[upload.Key] = true
		return providerFailure
	})

	for _, user := range []*entity.User{f.user, secondUser.Result, otherUser.Result} {
		user.Role = enum.RoleAdministrator
		if _, err := mediaFixtureSQL("UPDATE users SET role = $1 WHERE id = $2", user.Role, user.ID); err != nil {
			t.Fatal(err)
		}
		ctx := withUser(f.ctx, user)
		tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
		tenant.GeneralSettings.MaxImagesPerComment = 5
		post := &cmd.AddNewPost{
			Title:       fmt.Sprintf("Image owner %d", user.ID),
			Description: "Account-scoped preparation",
		}
		if err := bus.Dispatch(ctx, post); err != nil {
			t.Fatal(err)
		}
		comment := &cmd.CreateComment{
			PostNumber: post.Result.Number, Content: "Own editable comment", SubmissionID: "owner",
		}
		if err := bus.Dispatch(ctx, comment); err != nil {
			t.Fatal(err)
		}

		commands := []any{
			&cmd.SubmitPost{
				SubmissionID: "same-id", Fingerprint: "same-content",
				Attachments: []*dto.ImageUpload{pngAttachment(t, 4)},
				Validate:    func(context.Context) error { return nil },
			},
			&cmd.CreateComment{
				PostNumber: post.Result.Number, Content: "New image", SubmissionID: "same-id",
				Attachments: []*dto.ImageUpload{pngAttachment(t, 4)},
			},
			&cmd.UpdateComment{
				CommentID: comment.Result.ID, Content: "New image", SubmissionID: "same-id",
				Attachments: []*dto.ImageUpload{pngAttachment(t, 4)},
			},
			&cmd.UploadImageFile{
				SubmissionID: "same-id", Name: "New image", Type: enum.FileUploadPrivate,
				Content: pngAttachment(t, 4).Upload.Content,
			},
		}
		for _, command := range commands {
			if err := bus.Dispatch(ctx, command); errors.Cause(err) != providerFailure {
				t.Fatalf("account %d/%d command %T did not reach preparation: %v", tenant.ID, user.ID, command, err)
			}
		}
	}

	if len(keys) != 12 {
		t.Fatalf("four operations across three accounts produced %d distinct keys", len(keys))
	}
}
