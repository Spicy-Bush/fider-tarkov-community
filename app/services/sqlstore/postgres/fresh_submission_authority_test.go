package postgres_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestTextSubmissionWaitsForCurrentAuthority(t *testing.T) {
	for _, kind := range []string{"post", "comment", "comment-edit"} {
		for _, transition := range []string{"block", "mute", "block accepted retry", "mute accepted retry"} {
			t.Run(kind+"/"+transition, func(t *testing.T) {
				muted := transition == "mute" || transition == "mute accepted retry"
				accepted := transition == "block accepted retry" || transition == "mute accepted retry"

				f := newPostWorkflow(t)
				if err := bus.Dispatch(f.ctx, &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleVisitor}); err != nil {
					t.Fatal(err)
				}
				actor := &query.GetUserByID{UserID: 2}
				if err := bus.Dispatch(f.ctx, actor); err != nil {
					t.Fatal(err)
				}
				post := &cmd.AddNewPost{Title: "Authority transition", Description: "A public discussion"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}

				acting := f
				acting.user = actor.Result
				handler := api.CreatePost()
				body := submissionBody(t, "fresh-authority", false)
				params := web.StringMap{}
				if kind != "post" {
					handler = api.CreateDiscussionComment()
					body = `{"submissionId":"fresh-authority","content":"Text submitted with current authority"}`
					params["number"] = fmt.Sprint(post.Result.Number)
				}
				if kind == "comment-edit" {
					comment := &cmd.CreateComment{PostNumber: post.Result.Number, SubmissionID: "original-comment", Content: "Original comment"}
					if err := bus.Dispatch(withUser(f.ctx, actor.Result), comment); err != nil {
						t.Fatal(err)
					}
					handler = api.EditDiscussionComment()
					params = web.StringMap{"id": fmt.Sprint(comment.Result.ID)}
				}

				request := func() (*httptest.ResponseRecorder, error) {
					return acting.requestWithParams(handler, http.MethodPost, "/api/authority", body, params)
				}
				if accepted {
					response, err := request()
					if err != nil || response.Code != http.StatusOK {
						t.Fatalf("initial submission: status=%d error=%v body=%s", response.Code, err, response.Body)
					}
				}

				postsBefore := workflowCount(t, "SELECT count(*) FROM posts")
				commentsBefore := workflowCount(t, "SELECT count(*) FROM comments")
				receiptsBefore := workflowCount(t, "SELECT count(*) FROM command_receipts")
				locking, err := dbx.BeginTx(f.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer locking.Rollback()
				identity := "command:" + kind + ":1:2:fresh-authority"
				if _, err := locking.Execute("SELECT pg_advisory_xact_lock(hashtextextended($1,0))", identity); err != nil {
					t.Fatal(err)
				}
				var mutation bus.Msg = &cmd.BlockUser{UserID: 2}
				if muted {
					mutation = &cmd.MuteUser{UserID: 2, Reason: "Concurrent submission", ExpiresAt: time.Now().Add(time.Hour)}
				}
				if err := bus.Dispatch(context.WithValue(f.ctx, app.TransactionCtxKey, locking), mutation); err != nil {
					t.Fatal(err)
				}

				type outcome struct {
					response *httptest.ResponseRecorder
					err      error
				}
				completed := make(chan outcome, 1)
				go func() {
					response, err := request()
					completed <- outcome{response, err}
				}()
				deadline := time.Now().Add(5 * time.Second)
				for workflowCount(t, `SELECT count(*) FROM pg_stat_activity
					WHERE datname=current_database() AND wait_event_type='Lock'`) == 0 {
					select {
					case outcome := <-completed:
						t.Fatalf("submission finished without waiting: status=%d error=%v body=%s", outcome.response.Code, outcome.err, outcome.response.Body)
					default:
					}
					if time.Now().After(deadline) {
						t.Fatal("submission did not reach its lock")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := locking.Commit(); err != nil {
					t.Fatal(err)
				}
				result := <-completed
				want := http.StatusForbidden
				if kind == "post" && muted {
					want = http.StatusBadRequest
				}
				if transition == "mute accepted retry" {
					want = http.StatusOK
				}
				if result.err != nil || result.response.Code != want {
					t.Fatalf("revoked submission: status=%d want=%d error=%v body=%s", result.response.Code, want, result.err, result.response.Body)
				}
				if workflowCount(t, "SELECT count(*) FROM posts") != postsBefore || workflowCount(t, "SELECT count(*) FROM comments") != commentsBefore || workflowCount(t, "SELECT count(*) FROM command_receipts") != receiptsBefore {
					t.Fatal("revoked submission left a new effect or receipt")
				}

				if kind == "comment-edit" && !accepted && workflowCount(t, "SELECT count(*) FROM comments WHERE content='Original comment'") != 1 {
					t.Fatal("rejected edit changed the existing comment")
				}

				var recovery bus.Msg = &cmd.UnblockUser{UserID: 2}
				if muted {
					var muteID int
					if err := mediaFixtureScalar(&muteID, "SELECT id FROM user_mutes WHERE user_id=2"); err != nil {
						t.Fatal(err)
					}
					recovery = &cmd.ExpireMute{UserID: 2, MuteID: muteID}
				}
				if err := bus.Dispatch(f.ctx, recovery); err != nil {
					t.Fatal(err)
				}
				response, err := request()
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("same submission did not recover: status=%d error=%v body=%s", response.Code, err, response.Body)
				}
			})
		}
	}
}
