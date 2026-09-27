package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestDiscussionEditTimesRemainOrderedAcrossTransactions(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Ordered comment edits", Description: "Transaction ordering"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	create := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		Content:      "Initial content",
		SubmissionID: "initial-content",
	}
	if err := bus.Dispatch(f.ctx, create); err != nil {
		t.Fatal(err)
	}

	older, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer older.Rollback()

	var started time.Time
	if err := older.Scalar(&started, "SELECT CURRENT_TIMESTAMP"); err != nil {
		t.Fatal(err)
	}

	newer := &cmd.UpdateComment{
		CommentID:    create.Result.ID,
		Content:      "Newer transaction",
		SubmissionID: "newer-edit",
	}
	if err := bus.Dispatch(f.ctx, newer); err != nil {
		t.Fatal(err)
	}

	oldContext := context.WithValue(f.ctx, app.TransactionCtxKey, older)
	later := &cmd.UpdateComment{
		CommentID:    create.Result.ID,
		Content:      "Older transaction edits afterward",
		SubmissionID: "later-edit",
	}
	if err := bus.Dispatch(oldContext, later); err != nil {
		t.Fatal(err)
	}

	if !newer.Result.EditedAt.After(started) || !later.Result.EditedAt.After(*newer.Result.EditedAt) {
		t.Fatalf("edit times went backward: started=%s newer=%s later=%s", started, newer.Result.EditedAt, later.Result.EditedAt)
	}

	if err := older.Commit(); err != nil {
		t.Fatal(err)
	}

	future := time.Date(2030, 1, 1, 0, 0, 0, 123456000, time.UTC)
	if _, err := dbx.Connection().Exec("UPDATE comments SET edited_at = $1 WHERE id = $2", future, create.Result.ID); err != nil {
		t.Fatal(err)
	}

	afterFuture := &cmd.UpdateComment{
		CommentID:    create.Result.ID,
		Content:      "After a future timestamp",
		SubmissionID: "edit-after-future",
	}
	if err := bus.Dispatch(f.ctx, afterFuture); err != nil {
		t.Fatal(err)
	}

	if afterFuture.Result.EditedAt.Sub(future) != time.Microsecond {
		t.Fatalf("future edit time did not advance by one microsecond: %s", afterFuture.Result.EditedAt)
	}
}
