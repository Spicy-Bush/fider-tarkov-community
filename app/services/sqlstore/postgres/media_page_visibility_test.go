package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/reearth/ygo/crdt"
)

func TestPageTextDoesNotPublishPrivateImages(t *testing.T) {
	owner := newPostWorkflow(t)
	image := pngAttachment(t, 2)
	if err := bus.Dispatch(owner.ctx, &cmd.UploadImage{
		Image: image, Folder: "attachments",
	}); err != nil {
		t.Fatal(err)
	}

	editor := owner
	editor.user = &entity.User{
		ID: 2, Role: enum.RoleCollaborator, Status: enum.UserActive, Tenant: owner.tenant,
	}
	editor.ctx = context.WithValue(owner.ctx, app.UserCtxKey, editor.user)
	if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=2", enum.RoleCollaborator); err != nil {
		t.Fatal(err)
	}
	if entity.Can(editor.user, editor.tenant, entity.ManageFiles) {
		t.Fatal("Page editor unexpectedly manages files")
	}

	anonymous := owner
	anonymous.user = nil
	readImage := func(viewer postWorkflow, key string, want int) {
		t.Helper()
		response, err := viewer.requestWithParams(handlers.ViewUploadedImage(), http.MethodGet,
			"/static/images/"+key, "", web.StringMap{"bkey": key})
		if err != nil || response.Code != want {
			t.Fatalf("image status=%d want=%d error=%v", response.Code, want, err)
		}
	}
	readImage(owner, image.BlobKey, http.StatusOK)
	readImage(editor, image.BlobKey, http.StatusNotFound)
	readImage(anonymous, image.BlobKey, http.StatusNotFound)

	page := &cmd.CreatePage{
		Title: "Existing image reference Page", Content: "Original content",
		Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(owner.ctx, page); err != nil {
		t.Fatal(err)
	}

	for index, content := range []string{
		"![Private image](/static/images/" + image.BlobKey + ")",
		`<img src="/static%2Fimages/` + image.BlobKey + `?size=200">`,
		`<img src="&#47;static&#47;images&#47;` + image.BlobKey + `">`,
		"Quoted URL: https://other.example/static/images/" + image.BlobKey,
	} {
		for _, pageID := range []int{0, page.Result.ID} {
			open := &cmd.OpenPageEdit{PageID: pageID, SubmissionID: fmt.Sprintf("private-image-%d", index)}
			if err := bus.Dispatch(editor.ctx, open); err != nil {
				t.Fatal(err)
			}
			syncPageEditingHTTP(t, editor, open.Result, func(transaction *crdt.Transaction) {
				for field, value := range map[string]string{
					"title":   fmt.Sprintf("Image reference %d", index),
					"content": content,
					"excerpt": "/static/images/" + image.BlobKey,
				} {
					text := transaction.GetText(field)
					text.Delete(transaction, 0, text.Len())
					text.Insert(transaction, 0, value, nil)
				}
			})
			body, err := json.Marshal(map[string]any{
				"status": "published", "submissionId": fmt.Sprintf("private-image-%d-%d", index, pageID),
			})
			if err != nil {
				t.Fatal(err)
			}
			handler := middlewares.RequirePermission(entity.ManagePages)(handlers.PublishPageEdit())
			response, err := editor.requestWithParams(handler, http.MethodPost, "/api/pages/publish", string(body),
				web.StringMap{"id": fmt.Sprint(open.Result.PageID)})
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("Page %d: status=%d error=%v body=%s", pageID, response.Code, err, response.Body)
			}

			readImage(editor, image.BlobKey, http.StatusNotFound)
			readImage(anonymous, image.BlobKey, http.StatusNotFound)
		}
	}

	content := "![Private image](/static/images/" + image.BlobKey + ")"

	scheduledFor := time.Now().Add(-time.Minute)
	scheduled := &cmd.CreatePage{
		Title: "Scheduled image reference", Content: content,
		Status: entity.PageStatusScheduled, Visibility: entity.PageVisibilityPublic,
		ScheduledFor: &scheduledFor,
	}
	if err := bus.Dispatch(owner.ctx, scheduled); err != nil {
		t.Fatal(err)
	}
	publish := &cmd.PublishScheduledPages{}
	if err := bus.Dispatch(owner.ctx, publish); err != nil || publish.Result != 1 {
		t.Fatalf("scheduled publication: count=%d error=%v", publish.Result, err)
	}
	readImage(anonymous, image.BlobKey, http.StatusNotFound)

	shared := uploadMediaFixture(t, owner.ctx, "shared-page-image", "Shared image", "attachments/")
	readImage(anonymous, shared.BlobKey, http.StatusOK)
	readImage(owner, image.BlobKey, http.StatusOK)
}

func TestPageInlineBannerHTTPVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	page := &cmd.CreatePage{
		Title:        "Private Page banner",
		Content:      "Only the allowed audience can view this Page.",
		Status:       entity.PageStatusPublished,
		Visibility:   entity.PageVisibilityPrivate,
		AllowedRoles: []string{"helper"},
		BannerImage:  pngAttachment(t, 4),
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	for _, actor := range []struct {
		role   enum.Role
		status int
	}{
		{0, http.StatusNotFound},
		{enum.RoleVisitor, http.StatusNotFound},
		{enum.RoleHelper, http.StatusOK},
		{enum.RoleModerator, http.StatusNotFound},
		{enum.RoleCollaborator, http.StatusOK},
		{enum.RoleAdministrator, http.StatusOK},
	} {
		request := f
		request.user = nil
		if actor.role != 0 {
			request.user = &entity.User{ID: 2, Role: actor.role, Status: enum.UserActive, Tenant: f.tenant}
		}

		for _, image := range []struct {
			path    string
			handler web.HandlerFunc
		}{
			{"/static/images/", handlers.ViewUploadedImage()},
			{"/static/images/?size=200", handlers.ViewUploadedImage()},
			{"/static/images/?size=512", handlers.ViewUploadedImage()},
			{"/static/favicon/?size=64", handlers.Favicon()},
		} {
			response, err := request.requestWithParams(image.handler, http.MethodGet, image.path, "", web.StringMap{
				"bkey": page.Result.BannerImageBKey,
			})
			if err != nil || response.Code != actor.status {
				t.Fatalf("role=%d image=%s: status=%d expected=%d error=%v", actor.role, image.path, response.Code, actor.status, err)
			}
			cacheControl := response.Header().Get("Cache-Control")
			if !strings.Contains(cacheControl, "no-cache") || (actor.status == http.StatusOK && cacheControl != "private, no-cache") {
				t.Fatalf("private Page image cache policy: %v", response.Header())
			}
		}
	}

	if _, err := dbx.Connection().Exec(`
		UPDATE media_assets
		SET storage_source=NULL, name=NULL, content_type=NULL, size=NULL,
		    cataloged_at=NULL, width=0, height=0
		WHERE key=$1
	`, page.Result.BannerImageBKey); err != nil {
		t.Fatal(err)
	}

	response, err := f.requestWithParams(handlers.ViewUploadedImage(), http.MethodGet, "/static/images/?size=200", "", web.StringMap{
		"bkey": page.Result.BannerImageBKey,
	})
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("Page administrator thumbnail before inventory: status=%d error=%v", response.Code, err)
	}

	for _, visibility := range []entity.PageVisibility{entity.PageVisibilityPublic, entity.PageVisibilityUnlisted} {
		if _, err := mediaFixtureSQL("UPDATE pages SET visibility=$2 WHERE id=$1", page.Result.ID, visibility); err != nil {
			t.Fatal(err)
		}

		anonymous := f
		anonymous.user = nil
		response, err := anonymous.requestWithParams(handlers.ViewUploadedImage(), http.MethodGet, "/static/images/", "", web.StringMap{
			"bkey": page.Result.BannerImageBKey,
		})
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("visibility=%s anonymous banner: status=%d error=%v", visibility, response.Code, err)
		}
	}
}

func TestPageAttachmentUsesCurrentPageVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	page := &cmd.CreatePage{
		Title:       "Published Page image",
		Content:     "Page content",
		Status:      entity.PageStatusPublished,
		Visibility:  entity.PageVisibilityPublic,
		BannerImage: pngAttachment(t, 4),
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}
	anonymous := context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil))

	for _, test := range []struct {
		status     entity.PageStatus
		visibility entity.PageVisibility
		allowed    bool
	}{
		{entity.PageStatusPublished, entity.PageVisibilityPublic, true},
		{entity.PageStatusPublished, entity.PageVisibilityPrivate, false},
		{entity.PageStatusDraft, entity.PageVisibilityPublic, false},
		{entity.PageStatusPublished, entity.PageVisibilityUnlisted, true},
	} {
		if _, err := mediaFixtureSQL("UPDATE pages SET status=$2,visibility=$3 WHERE id=$1", page.Result.ID, test.status, test.visibility); err != nil {
			t.Fatal(err)
		}
		access := &query.CanReadAttachment{Key: page.Result.BannerImageBKey}
		if err := bus.Dispatch(anonymous, access); err != nil || access.Result != test.allowed {
			t.Fatalf("status=%s visibility=%s allowed=%t want=%t error=%v", test.status, test.visibility, access.Result, test.allowed, err)
		}
	}
}

func TestPageStoredBannerRequiresOwnershipAndPreservesExistingBanner(t *testing.T) {
	f := newPostWorkflow(t)
	image := pngAttachment(t, 4)
	page := &cmd.CreatePage{
		Title: "Page banner ownership", Content: "Page content", Status: entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPrivate, BannerImage: image,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}
	key := page.Result.BannerImageBKey
	other := context.WithValue(f.ctx, app.UserCtxKey, &entity.User{
		ID: 2, Role: enum.RoleAdministrator, Status: enum.UserActive,
	})
	copyPage := &cmd.CreatePage{
		Title: "Another Page", Content: "Page content", Status: entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic, BannerImage: &dto.ImageUpload{BlobKey: key},
	}
	if err := bus.Dispatch(other, copyPage); err == nil {
		t.Fatal("another account claimed a private banner")
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM pages"); count != 1 {
		t.Fatalf("rejected banner left %d Pages", count)
	}

	edit := &cmd.UpdatePage{
		PageID: page.Result.ID, Title: page.Title, Content: "An unrelated text edit",
		Status: page.Status, Visibility: page.Visibility, BannerImage: &dto.ImageUpload{BlobKey: key},
	}
	if err := bus.Dispatch(other, edit); err != nil || edit.Result.BannerImageBKey != key {
		t.Fatalf("existing banner was not retained: error=%v", err)
	}

	edit.BannerImage = pngAttachment(t, 4)
	edit.BannerImage.Remove = true
	if err := bus.Dispatch(other, edit); err != nil || edit.Result.BannerImageBKey != "" {
		t.Fatalf("explicit banner removal was ignored: error=%v", err)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM blobs"); count != 1 {
		t.Fatalf("removal prepared another upload: blobs=%d", count)
	}

	if _, err := dbx.Connection().Exec("UPDATE media_assets SET is_public=true,size=5001*1024 WHERE key=$1", key); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(other, copyPage); err == nil {
		t.Fatal("new Page accepted a stored banner above the banner upload limit")
	}
	if _, err := dbx.Connection().Exec("UPDATE media_assets SET size=100 WHERE key=$1", key); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(other, copyPage); err != nil {
		t.Fatalf("public library banner was rejected: %v", err)
	}
}
