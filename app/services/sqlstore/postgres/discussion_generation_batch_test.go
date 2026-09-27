package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestDiscussionTreeBatchInvalidation(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Batch post", Description: "Structural changes across discussion owners"}
	page := &cmd.CreatePage{
		Title:         "Batch Page",
		Slug:          "batch-page",
		Content:       "Page content",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(f.ctx, post, page); err != nil {
		t.Fatal(err)
	}

	postOwner := &query.GetDiscussion{PostNumber: post.Result.Number}
	pageOwner := &query.GetDiscussion{PageID: page.Result.ID}
	if err := bus.Dispatch(f.ctx, postOwner, pageOwner); err != nil {
		t.Fatal(err)
	}

	transaction, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()

	ctx := context.WithValue(f.ctx, app.TransactionCtxKey, transaction)
	_, err = transaction.Execute(`
		WITH owners(post_id, page_id) AS (VALUES ($1::integer, NULL::integer), (NULL::integer, $2::integer)),
		roots AS (
			INSERT INTO comments (tenant_id, user_id, post_id, page_id, content, created_at)
			SELECT 1, 1, post_id, page_id, 'Root ' || n, NOW() + n * INTERVAL '1 second'
			FROM owners CROSS JOIN generate_series(1, 2) n
			RETURNING id, tenant_id, user_id, post_id, page_id, content
		)
		INSERT INTO comments (tenant_id, user_id, post_id, page_id, parent_id, content, created_at)
		SELECT tenant_id, user_id, post_id, page_id, id, 'Child', NOW() FROM roots WHERE content = 'Root 1'
	`, post.Result.ID, page.Result.ID)
	if err != nil {
		t.Fatal(err)
	}

	owners := []*entity.Discussion{postOwner.Result, pageOwner.Result}
	assertRanks := func(readContext context.Context, score int) {
		t.Helper()

		for _, owner := range owners {
			comments := &query.GetDiscussionComments{Discussion: owner, Sort: "replies"}
			if err := bus.Dispatch(readContext, comments); err != nil {
				t.Fatal(err)
			}

			if len(comments.Result) != 2 || comments.Result[0].SortScore != score {
				t.Fatalf("%s ranked a stale structural state: %+v", owner.Owner.Kind, comments.Result)
			}
		}
	}

	assertRanks(ctx, 1)
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}

	versions := func() map[string]int64 {
		t.Helper()

		result := make(map[string]int64)
		for _, owner := range owners {
			var generation int64
			err := dbx.Connection().QueryRow(`
				SELECT generation FROM discussion_tree_versions
				WHERE tenant_id = $1 AND (($2 = 'post' AND post_id = $3) OR ($2 = 'page' AND page_id = $3))
			`, f.tenant.ID, owner.Owner.Kind, owner.Owner.ID).Scan(&generation)
			if err != nil {
				t.Fatal(err)
			}

			result[owner.Owner.Kind] = generation
		}

		return result
	}

	before := versions()
	if _, err := dbx.Connection().Exec("UPDATE comments SET content = content || ' edited', moderation_pending = TRUE"); err != nil {
		t.Fatal(err)
	}

	for owner, generation := range versions() {
		if generation != before[owner] {
			t.Fatalf("%s nonstructural update changed generation", owner)
		}
	}

	for _, mutation := range []struct {
		sql   string
		score int
	}{
		{"UPDATE comments SET deleted_at = NOW() WHERE parent_id IS NOT NULL", 0},
		{"UPDATE comments SET deleted_at = NULL WHERE parent_id IS NOT NULL", 1},
		{"UPDATE comments SET created_at = created_at + INTERVAL '1 minute'", 1},
	} {
		before = versions()
		if _, err := dbx.Connection().Exec(mutation.sql); err != nil {
			t.Fatal(err)
		}

		for owner, generation := range versions() {
			if generation <= before[owner] {
				t.Fatalf("%s structural update retained generation", owner)
			}
		}

		assertRanks(f.ctx, mutation.score)
	}

	before = versions()
	if _, err := dbx.Connection().Exec("DELETE FROM comments WHERE post_id = $1 AND parent_id IS NULL", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	if versions()["post"] <= before["post"] {
		t.Fatal("post subtree deletion retained its generation")
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM comments WHERE post_id = $1", post.Result.ID); count != 0 {
		t.Fatal("post subtree deletion retained descendants")
	}

	if _, err := dbx.Connection().Exec("DELETE FROM pages WHERE id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM discussion_tree_versions WHERE page_id = $1", page.Result.ID); count != 0 {
		t.Fatal("Page cascade recreated its tree version")
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM comments"); count != 0 {
		t.Fatal("owner cascades retained descendants")
	}

	if _, err := dbx.Connection().Exec("DELETE FROM post_subscribers WHERE post_id = $1", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := dbx.Connection().Exec("DELETE FROM posts WHERE id = $1", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM discussion_tree_versions"); count != 0 {
		t.Fatal("owner cleanup retained tree versions")
	}
}

func TestDiscussionTreeConcurrentVersions(t *testing.T) {
	f := newPostWorkflow(t)
	page := &cmd.CreatePage{
		Title:         "Concurrent Page",
		Slug:          "concurrent-page",
		Content:       "Concurrent structural changes",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	first, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback()

	insert := `INSERT INTO comments (tenant_id, user_id, page_id, content, created_at) VALUES (1, 1, $1, 'Concurrent root', NOW())`
	if _, err := first.Execute(insert, page.Result.ID); err != nil {
		t.Fatal(err)
	}

	var firstGeneration int64
	if err := first.Scalar(&firstGeneration, "SELECT generation FROM discussion_tree_versions WHERE page_id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}

	second, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		second.Rollback()
	}()

	var secondPID int
	if err := second.Scalar(&secondPID, "SELECT pg_backend_pid()"); err != nil {
		t.Fatal(err)
	}

	completed := make(chan error, 1)
	go func() {
		_, err := second.Execute(insert, page.Result.ID)
		completed <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, "SELECT COUNT(*) FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock'", secondPID) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("concurrent owner mutation did not wait for its version: %v", err)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("concurrent mutation did not reach the version lock")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-completed; err != nil {
		t.Fatal(err)
	}

	var secondGeneration int64
	if err := second.Scalar(&secondGeneration, "SELECT generation FROM discussion_tree_versions WHERE page_id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}

	if secondGeneration <= firstGeneration {
		t.Fatal("concurrent writers reused a structural identity")
	}

	if err := second.Commit(); err != nil {
		t.Fatal(err)
	}

	owner := &query.GetDiscussion{PageID: page.Result.ID}
	if err := bus.Dispatch(f.ctx, owner); err != nil {
		t.Fatal(err)
	}

	comments := &query.GetDiscussionComments{Discussion: owner.Result, Sort: "replies"}
	if err := bus.Dispatch(f.ctx, comments); err != nil {
		t.Fatal(err)
	}

	if len(comments.Result) != 2 {
		t.Fatal("the committed generation omitted a concurrent writer")
	}
}
