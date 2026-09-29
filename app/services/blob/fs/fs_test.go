package fs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func TestFilesystemReplacementPublishesCompleteObjects(t *testing.T) {
	_, ctx := filesystemFixture(t)
	content := bytes.Repeat([]byte("complete image content"), 200000)
	key := "attachments/retry.webp"
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: key, Content: content}); err != nil {
		t.Fatal(err)
	}

	var readers sync.WaitGroup
	done := make(chan struct{})
	failures := make(chan error, 4)
	for range 3 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				read := &query.GetBlobByKey{Key: key}
				if err := backend.Get(ctx, read); err != nil {
					failures <- err
					return
				}
				if !bytes.Equal(read.Result.Content, content) {
					failures <- errors.New("reader observed an incomplete replacement")
					return
				}
			}
		}()
	}
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-done:
				return
			default:
			}

			listed := &query.ListBlobs{}
			if err := backend.List(ctx, listed); err != nil {
				failures <- err
				return
			}
			if !reflect.DeepEqual(listed.Result, []string{key}) || listed.Skipped != 0 {
				failures <- errors.New("listing observed a staged replacement")
				return
			}

			scan := &query.ScanBlobMetadata{BatchSize: 20}
			scan.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
				for _, file := range files {
					if file.Key != key || file.Size != int64(len(content)) {
						return errors.New("inventory observed an unpublished replacement")
					}
				}
				return nil
			}
			if err := backend.Scan(ctx, scan); err != nil {
				failures <- err
				return
			}
			if scan.Skipped != 0 {
				failures <- errors.New("inventory counted an internal staged file as an invalid blob")
				return
			}
		}
	}()

	for range 20 {
		if err := backend.Store(ctx, &cmd.StoreBlob{Key: key, Content: content}); err != nil {
			t.Error(err)
			break
		}
	}
	close(done)
	readers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}

	list := &query.ListBlobs{Prefix: "attachments/"}
	if err := backend.List(ctx, list); err != nil || !reflect.DeepEqual(list.Result, []string{key}) {
		t.Fatalf("replacement retained temporary objects: %v %v", list.Result, err)
	}
}

func filesystemFixture(t *testing.T) (string, context.Context) {
	t.Helper()
	previous := env.Config.BlobStorage.FS.Path
	root := t.TempDir()
	env.Config.BlobStorage.FS.Path = root
	t.Cleanup(func() { env.Config.BlobStorage.FS.Path = previous })
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	return root, ctx
}

func TestFilesystemStagingRemainsPrivateUntilPublication(t *testing.T) {
	_, ctx := filesystemFixture(t)
	public := []string{"images/.fider-upload-existing.webp", "images/visible.webp"}
	for _, key := range public {
		if err := backend.Store(ctx, &cmd.StoreBlob{Key: key, Content: []byte("published")}); err != nil {
			t.Fatal(err)
		}
	}

	root, err := openRoot(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	stagedKey := "images/" + stagingDirectory + "/interrupted"
	writer, err := root.OpenFile(stagedKey, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.WriteString("partial"); err != nil {
		t.Fatal(err)
	}

	checkDiscovery := func() {
		t.Helper()
		listed := &query.ListBlobs{}
		if err := backend.List(ctx, listed); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(listed.Result, public) || listed.Skipped != 0 {
			t.Fatalf("staging changed listing: files=%v skipped=%d", listed.Result, listed.Skipped)
		}

		var scanned []string
		scan := &query.ScanBlobMetadata{BatchSize: 1}
		scan.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
			for _, file := range files {
				scanned = append(scanned, file.Key)
			}
			return nil
		}
		if err := backend.Scan(ctx, scan); err != nil {
			t.Fatal(err)
		}
		slices.Sort(scanned)
		if !reflect.DeepEqual(scanned, public) || scan.Skipped != 0 {
			t.Fatalf("staging changed inventory: files=%v skipped=%d", scanned, scan.Skipped)
		}
	}

	checkDiscovery()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	checkDiscovery()

	if err := backend.Get(ctx, &query.GetBlobByKey{Key: stagedKey}); err != blob.ErrNotFound {
		t.Fatalf("staging read was accepted: %v", err)
	}
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: stagedKey, Content: []byte("replacement")}); err != blob.ErrInvalidKeyFormat {
		t.Fatalf("staging write was accepted: %v", err)
	}
	if err := backend.Delete(ctx, &cmd.DeleteBlob{Key: stagedKey}); err != blob.ErrInvalidKeyFormat {
		t.Fatalf("staging deletion was accepted: %v", err)
	}

	if err := backend.Store(ctx, &cmd.StoreBlob{Key: "images/recovered.webp", Content: []byte("complete replacement")}); err != nil {
		t.Fatal(err)
	}
	read := &query.GetBlobByKey{Key: "images/recovered.webp"}
	if err := backend.Get(ctx, read); err != nil || string(read.Result.Content) != "complete replacement" {
		t.Fatalf("publication changed replacement bytes: %+v error=%v", read.Result, err)
	}
	public = append(public, "images/recovered.webp")
	slices.Sort(public)
	checkDiscovery()

	if err := root.Mkdir("images/occupied", 0700); err != nil {
		t.Fatal(err)
	}
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: "images/occupied", Content: []byte("cannot replace a directory")}); err == nil {
		t.Fatal("failed publication was accepted")
	}
	staging, err := root.Open("images/" + stagingDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer staging.Close()
	remaining, err := staging.Readdirnames(-1)
	if err != nil || !reflect.DeepEqual(remaining, []string{"interrupted"}) {
		t.Fatalf("publication cleanup changed another writer's staging: files=%v error=%v", remaining, err)
	}
}

func TestCanonicalKeysAcrossOperations(t *testing.T) {
	_, ctx := filesystemFixture(t)
	for _, key := range []string{"../escape", "a/../../escape", "a//b", "a/./b", "a/../b", "/absolute", "back\\slash", "a\x00b", "a\nb", "a/", ""} {
		t.Run(key, func(t *testing.T) {
			if err := backend.Store(ctx, &cmd.StoreBlob{Key: key, Content: []byte("unsafe")}); err == nil {
				t.Fatal("invalid write accepted")
			}
			if err := backend.Get(ctx, &query.GetBlobByKey{Key: key}); err == nil {
				t.Fatal("invalid read accepted")
			}
			if err := backend.Delete(ctx, &cmd.DeleteBlob{Key: key}); err == nil {
				t.Fatal("invalid delete accepted")
			}
		})
	}
}

func TestFilesystemScopeAndSymlinks(t *testing.T) {
	root, ctx := filesystemFixture(t)
	other := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 2})
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: "images/one", Content: []byte("one")}); err != nil {
		t.Fatal(err)
	}
	if err := backend.Store(other, &cmd.StoreBlob{Key: "secret", Content: []byte("two")}); err != nil {
		t.Fatal(err)
	}
	if err := backend.Store(context.Background(), &cmd.StoreBlob{Key: "public", Content: []byte("global")}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../2", filepath.Join(root, "tenants", "1", "escape")); err != nil {
		t.Fatal(err)
	}

	if err := backend.Get(ctx, &query.GetBlobByKey{Key: "escape/secret"}); err == nil {
		t.Fatal("symlink read crossed tenant root")
	}
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: "escape/secret", Content: []byte("changed")}); err == nil {
		t.Fatal("symlink write crossed tenant root")
	}
	if err := backend.Delete(ctx, &cmd.DeleteBlob{Key: "escape/secret"}); err == nil {
		t.Fatal("symlink delete crossed tenant root")
	}
	secret := &query.GetBlobByKey{Key: "secret"}
	if err := backend.Get(other, secret); err != nil || string(secret.Result.Content) != "two" {
		t.Fatalf("other tenant changed: %v", err)
	}

	for _, test := range []struct {
		ctx    context.Context
		prefix string
		want   []string
	}{
		{ctx, "", []string{"images/one"}},
		{ctx, "images/o", []string{"images/one"}},
		{ctx, "images/", []string{"images/one"}},
		{context.Background(), "", []string{"public"}},
	} {
		list := &query.ListBlobs{Prefix: test.prefix}
		if err := backend.List(test.ctx, list); err != nil || !reflect.DeepEqual(list.Result, test.want) {
			t.Fatalf("prefix %q: files=%v err=%v", test.prefix, list.Result, err)
		}
	}

	if err := os.RemoveAll(filepath.Join(root, "tenants", "1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("2", filepath.Join(root, "tenants", "1")); err != nil {
		t.Fatal(err)
	}
	if err := backend.Get(ctx, &query.GetBlobByKey{Key: "secret"}); err != blob.ErrInvalidKeyFormat {
		t.Fatalf("aliased tenant root accepted: %v", err)
	}
}

func TestFilesystemReadLimit(t *testing.T) {
	_, ctx := filesystemFixture(t)
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: "large", Content: make([]byte, 4096)}); err != nil {
		t.Fatal(err)
	}
	request := &query.GetBlobByKey{Key: "large", MaxBytes: 64}
	if err := backend.Get(ctx, request); !errors.Is(err, readlimit.ErrTooLarge) || request.Result != nil {
		t.Fatalf("large filesystem object bypassed cap: %v", err)
	}
	if err := backend.Store(ctx, &cmd.StoreBlob{Key: "large", Content: []byte("small")}); err != nil {
		t.Fatal(err)
	}
	if err := backend.Get(ctx, request); err != nil || string(request.Result.Content) != "small" {
		t.Fatalf("healthy replacement failed: %v", err)
	}
}
