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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestUserMutationWaitsForCurrentActor(t *testing.T) {
	for _, change := range []string{"role", "permission", "assign administrator"} {
		t.Run(change, func(t *testing.T) {
			f := newPostWorkflow(t)
			role := enum.RoleCollaborator
			if change == "assign administrator" {
				role = enum.RoleAdministrator
			}
			if err := bus.Dispatch(f.ctx, &cmd.ChangeUserRole{UserID: 3, Role: role}, &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleVisitor}); err != nil {
				t.Fatal(err)
			}
			actor := &query.GetUserByID{UserID: 3}
			if err := bus.Dispatch(f.ctx, actor); err != nil {
				t.Fatal(err)
			}

			locking, err := dbx.BeginTx(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer locking.Rollback()
			mutationCtx := context.WithValue(f.ctx, app.TransactionCtxKey, locking)
			var revoke bus.Msg = &cmd.ChangeUserRole{UserID: 3, Role: enum.RoleVisitor}
			if change == "permission" {
				revoke = &cmd.UpdateRolePermissions{SubmissionID: "revoke-block", Changes: []entity.RolePermissionChange{
					{Role: role, Permission: entity.BlockUsers, Granted: false},
				}}
			} else if change == "assign administrator" {
				revoke = &cmd.ChangeUserRole{UserID: 3, Role: enum.RoleModerator}
			}
			if err := bus.Dispatch(mutationCtx, revoke); err != nil {
				t.Fatal(err)
			}

			attempt := func() bus.Msg {
				if change == "assign administrator" {
					return &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleAdministrator}
				}
				return &cmd.BlockUser{UserID: 2}
			}

			ctx, cancel := context.WithTimeout(withUser(f.ctx, actor.Result), 10*time.Second)
			defer cancel()
			completed := make(chan error, 1)
			go func() { completed <- bus.Dispatch(ctx, attempt()) }()
			deadline := time.Now().Add(5 * time.Second)
			for workflowCount(t, `SELECT count(*) FROM pg_stat_activity
				WHERE datname=current_database() AND wait_event_type='Lock'`) == 0 {
				select {
				case err := <-completed:
					t.Fatalf("user mutation did not wait for revocation: %v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("user mutation did not reach its authority lock")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := locking.Commit(); err != nil {
				t.Fatal(err)
			}
			if result, ok := (<-completed).(*validate.Result); !ok || result.Authorized {
				t.Fatalf("revoked user mutation was accepted: %+v", result)
			}
			if count := workflowCount(t, "SELECT count(*) FROM users WHERE id=2 AND role=$1 AND status=$2", enum.RoleVisitor, enum.UserActive); count != 1 {
				t.Fatal("revoked mutation changed the target")
			}

			var restore bus.Msg = &cmd.ChangeUserRole{UserID: 3, Role: role}
			if change == "permission" {
				restore = &cmd.UpdateRolePermissions{SubmissionID: "restore-block", Changes: []entity.RolePermissionChange{
					{Role: role, Permission: entity.BlockUsers, Granted: true},
				}}
			}
			if err := bus.Dispatch(f.ctx, restore); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(ctx, attempt()); err != nil {
				t.Fatalf("restored actor could not recover: %v", err)
			}
		})
	}
}

func TestUserProfileMutationsLockBothActorsInOrder(t *testing.T) {
	f := newPostWorkflow(t)
	if err := bus.Dispatch(f.ctx, &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleAdministrator}); err != nil {
		t.Fatal(err)
	}
	other := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, other); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	completed := make(chan error, 2)
	for _, operation := range []struct {
		actor  *entity.User
		target int
	}{{f.user, 2}, {other.Result, 1}} {
		go func() {
			<-start
			completed <- bus.Dispatch(withUser(ctx, operation.actor), &cmd.SaveProfileName{UserID: operation.target, Name: "Shared administrative edit"})
		}()
	}
	close(start)
	for range 2 {
		if err := <-completed; err != nil {
			t.Fatalf("mutual administrator edits failed: %v", err)
		}
	}
	if count := workflowCount(t, "SELECT count(*) FROM users WHERE id IN (1,2) AND name='Shared administrative edit'"); count != 2 {
		t.Fatal("mutual administrator edits were not both retained")
	}
}
