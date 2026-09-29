package postgres

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func exportSelectionFixture(t testing.TB, count int) *dbx.Trx {
	t.Helper()
	trx, err := dbx.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = trx.Rollback() })

	for _, statement := range []string{
		"SET LOCAL statement_timeout = '45s'",
		"ALTER TABLE posts DISABLE TRIGGER USER",
		"UPDATE posts SET status = 6 WHERE tenant_id = 1",
	} {
		if _, err := trx.Execute(statement); err != nil {
			t.Fatal(err)
		}
	}

	_, err = trx.Execute(`
		INSERT INTO posts (
			tenant_id, title, slug, description, number, status, user_id,
			created_at, upvotes, downvotes, comments_count, moderation_pending
		)
		SELECT 1, 'Export selection ' || g, 'export-selection-' || g, 'Fixture',
			1000 + g, g % 6, 1, NOW() - make_interval(days => g % 100),
			g % 1000, g % 200, g % 50, g % 101 = 0
		FROM generate_series(1, $1::integer) g
	`, count)
	if err != nil {
		t.Fatal(err)
	}

	for _, statement := range []string{"ALTER TABLE posts ENABLE TRIGGER USER", "ANALYZE posts"} {
		if _, err := trx.Execute(statement); err != nil {
			t.Fatal(err)
		}
	}
	return trx
}
