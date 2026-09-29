package postgres_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestProfileHTTPWaitsForCurrentAuthority(t *testing.T) {
	for _, field := range []string{"name", "avatar"} {
		for _, transition := range []string{"role", "permission", "self block"} {
			t.Run(field+"/"+transition, func(t *testing.T) {
				f := newPostWorkflow(t)
				if err := bus.Dispatch(f.ctx, &cmd.ChangeUserRole{UserID: 3, Role: enum.RoleCollaborator}); err != nil {
					t.Fatal(err)
				}
				actorID := 3
				if transition == "self block" {
					actorID = 2
				}
				actor := &query.GetUserByID{UserID: actorID}
				if err := bus.Dispatch(f.ctx, actor); err != nil {
					t.Fatal(err)
				}

				acting := f
				acting.user, acting.tenant = actor.Result, actor.Result.Tenant
				handler, body := handlers.UpdateUserName(), `{"name":"Profile after recovery"}`
				if field == "avatar" {
					handler, body = handlers.UpdateUserAvatar(), `{"avatarType":"letter"}`
				}
				request := func() (*httptest.ResponseRecorder, error) {
					return acting.requestWithParams(handler, http.MethodPost, "/api/user/2/profile", body, web.StringMap{"userID": "2"})
				}

				locking, err := dbx.BeginTx(f.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer locking.Rollback()
				var revoke bus.Msg = &cmd.ChangeUserRole{UserID: actorID, Role: enum.RoleVisitor}
				if transition == "permission" {
					revoke = &cmd.UpdateRolePermissions{SubmissionID: "revoke-profile-edit", Changes: []entity.RolePermissionChange{
						{Role: enum.RoleCollaborator, Permission: entity.EditUserProfiles, Granted: false},
					}}
				} else if transition == "self block" {
					revoke = &cmd.BlockUser{UserID: actorID}
				}
				if err := bus.Dispatch(context.WithValue(f.ctx, app.TransactionCtxKey, locking), revoke); err != nil {
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
					case result := <-completed:
						t.Fatalf("profile write did not wait for revocation: status=%d error=%v body=%s", result.response.Code, result.err, result.response.Body)
					default:
					}
					if time.Now().After(deadline) {
						t.Fatal("profile write did not reach its authority lock")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := locking.Commit(); err != nil {
					t.Fatal(err)
				}
				result := <-completed
				if result.err != nil || result.response.Code != http.StatusForbidden {
					t.Fatalf("revoked profile write: status=%d error=%v body=%s", result.response.Code, result.err, result.response.Body)
				}
				if count := workflowCount(t, "SELECT count(*) FROM users WHERE id=2 AND name='Arya Stark' AND avatar_type=$1", enum.AvatarTypeGravatar); count != 1 {
					t.Fatal("revoked profile write changed the target")
				}

				var restore bus.Msg = &cmd.ChangeUserRole{UserID: actorID, Role: enum.RoleCollaborator}
				if transition == "permission" {
					restore = &cmd.UpdateRolePermissions{SubmissionID: "restore-profile-edit", Changes: []entity.RolePermissionChange{
						{Role: enum.RoleCollaborator, Permission: entity.EditUserProfiles, Granted: true},
					}}
				} else if transition == "self block" {
					restore = &cmd.UnblockUser{UserID: actorID}
				}
				if err := bus.Dispatch(f.ctx, restore); err != nil {
					t.Fatal(err)
				}
				response, err := request()
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("same profile update did not recover: status=%d error=%v body=%s", response.Code, err, response.Body)
				}
				if field == "name" {
					if count := workflowCount(t, "SELECT count(*) FROM users WHERE id=2 AND name='Profile after recovery'"); count != 1 {
						t.Fatal("recovered name was not published")
					}
				} else if count := workflowCount(t, "SELECT count(*) FROM users WHERE id=2 AND avatar_type=$1", enum.AvatarTypeLetter); count != 1 {
					t.Fatal("recovered avatar choice was not published")
				}
			})
		}
	}
}
