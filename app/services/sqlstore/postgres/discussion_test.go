package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestDiscussionStorageSubmissionAndOwnership(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	post := &cmd.AddNewPost{Title: "Threaded post", Description: "Description"}
	page := &cmd.CreatePage{
		Title:         "Threaded Page",
		Slug:          "threaded-page",
		Content:       "Page content",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(jonSnowCtx, post, page); err != nil {
		t.Fatal(err)
	}

	root := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		Content:      "Root comment",
		SubmissionID: "root-submission",
		BaseURL:      "http://localhost:3000",
	}
	if err := bus.Dispatch(aryaStarkCtx, root); err != nil {
		t.Fatal(err)
	}

	rootID := root.Result.ID
	if !root.Created {
		t.Fatal("new submission was not marked as created")
	}

	if err := bus.Dispatch(aryaStarkCtx, root); err != nil {
		t.Fatal(err)
	}

	if root.Created || root.Result.ID != rootID {
		t.Fatal("retry did not recover the original comment")
	}

	root.Content = "Different content"
	if err := bus.Dispatch(aryaStarkCtx, root); errors.Cause(err) != app.ErrConflict {
		t.Fatalf("changed submission identity must conflict, got %v", err)
	}

	reply := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		ParentID:     &rootID,
		Content:      "Reply",
		SubmissionID: "reply",
	}
	if err := bus.Dispatch(jonSnowCtx, reply); err != nil {
		t.Fatal(err)
	}

	if reply.Result.ParentID == nil || *reply.Result.ParentID != rootID {
		t.Fatal("reply lost its parent")
	}

	wrongOwner := &cmd.CreateComment{
		PageID:       page.Result.ID,
		ParentID:     &rootID,
		Content:      "Wrong owner",
		SubmissionID: "wrong-owner",
	}
	if err := bus.Dispatch(aryaStarkCtx, wrongOwner); err == nil {
		t.Fatal("Page accepted a reply to a post comment")
	}

	if err := bus.Dispatch(tonyStarkCtx, &query.GetDiscussion{CommentID: rootID}); errors.Cause(err) != app.ErrNotFound {
		t.Fatalf("another tenant resolved the comment owner: %v", err)
	}

	if err := bus.Dispatch(aryaStarkCtx, &cmd.DeleteComment{CommentID: rootID}); err != nil {
		t.Fatal(err)
	}

	owner := &query.GetDiscussion{PostNumber: post.Result.Number}
	if err := bus.Dispatch(aryaStarkCtx, owner); err != nil {
		t.Fatal(err)
	}

	roots := &query.GetDiscussionComments{Discussion: owner.Result}
	if err := bus.Dispatch(aryaStarkCtx, roots); err != nil {
		t.Fatal(err)
	}

	if len(roots.Result) != 1 || !roots.Result[0].Deleted || !roots.Result[0].HasReplies {
		t.Fatalf("deleted ancestor became unreachable: %+v", roots.Result)
	}

	children := &query.GetDiscussionComments{Discussion: owner.Result, ParentID: &rootID}
	if err := bus.Dispatch(aryaStarkCtx, children); err != nil {
		t.Fatal(err)
	}

	if len(children.Result) != 1 || children.Result[0].ID != reply.Result.ID {
		t.Fatal("deleting a parent lost its reply")
	}
}

func TestDiscussionStoragePrivatePageAndModeration(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	page := &cmd.CreatePage{
		Title:         "Private discussion",
		Slug:          "private-discussion",
		Content:       "Page content",
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPrivate,
		AllowedRoles:  []string{"visitor"},
		AllowComments: true,
	}
	if err := bus.Dispatch(jonSnowCtx, page); err != nil {
		t.Fatal(err)
	}

	visitor := *aryaStark
	visitor.Role = enum.RoleVisitor
	visitorCtx := context.WithValue(aryaStarkCtx, app.UserCtxKey, &visitor)
	comment := &cmd.CreateComment{
		PageID:       page.Result.ID,
		Content:      "Private comment",
		SubmissionID: "private-comment",
	}
	if err := bus.Dispatch(visitorCtx, comment); err != nil {
		t.Fatal(err)
	}

	moderator := *sansaStark
	moderator.Role = enum.RoleModerator
	moderatorCtx := context.WithValue(sansaStarkCtx, app.UserCtxKey, &moderator)
	operations := []any{
		&query.GetDiscussion{CommentID: comment.Result.ID},
		&query.GetCommentAncestors{CommentID: comment.Result.ID, Discussion: entity.PageDiscussion(page.Result)},
		&cmd.UpdateComment{
			CommentID:    comment.Result.ID,
			Content:      "Forbidden edit",
			SubmissionID: "forbidden-edit",
		},
		&cmd.DeleteComment{CommentID: comment.Result.ID},
		&cmd.SetModerationPending{ContentType: "comment", ContentID: comment.Result.ID, Pending: true},
	}
	for _, operation := range operations {
		if err := bus.Dispatch(moderatorCtx, operation); errors.Cause(err) != app.ErrNotFound {
			t.Fatalf("private Page operation %T did not deny moderator: %v", operation, err)
		}
	}

	if _, err := trx.Execute("UPDATE pages SET allowed_roles = '[\"visitor\", \"moderator\"]' WHERE id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(moderatorCtx, &cmd.SetModerationPending{ContentType: "comment", ContentID: comment.Result.ID, Pending: true}); err != nil {
		t.Fatal(err)
	}

	stored := &query.GetCommentByID{CommentID: comment.Result.ID}
	if err := bus.Dispatch(jonSnowCtx, stored); err != nil {
		t.Fatal(err)
	}

	if !stored.Result.ModerationPending || stored.Result.Content != "Private comment" {
		t.Fatal("moderation did not preserve the stored body")
	}
}

func TestDiscussionStorageCursorSurvivesDeletion(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	post := &cmd.AddNewPost{Title: "Paginated comments", Description: "Description"}
	if err := bus.Dispatch(jonSnowCtx, post); err != nil {
		t.Fatal(err)
	}

	if _, err := trx.Execute(`
        INSERT INTO comments (tenant_id, post_id, user_id, content, created_at)
        SELECT $1, $2, $3, 'comment ' || n, $4::timestamptz
        FROM generate_series(1, 60) n
    `, demoTenant.ID, post.Result.ID, jonSnow.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	owner := entity.PostDiscussion(post.Result)
	first := &query.GetDiscussionComments{Discussion: owner}
	if err := bus.Dispatch(jonSnowCtx, first); err != nil {
		t.Fatal(err)
	}

	if len(first.Result) != 26 {
		t.Fatalf("expected 25 comments and a continuation row, got %d", len(first.Result))
	}

	anchor := first.Result[24]
	nextID := first.Result[25].ID
	if err := bus.Dispatch(jonSnowCtx, &cmd.DeleteComment{CommentID: anchor.ID}); err != nil {
		t.Fatal(err)
	}

	second := &query.GetDiscussionComments{Discussion: owner, After: anchor.CreatedAt, AfterID: anchor.ID}
	if err := bus.Dispatch(jonSnowCtx, second); err != nil {
		t.Fatal(err)
	}

	if len(second.Result) != 26 || second.Result[0].ID != nextID {
		t.Fatal("deleting the cursor anchor skipped the next comment")
	}
}
