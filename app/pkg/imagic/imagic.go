package imagic

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"golang.org/x/image/draw"
	xwebp "golang.org/x/image/webp"
)

var ErrNotSupported = errors.New("File not supported")
var ErrTooLarge = errors.New("Image must be at most 8192 pixels per side and 25 megapixels")
var ErrTooManyBytes = fmt.Errorf("Image must be at most %dKB", MaxImageKilobytes)

const MaxImageKilobytes = 50000
const MaxImageBytes = MaxImageKilobytes * 1024

// File contains metadata of a given image
type File struct {
	Width  int
	Height int
	Size   int
	Format string
}

// Parse returns a File if it's in a supported format
func Parse(file []byte) (*File, error) {
	if len(file) > MaxImageBytes {
		return nil, ErrTooManyBytes
	}

	reader := bytes.NewReader(file)
	var cfg image.Config
	var format string
	var err error

	// The encoder also registers a WebP decoder; use Go's decoder for untrusted input.
	if len(file) >= 12 && string(file[:4]) == "RIFF" && string(file[8:12]) == "WEBP" {
		cfg, err = xwebp.DecodeConfig(reader)
		format = "webp"
	} else {
		cfg, format, err = image.DecodeConfig(reader)
	}

	if err != nil || (format != "png" && format != "gif" && format != "jpeg" && format != "webp") {
		return nil, ErrNotSupported
	}

	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || cfg.Width > 25000000/cfg.Height {
		return nil, ErrTooLarge
	}

	return &File{
		Size:   len(file),
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: format,
	}, nil
}

// ImageOperation is an operation that can be performed on an image and return a modified version of it
type ImageOperation func(image.Image, string) image.Image

// ChangeBackground changes a transparent background to the given color (only if PNG)
func ChangeBackground(bgColor color.Color) ImageOperation {
	return func(src image.Image, format string) image.Image {
		if format != "png" && format != "webp" {
			return src
		}
		dst := image.NewRGBA(src.Bounds())
		draw.Draw(dst, dst.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Over)
		return dst
	}
}

// Padding adds padding (in pixels) around the image
func Padding(padding int) ImageOperation {
	return func(src image.Image, format string) image.Image {
		if padding == 0 {
			return src
		}
		srcBounds := src.Bounds()
		srcW, srcH := srcBounds.Dx(), srcBounds.Dy()

		dst := image.NewRGBA(image.Rect(0, 0, srcW+padding, srcH+padding))
		draw.Draw(dst, dst.Bounds(), src, image.Pt(-padding/2, -padding/2), draw.Src)
		return dst
	}
}

// Resize image to fit within the given size (whichever dimension is larger)
func Resize(size int) ImageOperation {
	return func(src image.Image, format string) image.Image {
		b := src.Bounds()
		srcW, srcH := b.Dx(), b.Dy()

		if size >= srcW && size >= srcH {
			return src
		}

		if srcW > srcH {
			return imaging.Resize(src, size, 0, imaging.Linear)
		}
		return imaging.Resize(src, 0, size, imaging.Linear)
	}
}

// Apply a list of operations on a given image
// Returns the final image bytes in WEBP format
func Apply(input []byte, operations ...ImageOperation) ([]byte, error) {
	img, format, err := Decode(input)
	if err != nil {
		return nil, err
	}

	// Apply each operation in order
	for _, op := range operations {
		img = op(img, format)
	}

	// Encode final result as WebP
	return EncodeWebP(img)
}

// Decode checks resource limits before allocating pixels.
func Decode(file []byte) (image.Image, string, error) {
	metadata, err := Parse(file)
	if err != nil {
		return nil, "", err
	}

	reader := bytes.NewReader(file)
	var src image.Image
	if metadata.Format == "webp" {
		src, err = xwebp.Decode(reader)
	} else {
		src, _, err = image.Decode(reader)
	}

	if err != nil {
		return nil, "", ErrNotSupported
	}

	return src, metadata.Format, nil
}

func EncodeWebP(img image.Image) ([]byte, error) {
	if _, ok := img.(*image.RGBA); !ok {
		// Bulk conversion avoids the encoder's allocation for each non-RGBA pixel.
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
		img = rgba
	}

	var buf bytes.Buffer
	options := &webp.Options{
		Lossless: false,
		Quality:  80,
	}
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
