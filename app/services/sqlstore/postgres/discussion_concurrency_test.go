package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestDiscussionCreationObservesConcurrentLock(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Lock boundary", Description: "A lock and comment racing"}
	visitor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, post, visitor); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()
	visitorCtx := context.WithValue(ctx, app.UserCtxKey, visitor.Result)

	locking, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer locking.Rollback()
	lockCtx := context.WithValue(ctx, app.TransactionCtxKey, locking)
	if err := bus.Dispatch(lockCtx, &cmd.LockPost{Post: post.Result, LockMessage: "Discussion closed"}); err != nil {
		t.Fatal(err)
	}

	create := &cmd.CreateComment{
		PostNumber:   post.Result.Number,
		Content:      "Started before the lock committed",
		SubmissionID: "concurrent-lock",
		BaseURL:      "http://localhost:3000",
	}
	completed := make(chan error, 1)
	go func() { completed <- bus.Dispatch(visitorCtx, create) }()

	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `
		SELECT COUNT(*) FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock'
		  AND query LIKE '%FROM posts WHERE tenant_id%FOR NO KEY UPDATE'
	`) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("submission did not wait for the lock decision: %v", err)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("submission did not reach the post lock boundary")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err := locking.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-completed; err == nil || create.Created {
		t.Fatalf("comment passed a committed lock: created=%v error=%v", create.Created, err)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM comments"); count != 0 {
		t.Fatalf("locked submission persisted %d comments", count)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM notification_deliveries"); count != 0 {
		t.Fatalf("locked submission scheduled %d notifications", count)
	}

	if err := bus.Dispatch(f.ctx, &cmd.UnlockPost{Post: post.Result}); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(visitorCtx, create); err != nil || !create.Created {
		t.Fatalf("unlock did not recover the draft: created=%v error=%v", create.Created, err)
	}

	commentID := create.Result.ID
	if err := bus.Dispatch(f.ctx, &cmd.LockPost{Post: post.Result}); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(visitorCtx, create); err != nil || create.Created || create.Result.ID != commentID {
		t.Fatalf("accepted submission receipt did not recover after locking: created=%v error=%v", create.Created, err)
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM comments"); count != 1 {
		t.Fatalf("receipt replay inserted a new comment after locking: %d", count)
	}
}

func TestDiscussionConcurrentArchivedCreation(t *testing.T) {
	for _, sameID := range []bool{true, false} {
		t.Run(fmt.Sprintf("same submission=%v", sameID), func(t *testing.T) {
			f := newPostWorkflow(t)
			post := &cmd.AddNewPost{Title: "Archived discussion", Description: "Concurrent replies"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}
			if _, err := dbx.Connection().Exec("UPDATE posts SET status = $1, archived_from_status = $2 WHERE id = $3", enum.PostArchived, enum.PostOpen, post.Result.ID); err != nil {
				t.Fatal(err)
			}

			_, err := dbx.Connection().Exec(`
                CREATE FUNCTION discussion_test_pause_insert() RETURNS trigger AS $$
                BEGIN
                    PERFORM pg_advisory_xact_lock(726192845);
                    RETURN NEW;
                END;
                $$ LANGUAGE plpgsql;
                CREATE TRIGGER discussion_test_pause_insert
                BEFORE INSERT ON comments FOR EACH ROW
                EXECUTE FUNCTION discussion_test_pause_insert();
            `)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_, err := dbx.Connection().Exec(`
                    DROP TRIGGER discussion_test_pause_insert ON comments;
                    DROP FUNCTION discussion_test_pause_insert();
                `)
				if err != nil {
					t.Error(err)
				}
			}()

			blocker, err := dbx.Connection().Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			if _, err := blocker.Exec("SELECT pg_advisory_xact_lock(726192845)"); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			type result struct {
				command *cmd.CreateComment
				err error
			}
			completed := make(chan result, 2)
			for index := 0; index < 2; index++ {
				identity := "archived-comment"
				if !sameID {
					identity = fmt.Sprintf("archived-comment-%d", index)
				}
				command := &cmd.CreateComment{
					PostNumber: post.Result.Number,
					Content: "A new reply",
					SubmissionID: identity,
				}
				go func() {
					completed <- result{command: command, err: bus.Dispatch(ctx, command)}
				}()
			}

			deadline := time.Now().Add(5 * time.Second)
			for {
				waiting := workflowCount(t, `
                    SELECT COUNT(*) FROM pg_stat_activity
                    WHERE datname = current_database() AND wait_event_type = 'Lock'
                      AND (query LIKE '%INSERT INTO comments%'
                           OR query LIKE '%SELECT pg_advisory_xact_lock(hashtextextended%'
                           OR query LIKE '%FROM posts WHERE tenant_id%FOR NO KEY UPDATE')
                `)
				if waiting >= 2 {
					break
				}
				if time.Now().After(deadline) {
					t.Error("both submissions did not reach the controlled lock boundary")
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			if err := blocker.Commit(); err != nil {
				t.Fatal(err)
			}
			ids := make(map[int]bool)
			created := 0
			for index := 0; index < 2; index++ {
				output := <-completed
				if output.err != nil {
					t.Errorf("concurrent creation failed: %v", output.err)
					continue
				}
				ids[output.command.Result.ID] = true
				if output.command.Created {
					created++
				}
			}

			want := 2
			if sameID {
				want = 1
			}
			if len(ids) != want || created != want {
				t.Errorf("received %d distinct receipts and %d creations, want %d", len(ids), created, want)
			}
			var count int
			if err := dbx.Connection().QueryRow("SELECT COUNT(*) FROM comments WHERE post_id = $1", post.Result.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != want {
				t.Errorf("persisted %d comments, want %d", count, want)
			}
			stored := &query.GetPostByNumber{Number: post.Result.Number}
			if err := bus.Dispatch(f.ctx, stored); err != nil {
				t.Fatal(err)
			}
			if stored.Result.Status != enum.PostOpen {
				t.Errorf("post did not reopen: %v", stored.Result.Status)
			}
		})
	}
}
