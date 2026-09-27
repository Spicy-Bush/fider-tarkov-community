package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/chai2010/webp"
)

func encodedImage(t testing.TB, format string, size image.Point) []byte {
	t.Helper()

	var encoded bytes.Buffer
	source := image.NewRGBA(image.Rectangle{Max: size})
	var err error

	switch format {
	case "png":
		err = png.Encode(&encoded, source)
	case "jpeg":
		err = jpeg.Encode(&encoded, source, nil)
	case "gif":
		err = gif.Encode(&encoded, source, nil)
	case "webp":
		err = webp.Encode(&encoded, source, nil)
	default:
		t.Fatalf("unknown fixture format: %s", format)
	}

	if err != nil {
		t.Fatal(err)
	}

	return encoded.Bytes()
}

func TestImageResizeResponse(t *testing.T) {
	for _, test := range []struct {
		name   string
		size   int
		source image.Point
		want   image.Point
		input  string
		output string
	}{
		{"original size", 0, image.Pt(80, 40), image.Pt(80, 40), "png", "png"},
		{"larger request", 100, image.Pt(80, 40), image.Pt(80, 40), "png", "png"},
		{"exact width", 64, image.Pt(64, 32), image.Pt(64, 32), "png", "png"},
		{"exact height", 64, image.Pt(32, 64), image.Pt(32, 64), "png", "png"},
		{"landscape", 64, image.Pt(80, 40), image.Pt(64, 32), "png", "webp"},
		{"portrait", 64, image.Pt(40, 80), image.Pt(32, 64), "png", "webp"},
		{"square", 64, image.Pt(80, 80), image.Pt(64, 64), "png", "webp"},
		{"JPEG unchanged", 100, image.Pt(80, 40), image.Pt(80, 40), "jpeg", "jpeg"},
		{"JPEG resized", 64, image.Pt(80, 40), image.Pt(64, 32), "jpeg", "webp"},
		{"GIF unchanged", 100, image.Pt(80, 40), image.Pt(80, 40), "gif", "gif"},
		{"GIF resized", 64, image.Pt(80, 40), image.Pt(64, 32), "gif", "webp"},
		{"WebP unchanged", 100, image.Pt(80, 40), image.Pt(80, 40), "webp", "webp"},
		{"WebP resized", 64, image.Pt(80, 40), image.Pt(64, 32), "webp", "webp"},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := encodedImage(t, test.input, test.source)

			bus.Init()
			bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
				q.Result = &dto.Blob{Content: original, ContentType: "image/" + test.input}
				return nil
			})

			status, response := mock.NewServer().
				OnTenant(mock.DemoTenant).
				WithURL(fmt.Sprintf("https://demo.test.fider.io/?size=%d", test.size)).
				AddParam("bkey", "logos/test.png").
				Execute(handlers.ViewUploadedImage())
			content := response.Body.Bytes()
			if status != http.StatusOK {
				t.Fatalf("image response: status=%d", status)
			}

			if test.source == test.want && !bytes.Equal(content, original) {
				t.Fatal("image was converted despite requiring no resize")
			}

			decoded, format, err := image.Decode(bytes.NewReader(content))
			if err != nil {
				t.Fatal(err)
			}

			if decoded.Bounds().Size() != test.want || format != test.output || response.Header().Get("Content-Type") != "image/"+test.output {
				t.Fatalf("image response: size=%v format=%s type=%s", decoded.Bounds().Size(), format, response.Header().Get("Content-Type"))
			}
		})
	}
}

func TestFaviconResponse(t *testing.T) {
	original := encodedImage(t, "png", image.Pt(96, 96))
	previousAssets := assets.FS
	assets.FS = fstest.MapFS{"favicon.png": {Data: original}}
	handler := handlers.Favicon()
	assets.FS = previousAssets

	bus.Init()
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return nil
	})

	for _, key := range []string{"", "logos/test.png"} {
		for _, background := range []string{"", "white"} {
			t.Run(key+"/"+background, func(t *testing.T) {
				status, response := mock.NewServer().
					OnTenant(mock.DemoTenant).
					WithURL("https://demo.test.fider.io/?size=64&bg=" + background).
					AddParam("bkey", key).
					Execute(handler)
				if status != http.StatusOK || response.Header().Get("Content-Type") != "image/webp" {
					t.Fatalf("favicon response: status=%d type=%s", status, response.Header().Get("Content-Type"))
				}

				decoded, format, err := image.Decode(bytes.NewReader(response.Body.Bytes()))
				if err != nil {
					t.Fatal(err)
				}

				if decoded.Bounds().Size() != image.Pt(64, 64) || format != "webp" {
					t.Fatalf("favicon: size=%v format=%s", decoded.Bounds().Size(), format)
				}

				_, _, _, alpha := decoded.At(0, 0).RGBA()
				if (background == "" && alpha != 0) || (background == "white" && alpha != 65535) {
					t.Fatalf("favicon background changed: %s alpha=%d", background, alpha)
				}
			})
		}
	}
}
