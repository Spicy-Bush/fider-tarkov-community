package imagic_test

import (
	"bytes"
	"image"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
)

func TestThumbnailDimensions(t *testing.T) {
	for _, test := range []struct {
		source image.Point
		size int
		want image.Point
	}{
		{image.Pt(800, 400), 200, image.Pt(200, 100)},
		{image.Pt(400, 800), 512, image.Pt(256, 512)},
		{image.Pt(80, 40), 200, image.Pt(80, 40)},
	} {
		source := image.NewRGBA(image.Rectangle{Max: test.source})
		content, err := imagic.Thumbnail(source, test.size)
		if err != nil {
			t.Fatal(err)
		}
		decoded, format, err := image.Decode(bytes.NewReader(content))
		if err != nil || format != "webp" || decoded.Bounds().Size() != test.want {
			t.Fatalf("thumbnail size=%d format=%s err=%v", test.size, format, err)
		}
	}
	if _, err := imagic.Thumbnail(image.NewRGBA(image.Rect(0, 0, 8, 8)), 201); err != imagic.ErrThumbnailSize {
		t.Fatalf("unsupported size accepted: %v", err)
	}
}
