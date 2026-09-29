package validate_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestImageUploadMetadataBounds(t *testing.T) {
	for _, test := range []struct {
		name        string
		fileName    string
		contentType string
		valid       bool
	}{
		{"unspecified", "", "", true},
		{"filename boundary", strings.Repeat("a", 255), "image/png", true},
		{"filename too long", strings.Repeat("a", 256), "image/png", false},
		{"filename byte limit", strings.Repeat("é", 128), "image/png", false},
		{"ordinary MIME parameters", "image.png", "image/png; charset=binary", true},
		{"generic content type", "image.png", "application/octet-stream", true},
		{"content type too long", "image.png", "image/png; name=" + strings.Repeat("a", 256), false},
		{"malformed content type", "image.png", "image/png\r\nInjected: yes", false},
		{"incomplete content type", "image.png", "png", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			image := &dto.ImageUploadData{FileName: test.fileName, ContentType: test.contentType}
			err := validate.ImageUploadMetadata(image)
			if (err == nil) != test.valid {
				t.Fatalf("metadata valid=%t, want %t: %v", err == nil, test.valid, err)
			}

			messages, err := validate.ImageUpload(context.Background(), &dto.ImageUpload{Upload: image}, validate.ImageUploadOpts{})
			if err != nil || (len(messages) == 0) != test.valid {
				t.Fatalf("image validation disagrees with metadata boundary: messages=%v error=%v", messages, err)
			}
		})
	}
}

func TestImageValidationPreservesRequest(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1501, 1501))); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		content     []byte
		contentType string
		maxKB       int
		failures    int
	}{
		{"valid large image", encoded.Bytes(), "image/png", 7500, 0},
		{"oversized image", encoded.Bytes(), "image/png", 1, 1},
		{"unsupported SVG", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml", 7500, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := append([]byte(nil), test.content...)
			upload := &dto.ImageUpload{Upload: &dto.ImageUploadData{Content: test.content, ContentType: test.contentType}}
			messages, err := validate.ImageUpload(context.Background(), upload, validate.ImageUploadOpts{MaxKilobytes: test.maxKB})
			if err != nil || len(messages) != test.failures {
				t.Fatalf("validation failures=%v error=%v", messages, err)
			}

			if !bytes.Equal(upload.Upload.Content, original) || upload.Upload.ContentType != test.contentType {
				t.Fatal("validation changed its input image")
			}
		})
	}
}

func TestValidateImageUpload(t *testing.T) {
	RegisterT(t)

	var testCases = []struct {
		fileName string
		count    int
	}{
		{"/app/pkg/web/testdata/logo1.png", 0},
		{"/app/pkg/web/testdata/logo2.jpg", 1},
		{"/app/pkg/web/testdata/logo3.gif", 1},
		{"/app/pkg/web/testdata/logo4.png", 1},
		{"/app/pkg/web/testdata/logo5.png", 0},
		{"/README.md", 1},
		{"/app/pkg/web/testdata/favicon.ico", 1},
	}

	for _, testCase := range testCases {
		img, _ := os.ReadFile(env.Path(testCase.fileName))

		upload := &dto.ImageUpload{
			Upload: &dto.ImageUploadData{
				Content: img,
			},
		}
		messages, err := validate.ImageUpload(context.Background(), upload, validate.ImageUploadOpts{
			MinHeight:    200,
			MinWidth:     200,
			MaxKilobytes: 100,
			ExactRatio:   true,
		})
		Expect(messages).HasLen(testCase.count)
		Expect(err).IsNil()
	}
}

func TestValidateImageUpload_ExactRatio(t *testing.T) {
	RegisterT(t)

	img, _ := os.ReadFile(env.Path("/app/pkg/web/testdata/logo3-200w.gif"))
	opts := validate.ImageUploadOpts{
		IsRequired:   false,
		MaxKilobytes: 200,
	}

	upload := &dto.ImageUpload{
		Upload: &dto.ImageUploadData{
			Content: img,
		},
	}
	opts.ExactRatio = true
	messages, err := validate.ImageUpload(context.Background(), upload, opts)
	Expect(messages).HasLen(1)
	Expect(err).IsNil()

	opts.ExactRatio = false
	messages, err = validate.ImageUpload(context.Background(), upload, opts)
	Expect(messages).HasLen(0)
	Expect(err).IsNil()
}

func TestValidateImageUpload_Nil(t *testing.T) {
	RegisterT(t)

	messages, err := validate.ImageUpload(context.Background(), nil, validate.ImageUploadOpts{
		IsRequired:   false,
		MinHeight:    200,
		MinWidth:     200,
		MaxKilobytes: 50,
		ExactRatio:   true,
	})
	Expect(messages).HasLen(0)
	Expect(err).IsNil()

	messages, err = validate.ImageUpload(context.Background(), &dto.ImageUpload{}, validate.ImageUploadOpts{
		IsRequired:   false,
		MinHeight:    200,
		MinWidth:     200,
		MaxKilobytes: 50,
		ExactRatio:   true,
	})
	Expect(messages).HasLen(0)
	Expect(err).IsNil()
}

func TestValidateImageUpload_Required(t *testing.T) {
	RegisterT(t)

	var testCases = []struct {
		upload *dto.ImageUpload
		count  int
	}{
		{nil, 1},
		{&dto.ImageUpload{}, 1},
		{&dto.ImageUpload{
			BlobKey: "some-file.png",
			Remove:  true,
		}, 1},
	}

	for _, testCase := range testCases {
		messages, err := validate.ImageUpload(context.Background(), testCase.upload, validate.ImageUploadOpts{
			IsRequired:   true,
			MinHeight:    200,
			MinWidth:     200,
			MaxKilobytes: 50,
			ExactRatio:   true,
		})
		Expect(messages).HasLen(testCase.count)
		Expect(err).IsNil()
	}
}

func TestValidateMultiImageUpload(t *testing.T) {
	RegisterT(t)

	img, _ := os.ReadFile(env.Path("/app/pkg/web/testdata/logo3-200w.gif"))

	uploads := []*dto.ImageUpload{
		{
			Upload: &dto.ImageUploadData{
				Content: img,
			},
		},
		{
			Upload: &dto.ImageUploadData{
				Content: img,
			},
		},
		{
			Upload: &dto.ImageUploadData{
				Content: img,
			},
		},
	}

	messages, err := validate.MultiImageUpload(context.Background(), nil, uploads, validate.MultiImageUploadOpts{
		MaxUploads:   2,
		MaxKilobytes: 500,
	})
	Expect(messages).HasLen(1)
	Expect(err).IsNil()
}

func TestValidateMultiImageUpload_Existing(t *testing.T) {
	RegisterT(t)

	img, _ := os.ReadFile(env.Path("/app/pkg/web/testdata/logo3-200w.gif"))

	uploads := []*dto.ImageUpload{
		{
			BlobKey: "attachments/file1.png",
			Remove:  true,
		},
		{
			BlobKey: "attachments/file2.png",
			Remove:  true,
		},
		{
			Upload: &dto.ImageUploadData{
				Content: img,
			},
		},
		{
			Upload: &dto.ImageUploadData{
				Content: img,
			},
		},
	}

	currentAttachments := []string{"attachments/file1.png", "attachments/file2.png"}
	messages, err := validate.MultiImageUpload(context.Background(), currentAttachments, uploads, validate.MultiImageUploadOpts{
		MaxUploads:   2,
		MaxKilobytes: 500,
	})
	Expect(messages).HasLen(0)
	Expect(err).IsNil()
}

func TestValidateMultiImageUploadDuplicateRemovalCapacity(t *testing.T) {
	image, err := os.ReadFile(env.Path("/app/pkg/web/testdata/logo1.png"))
	if err != nil {
		t.Fatal(err)
	}

	existing := []string{"attachments/existing.webp"}
	uploads := []*dto.ImageUpload{
		{BlobKey: existing[0], Remove: true},
		{BlobKey: existing[0], Remove: true},
		{Upload: &dto.ImageUploadData{Content: image}},
		{Upload: &dto.ImageUploadData{Content: image}},
		{Upload: &dto.ImageUploadData{Content: image}},
	}
	opts := validate.MultiImageUploadOpts{MaxUploads: 2, MaxKilobytes: 7500}

	messages, err := validate.MultiImageUpload(context.Background(), existing, uploads, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 {
		t.Error("removing one existing image twice allowed three replacement images")
	}

	validReplacement := []*dto.ImageUpload{uploads[0], uploads[2], uploads[3]}
	messages, err = validate.MultiImageUpload(context.Background(), existing, validReplacement, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("valid replacement did not recover: %v", messages)
	}
}

func TestValidateMultiImageUploadRemovalDominatesRetainedKeys(t *testing.T) {
	key := "attachments/removed.png"
	for _, changes := range [][]*dto.ImageUpload{
		{{BlobKey: key, Remove: true}, {BlobKey: key}},
		{{BlobKey: key}, {BlobKey: key, Remove: true}},
	} {
		messages, err := validate.MultiImageUpload(context.Background(), []string{key}, changes, validate.MultiImageUploadOpts{MaxUploads: 0})
		if err != nil || len(messages) != 0 {
			t.Fatalf("retained key reversed removal during validation: %v %v", messages, err)
		}
	}
}
