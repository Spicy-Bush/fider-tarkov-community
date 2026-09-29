package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestChangeUserRoleResponseUsesCurrentAuthority(t *testing.T) {
	f := newPostWorkflow(t)
	if err := bus.Dispatch(f.ctx,
		&cmd.ChangeUserRole{UserID: 3, Role: enum.RoleCollaborator},
		&cmd.ChangeUserRole{UserID: 2, Role: enum.RoleVisitor},
		&cmd.ChangeUserVisualRole{UserID: 2, VisualRole: enum.VisualRoleNone},
		&cmd.UpdateRolePermissions{
			SubmissionID: "delegate-role-change",
			Changes: []entity.RolePermissionChange{
				{Role: enum.RoleCollaborator, Permission: entity.ChangeUserRoles, Granted: true},
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	tenant := &query.GetTenantByDomain{Domain: "demo"}
	actor := &query.GetUserByID{UserID: 3}
	if err := bus.Dispatch(f.ctx, tenant, actor); err != nil {
		t.Fatal(err)
	}
	if !entity.Can(actor.Result, tenant.Result, entity.ChangeUserRoles) || !entity.Can(actor.Result, tenant.Result, entity.BlockUsers) {
		t.Fatal("actor must initially have both role-change and blocking authority")
	}

	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "http://localhost:3000/api/admin/roles/helper/users", strings.NewReader(`{"userID":2}`)).WithContext(ctx)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	requestContext, err := web.NewContext(f.engine, req, recorder, web.StringMap{"role": "helper"})
	if err != nil {
		t.Fatal(err)
	}
	requestContext.SetTenant(tenant.Result)
	requestContext.SetUser(actor.Result)

	locking, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer locking.Rollback()
	mutationCtx := context.WithValue(f.ctx, app.TransactionCtxKey, locking)
	if err := bus.Dispatch(mutationCtx, &cmd.UpdateRolePermissions{
		SubmissionID: "revoke-block-during-role-change",
		Changes: []entity.RolePermissionChange{
			{Role: enum.RoleCollaborator, Permission: entity.BlockUsers, Granted: false},
		},
	}); err != nil {
		t.Fatal(err)
	}

	completed := make(chan error, 1)
	go func() {
		completed <- handlers.ChangeUserRole()(requestContext)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `
		SELECT count(*) FROM pg_stat_activity
		WHERE datname=current_database() AND wait_event_type='Lock'
		AND query LIKE '%role_post_responses%' AND query LIKE '%FOR SHARE%'
	`) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("role change did not wait for policy update: %v, response %d %s", err, recorder.Code, recorder.Body)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("role change did not reach its authority lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := locking.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("still-authorized role change failed: %d %s", recorder.Code, recorder.Body)
	}

	var response struct {
		ID                 int                    `json:"id"`
		Role               enum.Role              `json:"role"`
		VisualRole         enum.VisualRole        `json:"visualRole"`
		VisualRoleOverride enum.VisualRole        `json:"visualRoleOverride"`
		Permissions        entity.UserPermissions `json:"permissions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != 2 || response.Role != enum.RoleHelper || response.VisualRole != enum.VisualRoleHelper || response.VisualRoleOverride != enum.VisualRoleNone {
		t.Fatalf("role change returned stale target identity: %s", recorder.Body)
	}
	if count := workflowCount(t, "SELECT count(*) FROM users WHERE id=2 AND role=$1 AND visual_role=$2", enum.RoleHelper, enum.VisualRoleNone); count != 1 {
		t.Fatal("successful response did not match the saved role")
	}
	if response.Permissions.Block || !response.Permissions.ChangeRole {
		t.Fatalf("role response did not use the current policy: %s", recorder.Body)
	}
}
