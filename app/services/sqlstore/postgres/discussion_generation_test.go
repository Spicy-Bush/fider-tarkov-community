package postgres_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestDiscussionTreeGenerationRollbackAndInvalidation(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Versioned discussion", Description: "Committed structural identities"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	roots := make([]int, 2)
	for index := range roots {
		comment := &cmd.CreateComment{
			PostNumber:   post.Result.Number,
			Content:      fmt.Sprintf("Root %d", index),
			SubmissionID: fmt.Sprintf("root-%d", index),
		}
		if err := bus.Dispatch(f.ctx, comment); err != nil {
			t.Fatal(err)
		}

		roots[index] = comment.Result.ID
	}

	owner := &query.GetDiscussion{PostNumber: post.Result.Number}
	if err := bus.Dispatch(f.ctx, owner); err != nil {
		t.Fatal(err)
	}

	rank := func(ctx context.Context, expectedID, expectedScore int) {
		t.Helper()
		comments := &query.GetDiscussionComments{Discussion: owner.Result, Sort: "replies"}
		if err := bus.Dispatch(ctx, comments); err != nil {
			t.Fatal(err)
		}

		if len(comments.Result) != 2 || comments.Result[0].ID != expectedID || comments.Result[0].SortScore != expectedScore {
			t.Fatalf("wrong first root: %+v", comments.Result)
		}
	}

	path := fmt.Sprintf("/api/posts/%d/comments?sort=replies", post.Result.Number)
	params := web.StringMap{"number": fmt.Sprint(post.Result.Number)}
	first, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
	if err != nil {
		t.Fatal(err)
	}

	second, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
	if err != nil || first.Code != http.StatusOK || second.Code != http.StatusOK || first.Body.String() != second.Body.String() {
		t.Fatal("warming the structural cache changed the complete HTTP response")
	}

	rank(f.ctx, roots[1], 0)

	transaction, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()

	transactionContext := context.WithValue(f.ctx, app.TransactionCtxKey, transaction)
	tentative := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		ParentID:     &roots[0],
		Content:      "Rolled back child",
		SubmissionID: "rolled-back-child",
	}
	if err := bus.Dispatch(transactionContext, tentative); err != nil {
		t.Fatal(err)
	}

	rank(transactionContext, roots[0], 1)

	if err := transaction.Rollback(); err != nil {
		t.Fatal(err)
	}

	committed := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		ParentID:     &roots[1],
		Content:      "Committed child",
		SubmissionID: "committed-child",
	}
	if err := bus.Dispatch(f.ctx, committed); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[1], 1)

	if err := bus.Dispatch(f.ctx, &cmd.DeleteComment{CommentID: committed.Result.ID}); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[1], 0)

	if _, err := mediaFixtureSQL("UPDATE comments SET deleted_at = NULL WHERE id = $1", committed.Result.ID); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[1], 1)

	if _, err := dbx.Connection().Exec("DELETE FROM comments WHERE id = $1", committed.Result.ID); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[1], 0)

	if _, err := mediaFixtureSQL("UPDATE comments SET created_at = NOW() WHERE id = $1", roots[0]); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[0], 0)

	if _, err := dbx.Connection().Exec("DELETE FROM discussion_tree_versions WHERE post_id = $1", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := mediaFixtureSQL("UPDATE comments SET content = 'Unversioned current body' WHERE id = $1", roots[0]); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[0], 0)

	repair := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		ParentID:     &roots[1],
		Content:      "Recreate version metadata",
		SubmissionID: "repair-metadata",
	}
	if err := bus.Dispatch(f.ctx, repair); err != nil {
		t.Fatal(err)
	}

	if _, err := dbx.Connection().Exec("DELETE FROM discussion_tree_versions WHERE post_id = $1", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[1], 1)

	if err := bus.Dispatch(f.ctx, &cmd.DeleteComment{CommentID: repair.Result.ID}); err != nil {
		t.Fatal(err)
	}

	rank(f.ctx, roots[0], 0)

	var before, after int64
	versionSQL := "SELECT generation FROM discussion_tree_versions WHERE post_id = $1"
	if err := dbx.Connection().QueryRow(versionSQL, post.Result.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx,
		&cmd.UpdateComment{
			CommentID:    roots[0],
			Content:      "Live body after warming",
			SubmissionID: "edit-after-warming",
		},
		&cmd.SetCommentReaction{CommentID: roots[0], Emoji: "👍", Active: true},
		&cmd.SetModerationPending{ContentType: "comment", ContentID: roots[0], Pending: true},
	); err != nil {
		t.Fatal(err)
	}

	if err := dbx.Connection().QueryRow(versionSQL, post.Result.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}

	comments := &query.GetDiscussionComments{Discussion: owner.Result, Sort: "replies"}
	if err := bus.Dispatch(f.ctx, comments); err != nil {
		t.Fatal(err)
	}

	changed := comments.Result[0]
	if before != after || changed.Content != "Live body after warming" || len(changed.ReactionCounts) != 1 || !changed.ModerationPending {
		t.Fatal("nonstructural changes were cached or changed the tree generation")
	}
}
