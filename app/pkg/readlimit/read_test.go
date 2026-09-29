package readlimit_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
)

type endlessReader struct {
	read int
}

func (reader *endlessReader) Read(destination []byte) (int, error) {
	clear(destination)
	reader.read += len(destination)
	return len(destination), nil
}

func TestReadLimitBoundsUntrustedStream(t *testing.T) {
	reader := &endlessReader{}
	content, err := readlimit.ReadAll(reader, 64)
	if !errors.Is(err, readlimit.ErrTooLarge) || content != nil || reader.read != 65 {
		t.Fatalf("unbounded read: bytes=%d content=%d err=%v", reader.read, len(content), err)
	}
	content, err = readlimit.ReadAll(bytes.NewReader(make([]byte, 64)), 64)
	if err != nil || len(content) != 64 {
		t.Fatalf("exact boundary rejected: bytes=%d err=%v", len(content), err)
	}
}

func TestReadLimitPreservesReaderFailureAndLargeLimit(t *testing.T) {
	failure := errors.New("connection closed")
	reader := io.MultiReader(bytes.NewBufferString("partial"), iotest.ErrReader(failure))
	content, err := readlimit.ReadAll(reader, 64)
	if !errors.Is(err, failure) || content != nil {
		t.Fatalf("partial content replaced read failure: content=%q err=%v", content, err)
	}

	content, err = readlimit.ReadAll(bytes.NewBufferString("complete"), math.MaxInt64)
	if err != nil || string(content) != "complete" {
		t.Fatalf("large limit truncated content: content=%q err=%v", content, err)
	}
}

func TestReadLimitSurvivesFileGrowthAfterStat(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "growing"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() != 0 {
		t.Fatalf("initial file size: %v", err)
	}
	if err := file.Truncate(4096); err != nil {
		t.Fatal(err)
	}
	content, err := readlimit.ReadAll(file, 64)
	if !errors.Is(err, readlimit.ErrTooLarge) || content != nil {
		t.Fatalf("file growth bypassed read limit: bytes=%d err=%v", len(content), err)
	}
}
