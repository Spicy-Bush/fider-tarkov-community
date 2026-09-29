package postgres

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

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
)

func TestMediaStoragePublicUploadSurvivesConcurrentInventory(t *testing.T) {
	dbx.Seed()
	bus.Reset()
	previous := env.Config.BlobStorage
	t.Cleanup(func() { env.Config.BlobStorage = previous })
	env.Config.BlobStorage.Type = "fs"
	env.Config.BlobStorage.FS.Path = t.TempDir()
	tenantContext := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	ctx := context.WithValue(tenantContext, app.UserCtxKey, &entity.User{
		ID:     1,
		Role:   enum.RoleAdministrator,
		Status: enum.UserActive,
	})
	stores := 0
	bus.AddHandler(func(ctx context.Context, c *cmd.StoreBlob) error {
		stores++
		if dbx.Connection().Stats().InUse != 0 {
			t.Error("external image write retained a SQL connection")
		}
		err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			return saveInventoryBatch(trx, tenant.ID, blob.StorageSource(), []dto.BlobMetadata{{
				Key:         c.Key,
				Size:        int64(len(c.Content)),
				ContentType: c.ContentType,
			}})
		})
		if err != nil {
			return err
		}
		var public bool
		if err := dbx.Connection().QueryRow("SELECT is_public FROM media_assets WHERE tenant_id=1 AND key=$1", c.Key).Scan(&public); err != nil {
			return err
		}
		if public {
			t.Error("inventory itself published the image")
		}
		return nil
	})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	upload := &cmd.UploadImageFile{
		Name:         "Public illustration",
		Content:      encoded.Bytes(),
		Type:         enum.FileUploadPublic,
		SubmissionID: "inventory-interleaving",
	}

	if err := uploadImageFile(ctx, upload); err != nil {
		t.Fatal(err)
	}
	if upload.Result.Name != "Public illustration" {
		t.Fatalf("inventory replaced the accepted upload name: %q", upload.Result.Name)
	}
	access := &query.CanReadAttachment{Key: upload.Result.BlobKey}
	if err := canReadAttachment(tenantContext, access); err != nil || !access.Result {
		t.Fatalf("successful public upload stayed private: allowed=%t err=%v", access.Result, err)
	}
	if _, err := dbx.Connection().Exec("UPDATE media_assets SET name='Renamed illustration' WHERE tenant_id=1 AND key=$1", upload.Result.BlobKey); err != nil {
		t.Fatal(err)
	}
	if err := uploadImageFile(ctx, upload); err != nil || stores != 1 {
		t.Fatalf("receipt replay repeated external storage: stores=%d err=%v", stores, err)
	}
	if upload.Result.Name != "Renamed illustration" {
		t.Fatalf("receipt replay reverted the accepted rename: %q", upload.Result.Name)
	}
}
