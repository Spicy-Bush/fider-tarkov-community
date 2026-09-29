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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestVoteWaitsForCurrentAuthority(t *testing.T) {
	for _, change := range []string{"role", "mute", "blocked", "permission"} {
		t.Run(change, func(t *testing.T) {
			f := newPostWorkflow(t)
			role := enum.RoleModerator
			if change == "permission" {
				role = enum.RoleCollaborator
			}
			if err := bus.Dispatch(f.ctx, &cmd.ChangeUserRole{UserID: 2, Role: role}); err != nil {
				t.Fatal(err)
			}
			actor := &query.GetUserByID{UserID: 2}
			if err := bus.Dispatch(f.ctx, actor); err != nil {
				t.Fatal(err)
			}
			post := &cmd.AddNewPost{Title: "Vote authority", Description: "Existing vote survives revocation"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}
			if _, err := mediaFixtureSQL(`UPDATE posts SET moderation_pending=$1,
				locked_settings=jsonb_build_object('locked', $2::boolean) WHERE id=$3`,
				change == "role", change == "permission", post.Result.ID); err != nil {
				t.Fatal(err)
			}
			staleCtx := withUser(f.ctx, actor.Result)
			initial := &cmd.ApplyPostVote{Number: post.Result.Number, Direction: 1}
			if err := bus.Dispatch(staleCtx, initial); err != nil || !initial.State.Applied || initial.State.Revision != 1 {
				t.Fatalf("initial vote: state=%+v error=%v", initial.State, err)
			}

			locking, err := dbx.BeginTx(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer locking.Rollback()
			mutationCtx := context.WithValue(f.ctx, app.TransactionCtxKey, locking)
			var mutation bus.Msg
			switch change {
			case "role":
				mutation = &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleVisitor}
			case "mute":
				mutation = &cmd.MuteUser{UserID: 2, Reason: "Pending vote", ExpiresAt: time.Now().Add(time.Hour)}
			case "blocked":
				mutation = &cmd.BlockUser{UserID: 2}
			case "permission":
				mutation = &cmd.UpdateRolePermissions{SubmissionID: "revoke-vote-lock-access", Changes: []entity.RolePermissionChange{
					{Role: role, Permission: entity.LockPosts, Granted: false},
				}}
			}
			if err := bus.Dispatch(mutationCtx, mutation); err != nil {
				t.Fatal(err)
			}

			waitingCtx, cancel := context.WithTimeout(staleCtx, 10*time.Second)
			defer cancel()
			completed := make(chan error, 1)
			attempt := &cmd.ApplyPostVote{Number: post.Result.Number, Direction: -1, Revision: 1}
			go func() { completed <- bus.Dispatch(waitingCtx, attempt) }()
			deadline := time.Now().Add(5 * time.Second)
			for workflowCount(t, `SELECT COUNT(*) FROM pg_stat_activity
				WHERE datname=current_database() AND wait_event_type='Lock'`) == 0 {
				select {
				case err := <-completed:
					t.Fatalf("vote did not wait for the authority change: %v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("vote did not reach its authority lock")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := locking.Commit(); err != nil {
				t.Fatal(err)
			}
			err = <-completed
			if change == "role" {
				if errors.Cause(err) != app.ErrNotFound {
					t.Fatalf("revoked visibility error=%v", err)
				}
			} else if result, ok := err.(*validate.Result); !ok || result.Authorized {
				t.Fatalf("revoked authority error=%v", err)
			}
			if got := workflowCount(t, "SELECT count(*) FROM post_votes WHERE user_id=2 AND vote_type=1"); got != 1 {
				t.Fatal("rejected change replaced the existing vote")
			}
			if got := workflowCount(t, "SELECT count(*) FROM post_vote_revisions WHERE user_id=2 AND revision=1"); got != 1 {
				t.Fatal("rejected change advanced the vote revision")
			}

			var recovery bus.Msg
			switch change {
			case "role":
				recovery = &cmd.ChangeUserRole{UserID: 2, Role: role}
			case "mute":
				var muteID int
				if err := mediaFixtureScalar(&muteID, "SELECT id FROM user_mutes WHERE user_id=2"); err != nil {
					t.Fatal(err)
				}
				recovery = &cmd.ExpireMute{UserID: 2, MuteID: muteID}
			case "blocked":
				recovery = &cmd.UnblockUser{UserID: 2}
			case "permission":
				recovery = &cmd.UpdateRolePermissions{SubmissionID: "restore-vote-lock-access", Changes: []entity.RolePermissionChange{
					{Role: role, Permission: entity.LockPosts, Granted: true},
				}}
			}
			if err := bus.Dispatch(f.ctx, recovery); err != nil {
				t.Fatal(err)
			}
			retry := &cmd.ApplyPostVote{Number: post.Result.Number, Direction: -1, Revision: 1}
			if err := bus.Dispatch(staleCtx, retry); err != nil || !retry.State.Applied || retry.State.Direction != -1 || retry.State.Revision != 2 {
				t.Fatalf("restored authority did not recover: state=%+v error=%v", retry.State, err)
			}
		})
	}
}

func TestVoteAcceptsMuteExpiryDuringAuthorityWait(t *testing.T) {
	f := newPostWorkflow(t)
	actor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, actor); err != nil {
		t.Fatal(err)
	}

	post := &cmd.AddNewPost{Title: "Expired mute", Description: "A vote waiting for current standing"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	locking, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer locking.Rollback()

	if _, err := locking.Execute("SELECT id FROM users WHERE id=$1 FOR UPDATE", actor.Result.ID); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(withUser(f.ctx, actor.Result), 10*time.Second)
	defer cancel()
	completed := make(chan error, 1)
	vote := &cmd.ApplyPostVote{Number: post.Result.Number, Direction: 1}
	go func() {
		completed <- bus.Dispatch(ctx, vote)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `SELECT count(*) FROM pg_stat_activity
		WHERE datname=current_database() AND wait_event_type='Lock'`) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("vote did not wait for the user lock: %v", err)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("vote did not reach its authority lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := locking.Execute(`INSERT INTO user_mutes
		(tenant_id, user_id, reason, created_by, expires_at)
		VALUES ($1, $2, 'Expired while waiting', $3, clock_timestamp())`, f.tenant.ID, actor.Result.ID, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if err := locking.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-completed; err != nil {
		t.Fatalf("expired mute rejected the waiting vote: %v", err)
	}
	if !vote.State.Applied || vote.State.Direction != 1 || vote.State.Revision != 1 {
		t.Fatalf("waiting vote was not applied: %+v", vote.State)
	}
}
