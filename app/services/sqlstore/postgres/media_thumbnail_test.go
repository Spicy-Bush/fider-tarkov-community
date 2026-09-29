package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func thumbnailFixture(t *testing.T, prefix string) (context.Context, string, []byte) {
	t.Helper()
	bus.Reset()
	key := fmt.Sprintf("%s/thumbnail-%d", prefix, time.Now().UnixNano())
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id, key, name, content_type, size, is_public, storage_source)
		VALUES (1, $1, 'Preserved name', 'image/png', 0, true, $2)
	`, key, blob.StorageSource())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := dbx.Connection().Exec("DELETE FROM media_assets WHERE tenant_id=1 AND key=$1", key); err != nil {
			t.Error(err)
		}
	})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 400, 200))); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive})
	return ctx, key, encoded.Bytes()
}

func TestMediaThumbnailRecoveryAndCachedAuthority(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")
	reads := 0
	unavailable := true
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		reads++
		if dbx.Connection().Stats().InUse != 0 {
			t.Error("original retrieval retained a SQL connection")
		}
		if unavailable {
			return errors.New("temporary provider failure")
		}
		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return nil
	})

	request := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, request); err == nil || request.Result != nil {
		t.Fatal("provider failure returned a thumbnail")
	}
	unavailable = false
	if err := getMediaThumbnail(ctx, request); err != nil {
		t.Fatal(err)
	}
	decoded, format, err := image.Decode(bytes.NewReader(request.Result.Content))
	if err != nil || format != "webp" || decoded.Bounds().Size() != image.Pt(200, 100) {
		t.Fatalf("wrong derivative format=%s err=%v", format, err)
	}
	unavailable = true
	if err := getMediaThumbnail(ctx, request); err != nil || reads != 2 {
		t.Fatalf("cached derivative reloaded original: reads=%d err=%v", reads, err)
	}
	visitor := context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive})
	if err := getMediaThumbnail(visitor, request); err != blob.ErrNotFound || request.Result != nil {
		t.Fatalf("cached private image exposed: %v", err)
	}
	otherTenant := context.WithValue(ctx, app.TenantCtxKey, &entity.Tenant{ID: 2})
	if err := getMediaThumbnail(otherTenant, request); err == nil || request.Result != nil || reads != 3 {
		t.Fatalf("another tenant received cached image: reads=%d err=%v", reads, err)
	}

	var name string
	var public bool
	var width, height int
	if err := dbx.Connection().QueryRow("SELECT name,is_public,width,height FROM media_assets WHERE tenant_id=1 AND key=$1", key).Scan(&name, &public, &width, &height); err != nil {
		t.Fatal(err)
	}
	if name != "Preserved name" || !public || width != 0 || height != 0 {
		t.Fatalf("metadata changed: name=%q public=%t size=%dx%d", name, public, width, height)
	}

	if _, err := dbx.Connection().Exec("UPDATE media_assets SET deletion_requested_at=NOW() WHERE tenant_id=1 AND key=$1", key); err != nil {
		t.Fatal(err)
	}
	if err := getMediaThumbnail(ctx, request); err != app.ErrNotFound || request.Result != nil || reads != 3 {
		t.Fatalf("deleting asset served its cached image: reads=%d err=%v", reads, err)
	}
}

func TestMediaThumbnailPreservesSmallAnimatedImage(t *testing.T) {
	ctx, key, _ := thumbnailFixture(t, "files")
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 20, 10), palette)
	second := image.NewPaletted(image.Rect(0, 0, 20, 10), palette)
	second.SetColorIndex(0, 0, 1)
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		q.Result = &dto.Blob{Content: encoded.Bytes(), ContentType: "image/gif"}
		return nil
	})
	for attempt := 0; attempt < 2; attempt++ {
		request := &query.GetMediaThumbnail{Key: key, Size: 200}
		if err := getMediaThumbnail(ctx, request); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(request.Result.Content, encoded.Bytes()) || request.Result.ContentType != "image/gif" {
			t.Fatal("small animated image was converted")
		}
	}
	var thumbnails int
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1", key).Scan(&thumbnails); err != nil || thumbnails != 0 {
		t.Fatalf("unnecessary derivative rows=%d err=%v", thumbnails, err)
	}
}

func TestMediaThumbnailCachedAvatarTracksPublication(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "avatars")
	published := true
	reads := 0
	bus.AddHandler(func(ctx context.Context, q *query.IsAvatarPublished) error {
		q.Result = published
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		reads++
		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return nil
	})

	request := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, request); err != nil {
		t.Fatal(err)
	}

	published = false
	if err := getMediaThumbnail(ctx, request); err != blob.ErrNotFound || request.Result != nil {
		t.Fatalf("unpublished cached avatar was exposed: %v", err)
	}

	request.AllowUnpublishedAvatar = true
	if err := getMediaThumbnail(ctx, request); err != nil || reads != 1 {
		t.Fatalf("administrator could not preview cached proposal: reads=%d err=%v", reads, err)
	}

	visitor := context.WithValue(ctx, app.UserCtxKey, &entity.User{
		ID:     2,
		Role:   enum.RoleVisitor,
		Status: enum.UserActive,
	})
	if err := getMediaThumbnail(visitor, request); err != app.ErrNotFound || request.Result != nil {
		t.Fatalf("untrusted caller bypassed avatar publication: %v", err)
	}
}

func TestMediaThumbnailRejectsOpenTransaction(t *testing.T) {
	ctx, key, _ := thumbnailFixture(t, "files")
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.MustRollback()

	request := &query.GetMediaThumbnail{Key: key, Size: 200}
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)
	if err := getMediaThumbnail(ctx, request); err == nil || request.Result != nil {
		t.Fatal("thumbnail work accepted an inherited transaction")
	}
}

func TestMediaThumbnailIgnoresOldProviderCacheUntilInventoryRefresh(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")
	if _, err := dbx.Connection().Exec(`
		UPDATE media_assets SET storage_source='old-provider', content_type='application/octet-stream'
		WHERE tenant_id=1 AND key=$1
	`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`
		INSERT INTO media_thumbnails (tenant_id,key,size,content)
		VALUES (1,$1,200,'old-thumbnail'), (1,$1,512,'old-thumbnail')
	`, key); err != nil {
		t.Fatal(err)
	}
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		q.Result = &dto.Blob{Content: original, ContentType: "application/octet-stream"}
		return nil
	})

	request := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, format, err := image.Decode(bytes.NewReader(request.Result.Content)); err != nil || format != "webp" {
		t.Fatalf("old provider bytes were served: format=%s err=%v", format, err)
	}
	var source, contentType string
	var oldDerivatives int
	if err := dbx.Connection().QueryRow(`
		SELECT storage_source,content_type,
		    (SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1 AND size=512)
		FROM media_assets WHERE tenant_id=1 AND key=$1
	`, key).Scan(&source, &contentType, &oldDerivatives); err != nil {
		t.Fatal(err)
	}
	if source != "old-provider" || contentType != "application/octet-stream" || oldDerivatives != 1 {
		t.Fatalf("thumbnail changed provider metadata: source=%s type=%s old=%d", source, contentType, oldDerivatives)
	}

	if err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		return saveInventoryBatch(trx, tenant.ID, blob.StorageSource(), []dto.BlobMetadata{{
			Key: key, ContentType: "image/png", Size: int64(len(original)),
		}})
	}); err != nil {
		t.Fatal(err)
	}

	if err := getMediaThumbnail(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, format, err := image.Decode(bytes.NewReader(request.Result.Content)); err != nil || format != "webp" {
		t.Fatalf("inventory did not replace old provider bytes: format=%s err=%v", format, err)
	}
	if err := dbx.Connection().QueryRow(`
		SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1 AND size=512
	`, key).Scan(&oldDerivatives); err != nil || oldDerivatives != 0 {
		t.Fatalf("inventory retained the previous provider derivative: count=%d err=%v", oldDerivatives, err)
	}
}

func TestMediaThumbnailDeletionDuringGeneration(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		_, err := dbx.Connection().Exec("UPDATE media_assets SET deletion_requested_at=NOW() WHERE tenant_id=1 AND key=$1", key)
		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return err
	})
	request := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, request); err != app.ErrNotFound || request.Result != nil {
		t.Fatalf("generation survived deletion: %v", err)
	}
	var thumbnails int
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1", key).Scan(&thumbnails); err != nil || thumbnails != 0 {
		t.Fatalf("late thumbnail persisted: rows=%d err=%v", thumbnails, err)
	}
}

func TestMediaThumbnailBoundsOriginalReadsAndWaitingCancellation(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	var releaseOnce sync.Once
	var workers sync.WaitGroup
	defer func() {
		releaseOnce.Do(func() { close(release) })
		workers.Wait()
	}()
	var reads atomic.Int32
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		reads.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return nil
	})
	results := make(chan error, 8)
	for request := 0; request < 8; request++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			results <- getMediaThumbnail(ctx, &query.GetMediaThumbnail{Key: key, Size: 200})
		}()
	}
	for entered := 0; entered < 2; entered++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("generation did not reach original reads")
		}
	}

	waiting, cancel := context.WithCancel(ctx)
	defer cancel()
	canceled := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		canceled <- getMediaThumbnail(waiting, &query.GetMediaThumbnail{Key: key, Size: 200})
	}()
	select {
	case err := <-canceled:
		t.Fatalf("request did not wait for generation capacity: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting request cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request remained queued")
	}
	if reads.Load() != 2 {
		t.Fatalf("waiting requests loaded original images: %d", reads.Load())
	}
	releaseOnce.Do(func() { close(release) })
	for request := 0; request < 8; request++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if reads.Load() != 2 {
		t.Fatalf("cache recheck did not suppress queued generation: %d", reads.Load())
	}
}
