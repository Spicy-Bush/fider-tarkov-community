package postgres_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func pngAttachment(t *testing.T, size int) *dto.ImageUpload {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatal(err)
	}

	return &dto.ImageUpload{Upload: &dto.ImageUploadData{
		FileName:    "image.png",
		ContentType: "image/png",
		Content:     encoded.Bytes(),
	}}
}

func expectStoredImage(t *testing.T, ctx context.Context, key, format string, size int) *dto.Blob {
	t.Helper()
	stored := &query.GetBlobByKey{Key: key}
	if err := bus.Dispatch(ctx, stored); err != nil {
		t.Fatal(err)
	}

	decoded, actualFormat, err := image.DecodeConfig(bytes.NewReader(stored.Result.Content))
	if err != nil || actualFormat != format || decoded.Width != size || decoded.Height != size {
		t.Fatalf("stored image %s: dimensions=%v format=%s error=%v", key, decoded, actualFormat, err)
	}

	if stored.Result.ContentType != "image/"+format {
		t.Fatalf("stored image MIME type: %s", stored.Result.ContentType)
	}

	return stored.Result
}

func TestFileUploadPreservesImageSizingAndFormat(t *testing.T) {
	f := newPostWorkflow(t)

	for _, size := range []int{2, 1501} {
		upload := pngAttachment(t, size)
		if size == 2 {
			upload.Upload.ContentType = "text/html"
		}

		body, err := json.Marshal(map[string]any{
			"name": "File upload",
			"file": upload,
		})
		if err != nil {
			t.Fatal(err)
		}

		response, err := f.requestWithParams(handlers.UploadFile(), http.MethodPost, "/api/files", string(body), nil)
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("file upload status=%d error=%v body=%s", response.Code, err, response.Body)
		}

		var file dto.FileInfo
		if err := json.Unmarshal(response.Body.Bytes(), &file); err != nil {
			t.Fatal(err)
		}

		wantFormat := "png"
		if size > 1500 {
			wantFormat = "webp"
		}

		stored := expectStoredImage(t, f.ctx, file.BlobKey, wantFormat, min(size, 1500))
		if file.ContentType != stored.ContentType || file.Size != int64(len(stored.Content)) {
			t.Fatalf("file metadata does not describe stored bytes: %+v", file)
		}

		if size == 2 && !bytes.Equal(stored.Content, upload.Upload.Content) {
			t.Fatal("small file was unnecessarily converted")
		}
	}

	body, err := json.Marshal(map[string]any{
		"name": "Unsupported SVG",
		"file": &dto.ImageUpload{Upload: &dto.ImageUploadData{
			FileName:    "image.svg",
			ContentType: "image/svg+xml",
			Content:     []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := f.requestWithParams(handlers.UploadFile(), http.MethodPost, "/api/files", string(body), web.StringMap{})
	if err != nil || response.Code != http.StatusBadRequest {
		t.Fatalf("unsupported file status=%d error=%v body=%s", response.Code, err, response.Body)
	}
}

func TestFileUploadRejectsUnsafeImages(t *testing.T) {
	f := newPostWorkflow(t)
	original := pngAttachment(t, 2).Upload.Content
	bomb := bytes.Clone(original[:33])
	binary.BigEndian.PutUint32(bomb[16:20], 100000)
	binary.BigEndian.PutUint32(bomb[20:24], 100000)
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	before := workflowCount(t, "SELECT COUNT(*) FROM blobs")

	for _, content := range [][]byte{bomb, original[:40]} {
		body, err := json.Marshal(map[string]any{
			"name": "Invalid image",
			"file": &dto.ImageUpload{Upload: &dto.ImageUploadData{
				Content:     content,
				ContentType: "image/png",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}

		response, err := f.requestWithParams(handlers.UploadFile(), http.MethodPost, "/api/files", string(body), nil)
		if err != nil || response.Code != http.StatusBadRequest {
			t.Fatalf("unsafe image: status=%d error=%v body=%s", response.Code, err, response.Body)
		}

		if workflowCount(t, "SELECT COUNT(*) FROM blobs") != before {
			t.Fatal("rejected image persisted a blob")
		}
	}
}

func TestAttachmentUploadPreservesRequest(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Uploaded image data", Description: "Retain request identity"}
	images := []*dto.ImageUpload{discussionImage(t), discussionImage(t)}
	original, err := json.Marshal(images)
	if err != nil {
		t.Fatal(err)
	}

	post.Attachments = images
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	unchanged, err := json.Marshal(images)
	if err != nil || !bytes.Equal(original, unchanged) {
		t.Fatalf("upload changed its request: error=%v", err)
	}

	stored := &query.GetPostAttachments{PostID: post.Result.ID}
	if err := bus.Dispatch(f.ctx, stored); err != nil {
		t.Fatal(err)
	}

	if len(stored.Result) != 2 || stored.Result[0] == stored.Result[1] {
		t.Fatalf("separate uploads lost their identities: %v", stored.Result)
	}

	for _, key := range stored.Result {
		blob := expectStoredImage(t, f.ctx, key, "webp", 2)
		if bytes.Equal(blob.Content, images[0].Upload.Content) {
			t.Fatal("stored image was not converted independently from the request")
		}
	}
}

func TestCommentAttachmentCommandReplay(t *testing.T) {
	for _, size := range []int{2, 1501} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			f := newPostWorkflow(t)
			post := &cmd.AddNewPost{Title: "Comment image replay", Description: "Attachment identities"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}

			create := &cmd.CreateComment{
				PostNumber:   post.Result.Number,
				Content:      "Original image",
				Attachments:  []*dto.ImageUpload{pngAttachment(t, size)},
				SubmissionID: "create-image-replay",
			}
			original := bytes.Clone(create.Attachments[0].Upload.Content)
			if err := bus.Dispatch(f.ctx, create); err != nil {
				t.Fatal(err)
			}

			commentID := create.Result.ID
			f.user.Muted = true
			if err := bus.Dispatch(f.ctx, create); err != nil || create.Created || create.Result.ID != commentID {
				t.Fatalf("same create command did not recover its receipt: %v", err)
			}
			f.user.Muted = false

			edit := &cmd.UpdateComment{
				CommentID:    commentID,
				Content:      "Another image",
				Attachments:  []*dto.ImageUpload{pngAttachment(t, size)},
				SubmissionID: "edit-image-replay",
			}
			if err := bus.Dispatch(f.ctx, edit); err != nil {
				t.Fatal(err)
			}

			f.user.Muted = true
			if err := bus.Dispatch(f.ctx, edit); err != nil {
				t.Fatalf("same edit command did not recover its receipt: %v", err)
			}
			f.user.Muted = false
			if len(edit.Result.Attachments) != 2 {
				t.Fatalf("receipt replay changed attachments: %v", edit.Result.Attachments)
			}

			for _, attachment := range []*dto.ImageUpload{create.Attachments[0], edit.Attachments[0]} {
				if attachment.BlobKey != "" || attachment.Upload.ContentType != "image/png" || !bytes.Equal(attachment.Upload.Content, original) {
					t.Fatal("comment operation changed its input image")
				}
			}

			for _, key := range edit.Result.Attachments {
				expectStoredImage(t, f.ctx, key, "webp", min(size, 1500))
			}
		})
	}
}

func privateCommentImage(t *testing.T) (postWorkflow, string) {
	t.Helper()
	f := newPostWorkflow(t)
	page := &cmd.CreatePage{
		Title:              "Private images",
		Slug:               "private-images",
		Content:            "Restricted Page",
		Status:             entity.PageStatusPublished,
		Visibility:         entity.PageVisibilityPrivate,
		AllowedRoles:       []string{"administrator"},
		AllowComments:      true,
		AllowCommentImages: true,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	comment := &cmd.CreateComment{
		PageID:       page.Result.ID,
		Content:      "Private image",
		Attachments:  []*dto.ImageUpload{discussionImage(t)},
		SubmissionID: "private-image",
	}
	if err := bus.Dispatch(f.ctx, comment); err != nil {
		t.Fatal(err)
	}

	key := comment.Result.Attachments[0]

	visitor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, visitor); err != nil {
		t.Fatal(err)
	}

	f.user = visitor.Result
	f.ctx = context.WithValue(f.ctx, app.UserCtxKey, visitor.Result)

	access := &query.CanReadAttachment{Key: key}
	if err := bus.Dispatch(f.ctx, access); err != nil || access.Result {
		t.Fatalf("private image was readable before submission: access=%v error=%v", access.Result, err)
	}

	return f, key
}

func TestPostWritesCannotAttachPrivateCommentImage(t *testing.T) {
	for _, operation := range []string{"create", "edit"} {
		t.Run(operation, func(t *testing.T) {
			f, key := privateCommentImage(t)
			post := entity.Post{}
			handler, method := api.CreatePost(), http.MethodPost
			if operation == "edit" {
				original := &cmd.AddNewPost{Title: "Original public post", Description: "Original description"}
				if err := bus.Dispatch(f.ctx, original); err != nil {
					t.Fatal(err)
				}

				post = *original.Result
				handler, method = api.UpdatePost(), http.MethodPut
			}

			submit := func(attachments []*dto.ImageUpload, want int) *httptest.ResponseRecorder {
				t.Helper()
				body, err := json.Marshal(map[string]any{
					"title":        "A public post with an image",
					"description":  strings.Repeat("Public description. ", 10),
					"attachments":  attachments,
					"submissionId": "private-image-retry",
				})
				if err != nil {
					t.Fatal(err)
				}

				response, err := f.request(handler, method, post.Number, string(body))
				if err != nil || response.Code != want {
					t.Fatalf("post write: want %d, status=%d error=%v body=%s", want, response.Code, err, response.Body)
				}

				return response
			}

			submit([]*dto.ImageUpload{{BlobKey: key, Upload: &dto.ImageUploadData{}}}, http.StatusBadRequest)
			access := &query.CanReadAttachment{Key: key}
			if err := bus.Dispatch(f.ctx, access); err != nil || access.Result {
				t.Fatalf("private image became readable: access=%v error=%v", access.Result, err)
			}

			if operation == "edit" {
				stored := &query.GetPostByNumber{Number: post.Number}
				if err := bus.Dispatch(f.ctx, stored); err != nil || stored.Result.Title != post.Title {
					t.Fatalf("failed edit changed the post: result=%+v error=%v", stored.Result, err)
				}
			} else if count := workflowCount(t, "SELECT COUNT(*) FROM posts"); count != 0 {
				t.Fatalf("invalid submission created %d posts", count)
			}

			if count := workflowCount(t, "SELECT COUNT(*) FROM attachments WHERE attachment_bkey = $1", key); count != 1 {
				t.Fatalf("private image acquired another owner: %d", count)
			}

			response := submit([]*dto.ImageUpload{discussionImage(t)}, http.StatusOK)
			if operation == "create" {
				if err := json.Unmarshal(response.Body.Bytes(), &post); err != nil {
					t.Fatal(err)
				}
			}

			stored := &query.GetPostAttachments{PostID: post.ID}
			if err := bus.Dispatch(f.ctx, stored, access); err != nil {
				t.Fatal(err)
			}

			if len(stored.Result) != 1 || stored.Result[0] == key || access.Result {
				t.Fatalf("corrected upload lost isolation: attachments=%v private readable=%v", stored.Result, access.Result)
			}
		})
	}
}
