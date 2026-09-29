package postgres

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	blobfs "github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/fs"
	blobsql "github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/sql"
)

func TestMediaInventoryReappearancePreservesIdentity(t *testing.T) {
	previous := env.Config.BlobStorage
	t.Cleanup(func() { env.Config.BlobStorage = previous })
	env.Config.BlobStorage.Type = "fs"
	env.Config.BlobStorage.FS.Path = t.TempDir()

	ctx := inventoryFixture(t)
	blobfs.Service{}.Init()

	const key = "files/reappearing.png"
	store := &cmd.StoreBlob{Key: key, Content: []byte("original"), ContentType: "image/png"}
	if err := bus.Dispatch(ctx, store); err != nil {
		t.Fatal(err)
	}

	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	_, fixture := mediaReferenceTransaction(t)
	for _, statement := range []string{
		`UPDATE media_assets SET name='Human name' WHERE tenant_id=1 AND key=$1`,
		`INSERT INTO media_thumbnails (tenant_id,key,size,content) VALUES (1,$1,200,'cached')`,
		`INSERT INTO command_receipts (tenant_id,user_id,kind,submission_id,fingerprint,result)
		VALUES (1,1,'media-upload','retained-receipt','original-fingerprint',to_jsonb($1::text))`,
		`INSERT INTO pages (tenant_id,title,slug,content,status,visibility,created_by_id,updated_by_id)
		VALUES (1,'Retained reference','retained-reference','![image](/static/images/' || $1 || ')','published','public',1,1)`,
	} {
		if _, err := fixture.Execute(statement, key); err != nil {
			t.Fatal(err)
		}
	}
	indexMediaFixture(t, fixture)
	fixture.MustCommit()

	beforeRemoval := time.Now()
	if err := bus.Dispatch(ctx, &cmd.DeleteBlob{Key: key}); err != nil {
		t.Fatal(err)
	}

	if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	if err := getMediaFile(ctx, &query.GetMediaFile{BlobKey: key}); err != app.ErrNotFound {
		t.Fatalf("absent provider file remains listed: %v", err)
	}

	var name string
	var public, absent, tombstone bool
	var references, receipts, thumbnails int
	err := dbx.Connection().QueryRow(`
		SELECT name,is_public,storage_source IS NULL,deleted_at IS NOT NULL,
		    (SELECT count(*) FROM media_asset_refs WHERE tenant_id=1 AND key=$1),
		    (SELECT count(*) FROM command_receipts WHERE tenant_id=1 AND kind='media-upload' AND result=to_jsonb($1::text)),
		    (SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1)
		FROM media_assets WHERE tenant_id=1 AND key=$1
	`, key).Scan(&name, &public, &absent, &tombstone, &references, &receipts, &thumbnails)
	if err != nil {
		t.Fatal(err)
	}

	if name != "Human name" || public || !absent || tombstone || references != 1 || receipts != 1 || thumbnails != 0 {
		t.Fatalf("absence lost identity: name=%q public=%t absent=%t tombstone=%t refs=%d receipts=%d thumbnails=%d", name, public, absent, tombstone, references, receipts, thumbnails)
	}

	if err := bus.Dispatch(ctx, store); err != nil {
		t.Fatal(err)
	}

	if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	file := &query.GetMediaFile{BlobKey: key}
	if err := getMediaFile(ctx, file); err != nil || file.Result.Name != "Human name" {
		t.Fatalf("reappearance lost the known name: result=%+v error=%v", file.Result, err)
	}

	var catalogedAt time.Time
	if err := dbx.Connection().QueryRow("SELECT is_public,cataloged_at FROM media_assets WHERE tenant_id=1 AND key=$1", key).Scan(&public, &catalogedAt); err != nil {
		t.Fatal(err)
	}

	if public || !catalogedAt.After(beforeRemoval) {
		t.Fatalf("reappearance changed visibility or reused an old prune cutoff: public=%t cataloged=%s", public, catalogedAt)
	}
}

func TestMediaInventoryFailureDoesNotInferAbsence(t *testing.T) {
	for _, provider := range []string{"sql", "fs"} {
		t.Run(provider, func(t *testing.T) {
			previous := env.Config.BlobStorage
			t.Cleanup(func() { env.Config.BlobStorage = previous })
			env.Config.BlobStorage.Type = provider
			env.Config.BlobStorage.FS.Path = t.TempDir()

			ctx := inventoryFixture(t)
			_, err := dbx.Connection().Exec(`
				INSERT INTO media_assets (tenant_id,key,name,content_type,size,storage_source)
				VALUES (1,'files/seen','Seen','image/png',1,$1),
				       (1,'files/unseen','Unseen','image/png',1,$1)
			`, blob.StorageSource())
			if err != nil {
				t.Fatal(err)
			}

			attempt := 0
			bus.AddHandler(func(ctx context.Context, scan *query.ScanBlobMetadata) error {
				attempt++
				if attempt == 1 {
					if err := scan.Accept([]dto.BlobMetadata{{Key: "files/seen", ContentType: "image/png", Size: 1}}, "next", false); err != nil {
						return err
					}

					return errors.New("listing interrupted")
				}

				return scan.Accept(nil, "", true)
			})

			if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err == nil {
				t.Fatal("provider interruption was hidden")
			}

			var available int
			if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_assets WHERE storage_source=$1", blob.StorageSource()).Scan(&available); err != nil || available != 2 {
				t.Fatalf("partial scan inferred absence: available=%d error=%v", available, err)
			}

			if _, err := dbx.Connection().Exec("UPDATE media_inventory SET retry_after=NOW() WHERE tenant_id=1"); err != nil {
				t.Fatal(err)
			}

			if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
				t.Fatal(err)
			}

			want := 1
			if provider == "fs" {
				want = 0
			}

			if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_assets WHERE storage_source=$1", blob.StorageSource()).Scan(&available); err != nil || available != want {
				t.Fatalf("retry used observations from the wrong scan: available=%d want=%d error=%v", available, want, err)
			}
		})
	}
}

func TestMediaInventoryPreservesUploadsAcrossScanCommit(t *testing.T) {
	ctx := inventoryFixture(t)
	ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive})
	blobsql.Service{}.Init()

	_, err := dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id,key,name,content_type,size)
		VALUES (1,'files/existing','Prior name','image/png',1),
		       (1,'files/during','Prior concurrent name','image/png',1)
	`)
	if err != nil {
		t.Fatal(err)
	}

	upload, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer upload.Rollback()

	uploadContext := context.WithValue(ctx, app.TransactionCtxKey, upload)
	if err := saveMediaImage(uploadContext, mediaUpload{
		PreparedImage: &dto.PreparedImage{
			Key:         "files/new",
			Content:     []byte("new"),
			ContentType: "image/png",
		},
		Name: "Concurrent new upload",
	}); err != nil {
		t.Fatal(err)
	}

	if err := saveMediaImage(uploadContext, mediaUpload{
		PreparedImage: &dto.PreparedImage{
			Key:         "files/existing",
			Content:     []byte("replacement"),
			ContentType: "image/png",
		},
		Name: "Upload started before capture",
	}); err != nil {
		t.Fatal(err)
	}

	bus.AddHandler(func(ctx context.Context, scan *query.ScanBlobMetadata) error {
		if err := saveMediaImage(ctx, mediaUpload{
			PreparedImage: &dto.PreparedImage{
				Key:         "files/during",
				Content:     []byte("replaced"),
				ContentType: "image/png",
			},
			Name: "Accepted upload name",
		}); err != nil {
			return err
		}

		if err := upload.Commit(); err != nil {
			return err
		}

		return scan.Accept(nil, "", true)
	})

	if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}

	var available int
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_assets WHERE storage_source='sql'").Scan(&available); err != nil || available != 3 {
		t.Fatalf("scan hid a concurrent accepted upload: available=%d error=%v", available, err)
	}
}

func TestMediaThumbnailCannotReviveFileMissingFromCompletedScan(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_inventory (tenant_id,storage_source,completed_at)
		SELECT id,$1,NOW() FROM tenants WHERE id<>1
		ON CONFLICT (tenant_id,storage_source) DO UPDATE SET completed_at=NOW()
	`, blob.StorageSource())
	if err != nil {
		t.Fatal(err)
	}

	bus.AddHandler(func(ctx context.Context, scan *query.ScanBlobMetadata) error {
		return scan.Accept(nil, "", true)
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		if err := refreshMediaInventory(ctx, &cmd.RefreshMediaInventory{}); err != nil {
			return err
		}

		if err := importMediaInventory(ctx, &cmd.ImportMediaInventory{}); err != nil {
			return err
		}

		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return nil
	})

	thumbnail := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, thumbnail); err != app.ErrNotFound || thumbnail.Result != nil {
		t.Fatalf("late thumbnail restored an absent image: result=%v error=%v", thumbnail.Result, err)
	}

	var absent bool
	var cached int
	if err := dbx.Connection().QueryRow(`
		SELECT storage_source IS NULL,
		    (SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1)
		FROM media_assets WHERE tenant_id=1 AND key=$1
	`, key).Scan(&absent, &cached); err != nil || !absent || cached != 0 {
		t.Fatalf("late derivative persisted stale presence: absent=%t cached=%d error=%v", absent, cached, err)
	}
}

func TestMediaThumbnailUsesChangedProviderMetadata(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")

	var replacement bytes.Buffer
	if err := png.Encode(&replacement, image.NewRGBA(image.Rect(0, 0, 400, 400))); err != nil {
		t.Fatal(err)
	}

	if _, err := dbx.Connection().Exec("UPDATE media_assets SET size=$2 WHERE tenant_id=1 AND key=$1", key, len(original)); err != nil {
		t.Fatal(err)
	}

	reads := 0
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		reads++
		q.Result = &dto.Blob{Content: replacement.Bytes(), ContentType: "image/png"}

		if reads == 1 {
			q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
			return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
				return saveInventoryBatch(trx, tenant.ID, blob.StorageSource(), []dto.BlobMetadata{{
					Key:         key,
					ContentType: "image/png",
					Size:        int64(replacement.Len()),
					ModifiedAt:  time.Now(),
				}})
			})
		}

		return nil
	})

	thumbnail := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, thumbnail); err != nil {
		t.Fatal(err)
	}

	decoded, _, err := image.Decode(bytes.NewReader(thumbnail.Result.Content))
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Bounds().Size() != image.Pt(200, 200) || reads != 2 {
		t.Fatalf("thumbnail published the replaced image: reads=%d bounds=%v", reads, decoded.Bounds())
	}

	var storedSize int
	if err := dbx.Connection().QueryRow("SELECT size FROM media_assets WHERE tenant_id=1 AND key=$1", key).Scan(&storedSize); err != nil || storedSize != replacement.Len() {
		t.Fatalf("thumbnail rewrote newer metadata: size=%d want=%d error=%v", storedSize, replacement.Len(), err)
	}
}

func TestMediaThumbnailStopsWhenProviderKeepsChanging(t *testing.T) {
	ctx, key, original := thumbnailFixture(t, "files")
	reads := 0
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		reads++
		if _, err := dbx.Connection().Exec("UPDATE media_assets SET cataloged_at=clock_timestamp() WHERE tenant_id=1 AND key=$1", key); err != nil {
			return err
		}

		q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
		return nil
	})

	thumbnail := &query.GetMediaThumbnail{Key: key, Size: 200}
	if err := getMediaThumbnail(ctx, thumbnail); err != app.ErrConflict || thumbnail.Result != nil || reads != 3 {
		t.Fatalf("changing image exceeded its retry bound: reads=%d result=%v error=%v", reads, thumbnail.Result, err)
	}

	var cached int
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM media_thumbnails WHERE tenant_id=1 AND key=$1", key).Scan(&cached); err != nil || cached != 0 {
		t.Fatalf("changing image persisted a stale derivative: cached=%d error=%v", cached, err)
	}
}
