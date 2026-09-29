package dbx_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func setupMigrationTest(t *testing.T) {
	RegisterT(t)
	ctx := context.Background()

	trx, _ := dbx.BeginTx(ctx)
	_, _ = trx.Execute("DELETE FROM migrations_history WHERE version >= 210001010000")
	_, _ = trx.Execute("DROP TABLE IF EXISTS dummy")
	_, _ = trx.Execute("DROP TABLE IF EXISTS foo")
	trx.MustCommit()
}

func TestMigrationBackfillFailureRollsBackSchemaAndHistory(t *testing.T) {
	setupMigrationTest(t)
	ctx := context.Background()
	wanted := errors.New("backfill unavailable")
	err := dbx.Migrate(ctx, "/app/pkg/dbx/testdata/migration_success", func(ctx context.Context, trx *dbx.Trx, version int) error {
		if _, err := trx.Execute("UPDATE dummy SET description='Partial backfill' WHERE id=100"); err != nil {
			t.Fatal(err)
		}
		return wanted
	})
	if !errors.Is(err, wanted) {
		t.Fatalf("migration lost the data failure: %v", err)
	}

	var schemaExists bool
	if err := dbx.Connection().QueryRow("SELECT to_regclass('dummy') IS NOT NULL").Scan(&schemaExists); err != nil || schemaExists {
		t.Fatalf("failed backfill retained schema: exists=%t err=%v", schemaExists, err)
	}
	var historyCount int
	if err := dbx.Connection().QueryRow("SELECT count(*) FROM migrations_history WHERE version>=210001010000").Scan(&historyCount); err != nil || historyCount != 0 {
		t.Fatalf("failed backfill advanced migration history: count=%d err=%v", historyCount, err)
	}

	err = dbx.Migrate(ctx, "/app/pkg/dbx/testdata/migration_success", func(ctx context.Context, trx *dbx.Trx, version int) error {
		_, err := trx.Execute("UPDATE dummy SET description='Complete backfill' WHERE id=100")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var description string
	if err := dbx.Connection().QueryRow("SELECT description FROM dummy WHERE id=100").Scan(&description); err != nil || description != "Complete backfill" {
		t.Fatalf("migration retry did not finish the same work: description=%q err=%v", description, err)
	}
}

func TestMigrate_Success(t *testing.T) {
	setupMigrationTest(t)
	ctx := context.Background()

	err := dbx.Migrate(ctx, "/app/pkg/dbx/testdata/migration_success", nil)
	Expect(err).IsNil()

	trx, _ := dbx.BeginTx(ctx)
	var value string
	err = trx.Scalar(&value, "SELECT description FROM dummy WHERE id = 200 LIMIT 1")
	Expect(err).IsNil()
	Expect(value).Equals("Description 200Y")

	var count int
	err = trx.Scalar(&count, "SELECT COUNT(*) FROM dummy")
	Expect(err).IsNil()
	Expect(count).Equals(2)
	trx.MustRollback()
}

func TestMigrate_Failure(t *testing.T) {
	setupMigrationTest(t)
	ctx := context.Background()

	trx, _ := dbx.BeginTx(ctx)
	defer trx.MustRollback()

	err := dbx.Migrate(context.Background(), "/app/pkg/dbx/testdata/migration_failure", nil)
	Expect(err).IsNotNil()

	_, err = trx.Execute("SELECT description FROM dummy")
	Expect(err).IsNotNil()
}
