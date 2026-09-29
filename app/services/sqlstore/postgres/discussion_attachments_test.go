package postgres_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func discussionImage(t *testing.T) *dto.ImageUpload {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}

	return &dto.ImageUpload{
		Upload: &dto.ImageUploadData{
			FileName:    "discussion.png",
			ContentType: "image/png",
			Content:     encoded.Bytes(),
		},
	}
}

func TestDiscussionAttachmentEditReceiptAndVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	page := &cmd.CreatePage{
		Title:              "Image discussion",
		Slug:               "image-discussion",
		Content:            "Page content",
		Status:             entity.PageStatusPublished,
		Visibility:         entity.PageVisibilityPublic,
		AllowComments:      true,
		AllowCommentImages: true,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	create := &cmd.CreateComment{
		PageID:       page.Result.ID,
		Content:      "Before image",
		BaseURL:      "http://localhost:3000",
		SubmissionID: "before-image",
	}
	if err := bus.Dispatch(f.ctx, create); err != nil {
		t.Fatal(err)
	}

	edit := func() *cmd.UpdateComment {
		return &cmd.UpdateComment{
			CommentID:    create.Result.ID,
			Content:      "After image",
			Attachments:  []*dto.ImageUpload{discussionImage(t)},
			SubmissionID: "image-edit-receipt",
			BaseURL:      "http://localhost:3000",
		}
	}

	first := edit()
	if err := bus.Dispatch(f.ctx, first); err != nil {
		t.Fatal(err)
	}

	if len(first.Result.Attachments) != 1 {
		t.Fatalf("saved attachments: %v", first.Result.Attachments)
	}

	key := first.Result.Attachments[0]
	retry := edit()
	if err := bus.Dispatch(f.ctx, retry); err != nil {
		t.Fatal(err)
	}

	if len(retry.Result.Attachments) != 1 || retry.Result.Attachments[0] != key {
		t.Fatalf("edit replay duplicated image: %v", retry.Result.Attachments)
	}

	checkImage := func(user *entity.User, want int) {
		t.Helper()
		for _, variant := range []struct {
			path    string
			handler web.HandlerFunc
		}{
			{"/static/images/" + key, handlers.ViewUploadedImage()},
			{"/static/images/" + key + "?size=200", handlers.ViewUploadedImage()},
			{"/static/favicon/" + key + "?size=200", handlers.Favicon()},
		} {
			request := httptest.NewRequest(http.MethodGet, variant.path, nil)
			request.Header.Set("Accept", "application/json")
			response := httptest.NewRecorder()
			ctx, err := web.NewContext(f.engine, request, response, web.StringMap{"bkey": key})
			if err != nil {
				t.Fatal(err)
			}

			ctx.SetTenant(f.tenant)
			ctx.SetUser(user)
			if err := variant.handler(ctx); err != nil && response.Code == http.StatusOK {
				t.Fatal(err)
			}

			if response.Code != want {
				t.Fatalf("%s status=%d, want=%d", variant.path, response.Code, want)
			}

			if response.Header().Get("Cache-Control") != "private, no-cache" && want == http.StatusOK {
				t.Fatalf("attachment cache policy: %v", response.Header())
			}

			if want == http.StatusOK {
				if _, _, err := image.Decode(bytes.NewReader(response.Body.Bytes())); err != nil {
					t.Fatalf("undecodable %s: %v", variant.path, err)
				}
			}
		}
	}

	checkImage(nil, http.StatusOK)
	checkImage(f.user, http.StatusOK)

	if err := bus.Dispatch(f.ctx, &cmd.UpdatePage{
		PageID:             page.Result.ID,
		Title:              page.Result.Title,
		Slug:               page.Result.Slug,
		Content:            page.Result.Content,
		Status:             entity.PageStatusPublished,
		Visibility:         entity.PageVisibilityPrivate,
		AllowedRoles:       []string{"administrator"},
		AllowComments:      true,
		AllowCommentImages: true,
	}); err != nil {
		t.Fatal(err)
	}

	moderator := *f.user
	moderator.ID = 2
	moderator.Role = enum.RoleModerator
	checkImage(nil, http.StatusNotFound)
	checkImage(&moderator, http.StatusNotFound)
	checkImage(f.user, http.StatusOK)

	foreign := context.WithValue(f.ctx, app.TenantCtxKey, &entity.Tenant{ID: 2})
	access := &query.CanReadAttachment{Key: key}
	if err := bus.Dispatch(foreign, access); err != nil || access.Result {
		t.Fatalf("foreign attachment access=%v error=%v", access.Result, err)
	}

	if err := bus.Dispatch(f.ctx, &cmd.DeleteComment{CommentID: create.Result.ID}); err != nil {
		t.Fatal(err)
	}
	checkImage(nil, http.StatusNotFound)
	checkImage(&moderator, http.StatusNotFound)
	checkImage(f.user, http.StatusOK)
}
