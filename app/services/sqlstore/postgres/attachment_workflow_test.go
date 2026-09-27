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
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
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

		response, err := f.requestWithParams(handlers.UploadFile(), http.MethodPost, "/_api/files", string(body), nil)
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

	response, err := f.requestWithParams(handlers.UploadFile(), http.MethodPost, "/_api/files", string(body), web.StringMap{})
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

		response, err := f.requestWithParams(handlers.UploadFile(), http.MethodPost, "/_api/files", string(body), nil)
		if err != nil || response.Code != http.StatusBadRequest {
			t.Fatalf("unsafe image: status=%d error=%v body=%s", response.Code, err, response.Body)
		}

		if workflowCount(t, "SELECT COUNT(*) FROM blobs") != before {
			t.Fatal("rejected image persisted a blob")
		}
	}
}
