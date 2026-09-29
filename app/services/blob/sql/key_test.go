package sql

import (
	"context"
	"reflect"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestInvalidKeysDoNotReachSQL(t *testing.T) {
	for _, key := range []string{"../escape", "a//b", "a/../b", "a\\b", "a\x00b", "/absolute"} {
		if err := backend.Get(context.Background(), &query.GetBlobByKey{Key: key}); err == nil {
			t.Errorf("invalid read accepted: %q", key)
		}
		if err := backend.Store(context.Background(), &cmd.StoreBlob{Key: key}); err == nil {
			t.Errorf("invalid write accepted: %q", key)
		}
		if err := backend.Delete(context.Background(), &cmd.DeleteBlob{Key: key}); err == nil {
			t.Errorf("invalid delete accepted: %q", key)
		}
	}
}

func TestSQLBlobPrefixesAreLiteral(t *testing.T) {
	ctx := context.Background()
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.MustRollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)

	for _, key := range []string{"prefix-probe%_literal", "prefix-probeZZliteral"} {
		if err := backend.Store(ctx, &cmd.StoreBlob{Key: key, Content: []byte("fixture")}); err != nil {
			t.Fatal(err)
		}
	}

	request := &query.ListBlobs{Prefix: "prefix-probe%_"}
	if err := backend.List(ctx, request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request.Result, []string{"prefix-probe%_literal"}) {
		t.Fatalf("literal prefix matched unrelated keys: %v", request.Result)
	}
}
