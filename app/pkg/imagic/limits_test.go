package imagic_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
)

func pngHeader(t testing.TB, width, height uint32) []byte {
	t.Helper()

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	header := encoded.Bytes()[:33]
	binary.BigEndian.PutUint32(header[16:20], width)
	binary.BigEndian.PutUint32(header[20:24], height)
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	return header
}

func TestDecodeRejectsOversizedImages(t *testing.T) {
	for _, test := range []struct {
		name    string
		content []byte
		err     error
	}{
		{"pixel bomb", pngHeader(t, 100000, 100000), imagic.ErrTooLarge},
		{"compressed bytes", make([]byte, imagic.MaxImageBytes+1), imagic.ErrTooManyBytes},
		{"truncated pixels", pngHeader(t, 2, 2), imagic.ErrNotSupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := imagic.Decode(test.content)
			if !errors.Is(err, test.err) {
				t.Fatalf("decode error: got %v, want %v", err, test.err)
			}
		})
	}
}

func BenchmarkRejectImageDimensions(b *testing.B) {
	header := pngHeader(b, 100000, 100000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, _, err := imagic.Decode(header); err != imagic.ErrTooLarge {
			b.Fatal(err)
		}
	}
}

func TestImageHeaderLimits(t *testing.T) {
	tests := []struct {
		name          string
		width, height uint32
		accepted      bool
	}{
		{"pixel boundary", 5000, 5000, true},
		{"too many pixels", 5000, 5001, false},
		{"width boundary", 8192, 1, true},
		{"too wide", 8193, 1, false},
		{"too tall", 1, 8193, false},
		{"decompression bomb", 100000, 100000, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := pngHeader(t, test.width, test.height)
			_, err := imagic.Parse(header)
			if (err == nil) != test.accepted {
				t.Fatalf("%dx%d header: accepted=%v error=%v", test.width, test.height, test.accepted, err)
			}
		})
	}
}
