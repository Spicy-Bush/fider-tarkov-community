package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
)

func TestSQLReadLimitChecksStoredAndActualSize(t *testing.T) {
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.MustRollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)
	if _, err := trx.Execute(`
		INSERT INTO blobs (tenant_id,key,content_type,size,file,created_at,modified_at)
		VALUES (1,'limit-understated','image/png',1,decode(repeat('aa',4096),'hex'),NOW(),NOW()),
		       (1,'limit-overstated','image/png',4096,'small',NOW(),NOW()),
		       (1,'limit-exact','image/png',64,decode(repeat('aa',64),'hex'),NOW(),NOW())
	`); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"limit-understated", "limit-overstated"} {
		request := &query.GetBlobByKey{Key: key, MaxBytes: 64}
		if err := backend.Get(ctx, request); !errors.Is(err, readlimit.ErrTooLarge) || request.Result != nil {
			t.Fatalf("%s bypassed size bound: err=%v", key, err)
		}
	}
	request := &query.GetBlobByKey{Key: "limit-exact", MaxBytes: 64}
	if err := backend.Get(ctx, request); err != nil || len(request.Result.Content) != 64 {
		t.Fatalf("exact SQL read boundary failed: err=%v", err)
	}
	request = &query.GetBlobByKey{Key: "limit-understated"}
	if err := backend.Get(ctx, request); err != nil || len(request.Result.Content) != 4096 {
		t.Fatalf("unbounded backup contract changed: err=%v", err)
	}
}
