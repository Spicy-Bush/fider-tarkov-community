package postgres

import (
	"context"
	"net/url"
	"slices"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestPostRankingUsesCommittedSnapshotAndOwningTransaction(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	fixture := exportSelectionFixture(t, 100)
	if err := fixture.Commit(); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	ctx = context.WithValue(ctx, app.RequestCtxKey, web.Request{URL: &url.URL{Scheme: "http", Host: "localhost:3000"}})
	read := func(ctx context.Context) []int {
		t.Helper()
		posts := &query.SearchPosts{View: "trending", Limit: "15"}
		if err := searchPosts(ctx, posts); err != nil {
			t.Fatal(err)
		}
		ids := make([]int, len(posts.Result))
		for index, post := range posts.Result {
			ids[index] = post.ID
		}
		return ids
	}

	initial := read(ctx)
	key := postRankingKey{TenantID: 1, View: "trending"}
	stale, found := postRankings.Load(key)
	if !found {
		t.Fatal("initial ranking was not cached")
	}
	if got := read(ctx); !slices.Equal(got, initial) {
		t.Fatalf("unchanged snapshot reordered posts: %v", got)
	}

	writer, err := dbx.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	var promotedID int
	if err := writer.Scalar(&promotedID, "UPDATE posts SET upvotes = 1000000 WHERE tenant_id = 1 AND number = 1001 RETURNING id"); err != nil {
		t.Fatal(err)
	}
	if got := read(ctx); !slices.Equal(got, initial) {
		t.Fatalf("uncommitted vote changed another reader: %v", got)
	}
	writerCtx := context.WithValue(ctx, app.TransactionCtxKey, writer)
	if got := read(writerCtx); got[0] != promotedID {
		t.Fatalf("owning transaction reused committed data: %v", got)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := read(ctx); got[0] != promotedID {
		t.Fatalf("committed vote did not update the ranking: %v", got)
	}

	postRankings.Store(key, stale)
	if got := read(ctx); got[0] != promotedID {
		t.Fatalf("late cache fill restored an older ranking: %v", got)
	}
	if _, err := dbx.Connection().Exec("UPDATE posts SET status = 2 WHERE id = $1", promotedID); err != nil {
		t.Fatal(err)
	}
	if got := read(ctx); slices.Contains(got, promotedID) {
		t.Fatalf("completed post remained in the trending view: %v", got)
	}

	rolledBack, err := dbx.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rolledBack.Rollback()
	if _, err := rolledBack.Execute("UPDATE posts SET status = 0 WHERE id = $1", promotedID); err != nil {
		t.Fatal(err)
	}
	if got := read(context.WithValue(ctx, app.TransactionCtxKey, rolledBack)); got[0] != promotedID {
		t.Fatalf("pending status change was lost: %v", got)
	}
	if err := rolledBack.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := read(ctx); slices.Contains(got, promotedID) {
		t.Fatalf("rolled-back status change reached the cache: %v", got)
	}
}
