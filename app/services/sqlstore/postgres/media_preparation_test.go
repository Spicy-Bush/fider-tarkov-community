package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres"
)

func TestInlineImagesReleaseConnectionBeforeExternalStorage(t *testing.T) {
	f := newPostWorkflow(t)
	f.allowImageUploads(t)

	const description = "This submission checks image preparation across the complete storage workflow. " +
		"The image must be prepared before an owner transaction begins, and its receipt must return " +
		"the same completed operation without another external write when the response is retried."
	useExternalImageStorage(t)

	stores := 0
	bus.AddHandler(func(ctx context.Context, c *cmd.StoreBlob) error {
		stores++
		if dbx.Connection().Stats().InUse != 0 {
			t.Error("external image write retained a database connection")
		}
		if len(c.Content) == 0 || c.ContentType != "image/webp" {
			t.Errorf("image preparation did not produce WebP bytes: %s", c.ContentType)
		}
		return nil
	})

	post := &cmd.AddNewPost{Title: "Image edit owner", Description: "An image owner"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	comment := &cmd.CreateComment{
		PostNumber: post.Result.Number, Content: "Image edit comment", SubmissionID: "prepare-owner",
	}
	if err := bus.Dispatch(f.ctx, comment); err != nil {
		t.Fatal(err)
	}
	page := &cmd.CreatePage{
		Title: "Image edit page", Content: "An image owner", Status: entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.OpenPageEdit{PageID: page.Result.ID}); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name    string
		handler web.HandlerFunc
		params  web.StringMap
		body    map[string]any
		replay  bool
	}{
		{
			name: "post create", handler: api.CreatePost(), replay: true,
			body: map[string]any{
				"title": "Image preparation submission", "description": description,
				"submissionId": "prepare-post", "attachments": []any{pngAttachment(t, 4)},
			},
		},
		{
			name: "post edit", handler: api.UpdatePost(),
			params: web.StringMap{"number": fmt.Sprint(post.Result.Number)},
			body: map[string]any{
				"title": "Image edit owner", "description": description,
				"attachments": []any{pngAttachment(t, 4)},
			},
		},
		{
			name: "comment create", handler: api.CreateDiscussionComment(), replay: true,
			params: web.StringMap{"number": fmt.Sprint(post.Result.Number)},
			body: map[string]any{
				"content": "Image preparation comment", "submissionId": "prepare-comment",
				"attachments": []any{pngAttachment(t, 4)},
			},
		},
		{
			name: "comment edit", handler: api.EditDiscussionComment(), replay: true,
			params: web.StringMap{"id": fmt.Sprint(comment.Result.ID)},
			body: map[string]any{
				"content": "Image preparation edited comment", "submissionId": "prepare-comment-edit",
				"attachments": []any{pngAttachment(t, 4)},
			},
		},
		{
			name: "shared page banner", handler: handlers.UploadPageEditBanner(), replay: true,
			params: web.StringMap{"id": fmt.Sprint(page.Result.ID)},
			body: map[string]any{
				"submissionId": "shared-page-banner", "image": pngAttachment(t, 4),
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			body, err := json.Marshal(scenario.body)
			if err != nil {
				t.Fatal(err)
			}

			before := stores
			if scenario.name == "post create" {
				if _, err := mediaFixtureSQL(`INSERT INTO user_mutes (tenant_id, user_id, reason, created_by)
					VALUES ($1, $2, 'Image preparation fixture', $2)`, f.tenant.ID, f.user.ID); err != nil {
					t.Fatal(err)
				}

				f.user.Muted = true
				denied, err := f.requestWithParams(scenario.handler, http.MethodPost, "/api/experiment", string(body), scenario.params)
				f.user.Muted = false
				if err != nil || denied.Code != http.StatusBadRequest || stores != before {
					t.Fatalf("denied post prepared an image: status=%d error=%v writes=%d", denied.Code, err, stores-before)
				}

				if _, err := mediaFixtureSQL("DELETE FROM user_mutes WHERE tenant_id=$1 AND user_id=$2", f.tenant.ID, f.user.ID); err != nil {
					t.Fatal(err)
				}
			}

			response, err := f.requestWithParams(scenario.handler, http.MethodPost, "/api/experiment", string(body), scenario.params)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("status=%d error=%v body=%s", response.Code, err, response.Body)
			}
			if stores != before+1 {
				t.Fatalf("expected one external image write, got %d", stores-before)
			}

			if scenario.replay {
				if _, err := mediaFixtureSQL(`INSERT INTO user_mutes (tenant_id, user_id, reason, created_by)
					VALUES ($1, $2, 'Accepted image retry fixture', $2)`, f.tenant.ID, f.user.ID); err != nil {
					t.Fatal(err)
				}

				f.user.Muted = true
				response, err = f.requestWithParams(scenario.handler, http.MethodPost, "/api/experiment", string(body), scenario.params)
				f.user.Muted = false
				if _, cleanupErr := mediaFixtureSQL("DELETE FROM user_mutes WHERE tenant_id=$1 AND user_id=$2", f.tenant.ID, f.user.ID); cleanupErr != nil {
					t.Fatal(cleanupErr)
				}
				if err != nil || response.Code != http.StatusOK || stores != before+1 {
					t.Fatalf("receipt replay repeated image work: status=%d error=%v writes=%d body=%s", response.Code, err, stores-before, response.Body)
				}
			}
		})
	}
}

func TestInlineImageCommitRechecksOwnerAfterPreparation(t *testing.T) {
	for _, kind := range []string{"comment", "post edit"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			f.allowImageUploads(t)

			post := &cmd.AddNewPost{Title: "Image preparation policy change", Description: "An image owner"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}
			f.user.Role = enum.RoleVisitor
			if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=$2", enum.RoleVisitor, f.user.ID); err != nil {
				t.Fatal(err)
			}

			useExternalImageStorage(t)

			bus.AddHandler(func(ctx context.Context, c *cmd.StoreBlob) error {
				if dbx.Connection().Stats().InUse != 0 {
					t.Fatal("image preparation kept its owner lock")
				}
				_, err := mediaFixtureSQL(`UPDATE posts SET locked_settings='{"locked":true}' WHERE id=$1`, post.Result.ID)
				return err
			})
			body := map[string]any{
				"content":      "This comment was allowed before preparation",
				"submissionId": "preparation-policy-change",
				"attachments":  []any{pngAttachment(t, 4)},
			}
			handler := api.CreateDiscussionComment()
			if kind == "post edit" {
				handler = api.UpdatePost()
				body["title"] = "Changed post title"
				body["description"] = "Changed description"
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}

			response, err := f.requestWithParams(handler, http.MethodPost, "/api/experiment", string(encoded),
				web.StringMap{"number": fmt.Sprint(post.Result.Number)})
			if err != nil || response.Code != http.StatusForbidden {
				t.Fatalf("policy change during preparation was ignored: status=%d error=%v body=%s", response.Code, err, response.Body)
			}
			for _, table := range []string{"comments", "attachments", "media_assets", "notification_deliveries"} {
				if count := workflowCount(t, "SELECT COUNT(*) FROM "+table); count != 0 {
					t.Errorf("rejected commit retained %d rows in %s", count, table)
				}
			}
		})
	}
}

func TestPreparedPostEditRollsBackWithModerationFailure(t *testing.T) {
	f := newPostWorkflow(t)
	f.allowImageUploads(t)

	post := &cmd.AddNewPost{Title: "Original post title", Description: "Original description"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"title": "Changed post title", "description": "Updated description",
		"attachments": []any{pngAttachment(t, 4)},
	})
	if err != nil {
		t.Fatal(err)
	}
	bus.AddHandler(func(context.Context, *cmd.ScheduleModeration) error {
		return fmt.Errorf("injected moderation failure")
	})

	response, err := f.request(api.UpdatePost(), http.MethodPost, post.Result.Number, string(body))
	if err == nil || response.Code == http.StatusOK {
		t.Fatalf("failed edit reported success: status=%d error=%v", response.Code, err)
	}
	var title string
	if err := dbx.Connection().QueryRow("SELECT title FROM posts WHERE id=$1", post.Result.ID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Original post title" {
		t.Fatalf("failed edit changed the post: %s", title)
	}
	for _, table := range []string{"attachments", "blobs", "media_assets"} {
		if count := workflowCount(t, "SELECT COUNT(*) FROM "+table); count != 0 {
			t.Errorf("failed edit retained %d rows in %s", count, table)
		}
	}

	bus.Init(postgres.Service{})
	response, err = f.request(api.UpdatePost(), http.MethodPost, post.Result.Number, string(body))
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("healthy edit did not recover: status=%d error=%v body=%s", response.Code, err, response.Body)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM attachments"); count != 1 {
		t.Fatalf("recovered edit has %d attachments", count)
	}
}

func TestAcceptedPostReplaysAfterItsImageIsDeleted(t *testing.T) {
	f := newPostWorkflow(t)
	f.allowImageUploads(t)

	var body map[string]any
	if err := json.Unmarshal([]byte(submissionBody(t, "accepted-deleted-image", false)), &body); err != nil {
		t.Fatal(err)
	}
	body["attachments"] = []*dto.ImageUpload{pngAttachment(t, 4)}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := f.request(api.CreatePost(), http.MethodPost, 0, string(encoded))
	if err != nil || accepted.Code != http.StatusOK {
		t.Fatalf("initial publication: status=%d error=%v body=%s", accepted.Code, err, accepted.Body)
	}

	var key string
	if err := dbx.Connection().QueryRow("SELECT attachment_bkey FROM attachments").Scan(&key); err != nil {
		t.Fatal(err)
	}
	remove := &cmd.DeleteFiles{BlobKeys: []string{key}, Force: true}
	if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Deleted) != 1 {
		t.Fatalf("force delete accepted image: result=%+v error=%v", remove.Result, err)
	}
	replay, err := f.request(api.CreatePost(), http.MethodPost, 0, string(encoded))
	if err != nil || replay.Code != http.StatusOK || replay.Body.String() != accepted.Body.String() {
		t.Fatalf("lost-response replay: status=%d error=%v body=%s", replay.Code, err, replay.Body)
	}

	for _, check := range []struct {
		query string
		want  int
	}{
		{"SELECT COUNT(*) FROM posts", 1},
		{"SELECT COUNT(*) FROM attachments", 0},
		{"SELECT COUNT(*) FROM media_assets WHERE deleted_at IS NOT NULL", 1},
	} {
		if count := workflowCount(t, check.query); count != check.want {
			t.Fatalf("%s: got=%d want=%d", check.query, count, check.want)
		}
	}
}
