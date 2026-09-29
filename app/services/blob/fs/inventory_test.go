package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func TestFilesystemInventoryBatchesAndStops(t *testing.T) {
	root, ctx := filesystemFixture(t)
	directory := filepath.Join(root, "tenants", "1", "images")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 405; index++ {
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%04d.png", index)), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../../2", filepath.Join(directory, "other-tenant")); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	pages := 0
	request := &query.ScanBlobMetadata{BatchSize: 200}
	request.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
		pages++
		if len(files) > 200 || next != "" {
			t.Fatalf("unbounded or resumable filesystem page: rows=%d cursor=%q", len(files), next)
		}
		for _, file := range files {
			if seen[file.Key] || file.Size != 7 || file.ContentType != "image/png" {
				t.Fatalf("invalid inventory entry: %+v", file)
			}
			seen[file.Key] = true
		}
		if complete && len(seen) != 405 {
			t.Fatalf("incomplete traversal marked complete: %d", len(seen))
		}
		return nil
	}
	if err := backend.Scan(ctx, request); err != nil || pages != 3 || len(seen) != 405 {
		t.Fatalf("pages=%d entries=%d err=%v", pages, len(seen), err)
	}

	stop := errors.New("scan replaced")
	pages = 0
	request.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
		pages++
		return stop
	}
	if err := backend.Scan(ctx, request); !errors.Is(err, stop) || pages != 1 {
		t.Fatalf("stale scan kept traversing: pages=%d err=%v", pages, err)
	}
}

func TestFilesystemInventorySkipsInvalidStoredNames(t *testing.T) {
	root, ctx := filesystemFixture(t)
	directory := filepath.Join(root, "tenants", "1")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid.webp", "old file.webp", "back\\slash.webp"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	listed := &query.ListBlobs{}
	if err := backend.List(ctx, listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Result) != 1 || listed.Result[0] != "valid.webp" || listed.Skipped != 2 {
		t.Fatalf("listing=%v skipped=%d", listed.Result, listed.Skipped)
	}

	seen, complete := 0, false
	scan := &query.ScanBlobMetadata{BatchSize: 1}
	scan.Accept = func(files []dto.BlobMetadata, next string, done bool) error {
		for _, file := range files {
			if file.Key != "valid.webp" {
				t.Fatalf("invalid file reached catalog: %q", file.Key)
			}
			seen++
		}
		complete = done
		return nil
	}
	if err := backend.Scan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	if seen != 1 || scan.Skipped != 2 || !complete {
		t.Fatalf("seen=%d skipped=%d complete=%t", seen, scan.Skipped, complete)
	}
}

func BenchmarkFilesystemInventoryFirstBatch(b *testing.B) {
	for _, count := range []int{1000, 50000} {
		b.Run(fmt.Sprintf("files%d", count), func(b *testing.B) {
			previous := env.Config.BlobStorage.FS.Path
			root := b.TempDir()
			env.Config.BlobStorage.FS.Path = root
			b.Cleanup(func() { env.Config.BlobStorage.FS.Path = previous })
			directory := filepath.Join(root, "tenants", "1")
			if err := os.MkdirAll(directory, 0700); err != nil {
				b.Fatal(err)
			}
			for index := 0; index < count; index++ {
				if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%06d", index)), nil, 0600); err != nil {
					b.Fatal(err)
				}
			}
			ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
			stop := errors.New("first batch measured")
			request := &query.ScanBlobMetadata{BatchSize: 200}
			request.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
				if len(files) != 200 || complete {
					b.Fatalf("wrong batch: size=%d complete=%t", len(files), complete)
				}
				return stop
			}

			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				if err := backend.Scan(ctx, request); !errors.Is(err, stop) {
					b.Fatal(err)
				}
			}
		})
	}
}
