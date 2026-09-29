package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func inventoryFixture(t *testing.T) context.Context {
	t.Helper()
	dbx.Seed()
	bus.Reset()
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_inventory (tenant_id, storage_source, completed_at)
		SELECT id, $1, NOW() FROM tenants WHERE id<>1
	`, blob.StorageSource())
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestMediaInventoryResumesAfterFailureWithoutPublishingAttachments(t *testing.T) {
	ctx := inventoryFixture(t)
	first := dto.BlobMetadata{Key: "attachments/imported", ContentType: "image/png", Size: 400}
	second := dto.BlobMetadata{Key: "files/second", ContentType: "image/png", Size: 500}
	attempts := 0
	bus.AddHandler(func(ctx context.Context, q *query.ScanBlobMetadata) error {
		attempts++
		if dbx.Connection().Stats().InUse != 0 {
			t.Error("provider inventory retained a SQL connection")
		}
		if attempts == 1 {
			if err := q.Accept([]dto.BlobMetadata{first}, "accepted-first", false); err != nil {
				return err
			}
			return errors.New("temporary listing failure")
		}
		if q.Cursor != "accepted-first" {
			t.Fatalf("accepted cursor was lost: %q", q.Cursor)
		}
		return q.Accept([]dto.BlobMetadata{second}, "", true)
	})

	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err == nil {
		t.Fatal("provider failure was hidden")
	}
	status := &query.GetMediaInventory{}
	if err := getMediaInventory(ctx, status); err != nil || status.Result.State != "retrying" || status.Result.Scanned != 1 {
		t.Fatalf("wrong retained progress: %+v err=%v", status.Result, err)
	}
	access := &query.CanReadAttachment{Key: first.Key}
	if err := canReadAttachment(ctx, access); err != nil || access.Result {
		t.Fatalf("inventory published an attachment: allowed=%t err=%v", access.Result, err)
	}

	if _, err := dbx.Connection().Exec("UPDATE media_inventory SET retry_after=NOW() WHERE tenant_id=1"); err != nil {
		t.Fatal(err)
	}
	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}
	if err := getMediaInventory(ctx, status); err != nil || status.Result.State != "ready" || status.Result.Scanned != 2 {
		t.Fatalf("inventory did not recover: %+v err=%v", status.Result, err)
	}
}

func TestMediaInventoryRefreshRejectsOldScan(t *testing.T) {
	ctx := inventoryFixture(t)
	file := dto.BlobMetadata{Key: "files/stale", ContentType: "image/png", Size: 20}
	bus.AddHandler(func(scanContext context.Context, q *query.ScanBlobMetadata) error {
		if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
			return err
		}
		err := q.Accept([]dto.BlobMetadata{file}, "old-cursor", true)
		if !errors.Is(err, errInventoryReplaced) {
			t.Fatalf("old scan remained accepted after refresh: %v", err)
		}
		return err
	})
	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_assets WHERE key=$1", file.Key).Scan(&count); err != nil || count != 0 {
		t.Fatalf("old scan published data: rows=%d err=%v", count, err)
	}
	status := &query.GetMediaInventory{}
	if err := getMediaInventory(ctx, status); err != nil || status.Result.State != "pending" || status.Result.Scanned != 0 {
		t.Fatalf("old scan replaced refreshed progress: %+v err=%v", status.Result, err)
	}
}

func TestMediaInventoryKeepsHiddenPostAndPrivatePageAttachmentsPrivate(t *testing.T) {
	ctx := inventoryFixture(t)
	_, fixture := mediaReferenceTransaction(t)
	_, err := fixture.Execute(`
		INSERT INTO posts (id,tenant_id,user_id,title,slug,number,status,created_at,moderation_pending)
		VALUES (1,1,1,'Hidden post','hidden',1,0,NOW(),true);
		INSERT INTO pages (id,tenant_id,created_by_id,updated_by_id,title,slug,content,status,visibility,allowed_roles)
		VALUES (1,1,1,1,'Private page','private','','published','private','[]');
		INSERT INTO comments (id,tenant_id,page_id,user_id,content,created_at)
		VALUES (1,1,1,1,'Private comment',NOW());
		INSERT INTO attachments (tenant_id,user_id,post_id,comment_id,attachment_bkey)
		VALUES (1,1,1,NULL,'attachments/hidden-post'),
		       (1,1,NULL,1,'attachments/private-page');
	`)
	if err != nil {
		t.Fatal(err)
	}
	fixture.BeforeCommit = flushMediaReferences
	fixture.MustCommit()

	files := []dto.BlobMetadata{
		{Key: "attachments/hidden-post", ContentType: "image/png", Size: 20},
		{Key: "attachments/private-page", ContentType: "image/png", Size: 20},
	}
	bus.AddHandler(func(ctx context.Context, q *query.ScanBlobMetadata) error {
		return q.Accept(files, "", true)
	})
	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		access := &query.CanReadAttachment{Key: file.Key}
		if err := canReadAttachment(ctx, access); err != nil || access.Result {
			t.Fatalf("inventory exposed %s: allowed=%t err=%v", file.Key, access.Result, err)
		}
	}
}

func TestMediaInventoryPreservesNamesTombstonesAndUnknownTimeCache(t *testing.T) {
	ctx := inventoryFixture(t)
	files := []dto.BlobMetadata{
		{Key: "files/named", ContentType: "application/octet-stream", Size: 10},
		{Key: "files/deleted", ContentType: "application/octet-stream", Size: 10},
	}
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id,key,name,content_type,size,is_public,deletion_requested_at,deleted_at)
		VALUES (1,'files/named','Human name','image/png',10,true,NULL,NULL),
		       (1,'files/deleted','Deleted name','image/png',10,false,NOW(),NOW());
		INSERT INTO media_thumbnails (tenant_id,key,size,content) VALUES (1,'files/named',200,'cached');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`
		UPDATE media_assets SET storage_source=$1
		WHERE tenant_id=1 AND key IN ('files/named','files/deleted')
	`, blob.StorageSource()); err != nil {
		t.Fatal(err)
	}
	bus.AddHandler(func(ctx context.Context, q *query.ScanBlobMetadata) error {
		return q.Accept(files, "", true)
	})
	for iteration := 0; iteration < 2; iteration++ {
		if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
			t.Fatal(err)
		}
		if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
			t.Fatal(err)
		}
	}

	var name, contentType string
	var public, unknownTime bool
	var thumbnails int
	if err := dbx.Connection().QueryRow(`
		SELECT name, content_type, is_public, storage_modified_at IS NULL,
		    (SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key='files/named')
		FROM media_assets WHERE tenant_id=1 AND key='files/named'
	`).Scan(&name, &contentType, &public, &unknownTime, &thumbnails); err != nil {
		t.Fatal(err)
	}
	if name != "Human name" || contentType != "image/png" || !public || !unknownTime || thumbnails != 1 {
		t.Fatalf("existing metadata/cache changed: name=%q type=%q public=%t unknown=%t thumbnails=%d", name, contentType, public, unknownTime, thumbnails)
	}
	var deleted bool
	if err := dbx.Connection().QueryRow("SELECT deleted_at IS NOT NULL FROM media_assets WHERE tenant_id=1 AND key='files/deleted'").Scan(&deleted); err != nil || !deleted {
		t.Fatalf("inventory resurrected a tombstone: deleted=%t err=%v", deleted, err)
	}

	files[0].ModifiedAt = time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	files[0].Size = 11
	if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
		t.Fatal(err)
	}
	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key='files/named'").Scan(&thumbnails); err != nil || thumbnails != 0 {
		t.Fatalf("changed original retained derived bytes: count=%d err=%v", thumbnails, err)
	}
}

func TestMediaInventoryCompletesIdentityWithoutPublishing(t *testing.T) {
	ctx := inventoryFixture(t)
	const key = "attachments/discovered-later.png"
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id,key,storage_source,cataloged_at)
		VALUES (1,$1,NULL,NULL)
	`, key)
	if err != nil {
		t.Fatal(err)
	}

	modifiedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	file := dto.BlobMetadata{
		Key: key, ContentType: "image/png", Size: 400, ModifiedAt: modifiedAt,
	}
	bus.AddHandler(func(ctx context.Context, q *query.ScanBlobMetadata) error {
		return q.Accept([]dto.BlobMetadata{file}, "", true)
	})
	beforeImport := time.Now().Truncate(time.Microsecond)
	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	var name, source, contentType string
	var size int64
	var public bool
	var createdAt, catalogedAt time.Time
	if err := dbx.Connection().QueryRow(`
		SELECT name,storage_source,content_type,size,is_public,created_at,cataloged_at
		FROM media_assets WHERE tenant_id=1 AND key=$1
	`, key).Scan(&name, &source, &contentType, &size, &public, &createdAt, &catalogedAt); err != nil {
		t.Fatal(err)
	}
	if name != "discovered-later.png" || source != blob.StorageSource() || contentType != "image/png" || size != 400 || public {
		t.Fatalf("wrong completed metadata: name=%q source=%q type=%q size=%d public=%t", name, source, contentType, size, public)
	}
	if !createdAt.Equal(modifiedAt) || catalogedAt.Before(beforeImport) {
		t.Fatalf("provider and discovery dates were conflated: created=%s cataloged=%s", createdAt, catalogedAt)
	}

	if _, err := dbx.Connection().Exec("UPDATE media_assets SET name='Human name' WHERE tenant_id=1 AND key=$1", key); err != nil {
		t.Fatal(err)
	}
	if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
		t.Fatal(err)
	}
	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	var refreshedCatalogedAt time.Time
	if err := dbx.Connection().QueryRow("SELECT name,cataloged_at FROM media_assets WHERE tenant_id=1 AND key=$1", key).Scan(&name, &refreshedCatalogedAt); err != nil {
		t.Fatal(err)
	}
	if name != "Human name" || !refreshedCatalogedAt.Equal(catalogedAt) {
		t.Fatalf("refresh changed accepted metadata: name=%q cataloged=%s", name, refreshedCatalogedAt)
	}
}
