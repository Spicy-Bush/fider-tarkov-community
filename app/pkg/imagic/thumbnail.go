package imagic

import (
	"fmt"
	"image"
)

var ErrThumbnailSize = fmt.Errorf("Thumbnail size must be %d or %d pixels", ThumbnailSmall, ThumbnailLarge)

const (
	ThumbnailSmall = 200
	ThumbnailLarge = 512
)

func ValidThumbnailSize(size int) bool {
	return size == ThumbnailSmall || size == ThumbnailLarge
}

func Thumbnail(src image.Image, size int) ([]byte, error) {
	if !ValidThumbnailSize(size) {
		return nil, ErrThumbnailSize
	}
	return EncodeWebP(Resize(size)(src, ""))
}
