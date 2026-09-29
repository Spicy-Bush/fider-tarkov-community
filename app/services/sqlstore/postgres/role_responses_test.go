package postgres_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestRoleResponsesSaveReplayAndEnforcement(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "A configurable response", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=2", enum.RoleModerator); err != nil {
		t.Fatal(err)
	}
	actor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, actor); err != nil {
		t.Fatal(err)
	}
	save := func(id string, allowed bool) entity.RolePermissionUpdate {
		t.Helper()
		body, err := json.Marshal(actions.UpdateRolePermissions{SubmissionID: id, ResponseChanges: []entity.RoleResponseChange{
			{Role: enum.RoleModerator, Status: enum.PostPlanned, Granted: allowed},
		}})
		if err != nil {
			t.Fatal(err)
		}
		response, err := f.request(handlers.UpdateRolePermissions(), http.MethodPost, 0, string(body))
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("save status=%d error=%v body=%s", response.Code, err, response.Body)
		}
		var result entity.RolePermissionUpdate
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Blocked != "" {
			t.Fatalf("save result=%+v error=%v", result, err)
		}
		return result
	}
	save("grant-planned", true)
	reloaded := &query.GetTenantByDomain{Domain: "demo"}
	if err := bus.Dispatch(f.ctx, reloaded); err != nil {
		t.Fatal(err)
	}
	moderator := f
	moderator.user, moderator.tenant = actor.Result, reloaded.Result
	response, err := moderator.request(api.SetResponse(), http.MethodPost, post.Result.Number, `{"status":"planned","text":"Accepted for planning"}`)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("configured moderator response status=%d error=%v body=%s", response.Code, err, response.Body)
	}
	response, err = moderator.request(api.SetResponse(), http.MethodPost, post.Result.Number, `{"status":"completed","text":"Not granted"}`)
	if err != nil || response.Code != http.StatusBadRequest {
		t.Fatalf("unconfigured response status=%d error=%v", response.Code, err)
	}

	save("revoke-planned", false)
	if result := save("grant-planned", true); slices.Contains(result.Responses[enum.RoleModerator], enum.PostPlanned) {
		t.Fatal("accepted retry re-applied a later-revoked response grant")
	}
	response, err = moderator.request(api.SetResponse(), http.MethodPost, post.Result.Number, `{"status":"planned","text":"Stale request authority"}`)
	if err != nil || response.Code != http.StatusForbidden {
		t.Fatalf("stale response bypassed locked policy: status=%d error=%v body=%s", response.Code, err, response.Body)
	}
	var text string
	if err := mediaFixtureScalar(&text, "SELECT response FROM posts WHERE id=$1", post.Result.ID); err != nil || text != "Accepted for planning" {
		t.Fatalf("denied response changed content: %q error=%v", text, err)
	}
}

func TestResponseWaitsForCurrentPostVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Concurrent visibility decision", Description: "Original content"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=2", enum.RoleModerator); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.UpdateRolePermissions{
		SubmissionID:    "response-scope",
		Changes:         []entity.RolePermissionChange{{Role: enum.RoleModerator, Permission: entity.ModeratePosts, Granted: false}},
		ResponseChanges: []entity.RoleResponseChange{{Role: enum.RoleModerator, Status: enum.PostPlanned, Granted: true}},
	}); err != nil {
		t.Fatal(err)
	}
	actor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, actor); err != nil {
		t.Fatal(err)
	}
	locking, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer locking.Rollback()
	if _, err := locking.Execute("UPDATE posts SET moderation_pending=true WHERE id=$1", post.Result.ID); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() {
		completed <- bus.Dispatch(withUser(f.ctx, actor.Result), &cmd.SetPostResponse{Post: post.Result, Status: enum.PostPlanned, Text: "Stale visible snapshot"})
	}()
	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `SELECT COUNT(*) FROM pg_stat_activity
		WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FOR NO KEY UPDATE'`) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("response did not wait for the visibility decision: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("response did not reach its post lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := locking.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; errors.Cause(err) != app.ErrNotFound {
		t.Fatalf("response used visibility from before its lock: %v", err)
	}
	if count := workflowCount(t, "SELECT count(*) FROM posts WHERE response IS NOT NULL"); count != 0 {
		t.Fatal("a newly hidden post received a response")
	}
}

func TestRoleResponseInputRequiresASelectableStatus(t *testing.T) {
	f := newPostWorkflow(t)
	for _, change := range []string{
		`{"role":"helper","granted":true}`,
		`{"role":"helper","status":null,"granted":true}`,
		`{"role":"helper","status":"deleted","granted":true}`,
		`{"role":"helper","status":"archived","granted":true}`,
		`{"role":"helper","status":"unknown","granted":true}`,
	} {
		body := `{"submissionId":"invalid-response","responseChanges":[` + change + `]}`
		response, err := f.request(handlers.UpdateRolePermissions(), http.MethodPost, 0, body)
		if err != nil || response.Code != http.StatusBadRequest {
			t.Fatalf("invalid response target was accepted: status=%d error=%v body=%s", response.Code, err, response.Body)
		}
	}
	if count := workflowCount(t, "SELECT count(*) FROM command_receipts WHERE kind='role-permissions'"); count != 0 {
		t.Fatal("invalid response input wrote an accepted receipt")
	}
}
