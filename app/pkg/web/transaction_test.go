package web_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/worker"
)

func TestTransactionResponseCommitFailure(t *testing.T) {
	table := fmt.Sprintf("transaction_response_%d", time.Now().UnixNano())
	_, err := dbx.Connection().Exec("CREATE TABLE " + table + " (id INTEGER UNIQUE DEFERRABLE INITIALLY DEFERRED)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbx.Connection().Exec("DROP TABLE " + table) })

	recorder := httptest.NewRecorder()
	c, err := web.NewContext(web.New(), httptest.NewRequest(http.MethodPost, "/", nil), recorder, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = c.WithTransaction(func() error {
		trx := c.Value(app.TransactionCtxKey).(*dbx.Trx)
		if _, err := trx.Execute("INSERT INTO " + table + " VALUES (1), (1)"); err != nil {
			return err
		}
		c.Response.Header().Set("Set-Cookie", "uncommitted=value")
		c.Enqueue(worker.Task{Name: "must not escape"})
		return c.Ok(web.Map{"saved": true})
	})
	if err == nil || recorder.Body.Len() != 0 || recorder.Header().Get("Set-Cookie") != "" {
		t.Fatalf("commit failure leaked response: error=%v body=%s headers=%v", err, recorder.Body, recorder.Header())
	}
	if c.Value(app.TransactionCtxKey) != nil {
		t.Fatal("finished transaction retained on context")
	}
	if err := c.Commit(); err != nil || c.Engine().Worker().Length() != 0 {
		t.Fatalf("failed transaction leaked task: %v", err)
	}
	var count int
	if err := dbx.Connection().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed transaction retained rows: count=%d error=%v", count, err)
	}
	err = c.WithTransaction(func() error {
		trx := c.Value(app.TransactionCtxKey).(*dbx.Trx)
		_, err := trx.Execute("INSERT INTO " + table + " VALUES (2)")
		return err
	})
	if err != nil {
		t.Fatalf("fresh operation failed after rollback: %v", err)
	}
}

func TestTransactionNestedAndPanicCleanup(t *testing.T) {
	c := newGetContext("http://localhost/", nil)
	primary := errors.New("operation failed")
	err := c.WithTransaction(func() error {
		outer := c.Value(app.TransactionCtxKey)
		return c.WithTransaction(func() error {
			if c.Value(app.TransactionCtxKey) != outer {
				t.Fatal("nested operation replaced owner")
			}
			return primary
		})
	})
	if !errors.Is(err, primary) {
		t.Fatalf("primary error lost: %v", err)
	}
	func() {
		defer func() {
			if recover() != primary {
				t.Error("panic cause changed")
			}
		}()
		_ = c.WithTransaction(func() error { panic(primary) })
	}()
	if c.Value(app.TransactionCtxKey) != nil {
		t.Fatal("panic retained transaction")
	}
	if err := dbx.InTransaction(context.Background(), func(ctx context.Context, trx *dbx.Trx) error {
		return dbx.InTransaction(ctx, func(_ context.Context, nested *dbx.Trx) error {
			if nested != trx {
				t.Fatal("storage operation escaped transaction")
			}
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
}