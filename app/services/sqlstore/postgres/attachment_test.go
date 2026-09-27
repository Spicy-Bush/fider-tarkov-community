package postgres_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestUploadImage(t *testing.T) {
	f := newPostWorkflow(t)
	upload := pngAttachment(t, 2)
	original := append([]byte(nil), upload.Upload.Content...)
	if err := bus.Dispatch(f.ctx, &cmd.UploadImage{Image: upload, Folder: "avatars"}); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(upload.BlobKey, "avatars/") || !strings.HasSuffix(upload.BlobKey, ".webp") {
		t.Fatalf("unexpected image key: %s", upload.BlobKey)
	}

	if upload.Upload.ContentType != "image/png" || !bytes.Equal(upload.Upload.Content, original) {
		t.Fatal("upload changed its source image")
	}

	stored := &query.GetBlobByKey{Key: upload.BlobKey, AllowUnpublishedAvatar: true}
	if err := bus.Dispatch(f.ctx, stored); err != nil {
		t.Fatal(err)
	}

	decoded, format, err := image.DecodeConfig(bytes.NewReader(stored.Result.Content))
	if err != nil || format != "webp" || stored.Result.ContentType != "image/webp" || decoded.Width != 2 || decoded.Height != 2 {
		t.Fatalf("stored image mismatch: dimensions=%v format=%s error=%v", decoded, format, err)
	}
}

func TestUploadImage_NoContent(t *testing.T) {
	f := newPostWorkflow(t)
	upload := &dto.ImageUpload{Upload: &dto.ImageUploadData{}}
	before := workflowCount(t, "SELECT COUNT(*) FROM blobs")
	if err := bus.Dispatch(f.ctx, &cmd.UploadImage{Image: upload, Folder: "avatars"}); err != nil {
		t.Fatal(err)
	}

	if upload.BlobKey != "" || workflowCount(t, "SELECT COUNT(*) FROM blobs") != before {
		t.Fatal("an empty upload created a blob")
	}
}

func TestUploadImageRejectsInvalidContent(t *testing.T) {
	f := newPostWorkflow(t)
	before := workflowCount(t, "SELECT COUNT(*) FROM blobs")

	for _, content := range [][]byte{
		[]byte("Hello World"),
		[]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
		pngAttachment(t, 2).Upload.Content[:40],
	} {
		upload := &dto.ImageUpload{Upload: &dto.ImageUploadData{Content: content, ContentType: "image/png"}}
		if err := bus.Dispatch(f.ctx, &cmd.UploadImage{Image: upload, Folder: "logos"}); err == nil {
			t.Fatal("invalid image was accepted")
		}

		if upload.BlobKey != "" || workflowCount(t, "SELECT COUNT(*) FROM blobs") != before {
			t.Fatal("rejected upload created a blob")
		}
	}
}

func TestUploadImageRetainsOneLargeDimension(t *testing.T) {
	f := newPostWorkflow(t)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1501, 2))); err != nil {
		t.Fatal(err)
	}

	upload := &dto.ImageUpload{Upload: &dto.ImageUploadData{Content: encoded.Bytes(), ContentType: "image/png"}}
	if err := bus.Dispatch(f.ctx, &cmd.UploadImage{Image: upload, Folder: "avatars"}); err != nil {
		t.Fatal(err)
	}

	stored := &query.GetBlobByKey{Key: upload.BlobKey, AllowUnpublishedAvatar: true}
	if err := bus.Dispatch(f.ctx, stored); err != nil {
		t.Fatal(err)
	}

	decoded, _, err := image.DecodeConfig(bytes.NewReader(stored.Result.Content))
	if err != nil || decoded.Width != 1501 || decoded.Height != 2 {
		t.Fatalf("image with one large dimension changed size: %v error=%v", decoded, err)
	}
}

func TestUploadMultipleImages(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	bus.AddHandler(func(ctx context.Context, c *cmd.StoreBlob) error {
		return nil
	})

	uploadImages := &cmd.UploadImages{
		Images: []*dto.ImageUpload{
			{
				Upload: &dto.ImageUploadData{
					Content:     pngAttachment(t, 2).Upload.Content,
					ContentType: "image/png",
				},
			},
			{
				Upload: &dto.ImageUploadData{
					Content:     pngAttachment(t, 2).Upload.Content,
					ContentType: "image/png",
				},
			},
		},
		Folder: "avatars",
	}
	err := bus.Dispatch(ctx, uploadImages)
	if err != nil {
		t.Fatal(err)
	}

	for _, image := range uploadImages.Images {
		if !strings.HasPrefix(image.BlobKey, "avatars/") || !strings.HasSuffix(image.BlobKey, ".webp") {
			t.Fatalf("unexpected image key: %s", image.BlobKey)
		}
	}

	if uploadImages.Images[0].BlobKey == uploadImages.Images[1].BlobKey {
		t.Fatal("separate uploads reused the same image key")
	}
}
