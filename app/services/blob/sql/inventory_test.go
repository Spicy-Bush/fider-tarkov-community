package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestSQLInventoryCommitsReadBeforeAcceptingBatch(t *testing.T) {
	keys := []string{"inventory-probe-a", "inventory-probe-b", "inventory-probe-c"}
	for _, key := range keys {
		_, err := dbx.Connection().Exec(`
			INSERT INTO blobs (tenant_id,key,content_type,size,file,created_at,modified_at)
			VALUES (1,$1,'image/png',50000000,'fixture',NOW(),NOW())
		`, key)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := dbx.Connection().Exec("DELETE FROM blobs WHERE tenant_id=1 AND key=$1", key); err != nil {
				t.Error(err)
			}
		})
	}
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	stop := errors.New("batch accepted")
	request := &query.ScanBlobMetadata{Cursor: keys[0], BatchSize: 2}
	request.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
		if dbx.Connection().Stats().InUse != 0 {
			t.Error("provider read retained a transaction during catalog acceptance")
		}
		if len(files) != 2 || files[0].Key != keys[1] || files[1].Key != keys[2] || next != keys[2] || complete {
			t.Fatalf("unexpected keyset page: %+v cursor=%q complete=%t", files, next, complete)
		}
		if files[0].Size != 50000000 || files[0].ContentType != "image/png" {
			t.Fatalf("wrong stored metadata: %+v", files[0])
		}
		return stop
	}
	if err := backend.Scan(ctx, request); !errors.Is(err, stop) {
		t.Fatal(err)
	}
}

func TestSQLInventoryAdvancesPastInvalidStoredNames(t *testing.T) {
	const invalid = "inventory-invalid name"
	const valid = "inventory-valid-name"
	for _, key := range []string{invalid, valid} {
		if _, err := dbx.Connection().Exec(`
			INSERT INTO blobs (tenant_id,key,content_type,size,file,created_at,modified_at)
			VALUES (1,$1,'image/png',7,'fixture',NOW(),NOW())
		`, key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := dbx.Connection().Exec("DELETE FROM blobs WHERE tenant_id=1 AND key=$1", key); err != nil {
				t.Error(err)
			}
		})
	}

	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	seen, complete := false, false
	scan := &query.ScanBlobMetadata{Cursor: "inventory-", BatchSize: 1}
	scan.Accept = func(files []dto.BlobMetadata, next string, done bool) error {
		for _, file := range files {
			if file.Key == invalid {
				t.Fatal("invalid file reached catalog")
			}
			seen = seen || file.Key == valid
		}
		complete = done
		return nil
	}
	if err := backend.Scan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	if !seen || scan.Skipped != 1 || !complete {
		t.Fatalf("seen=%t skipped=%d complete=%t", seen, scan.Skipped, complete)
	}
}
