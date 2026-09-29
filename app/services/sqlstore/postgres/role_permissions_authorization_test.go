package postgres_test

import (
	"context"
	"net/http"
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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestDelegatedRoleEditorCannotAdministerAuthentication(t *testing.T) {
	f := newPostWorkflow(t)
	if err := bus.Dispatch(f.ctx, rolePermissionChange(enum.RoleCollaborator, entity.ManageRolePermissions, true)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleCollaborator}); err != nil {
		t.Fatal(err)
	}

	editor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, editor); err != nil {
		t.Fatal(err)
	}
	f.user = editor.Result

	response, err := f.requestWithParams(handlers.UpdateRolePermissions(), http.MethodPut,
		"http://localhost:3000/api/admin/permissions",
		`{"changes":[{"role":"helper","permission":"manageTags","granted":true}],"submissionId":"permission-save"}`, nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("delegated policy save: status=%d body=%s err=%v", response.Code, response.Body.String(), err)
	}

	// Even a pre-migration override cannot reopen authentication administration.
	f.tenant.RolePermissions[enum.RoleCollaborator][entity.ManageAuthentication] = true
	for _, handler := range []struct {
		name   string
		path   string
		body   string
		action web.HandlerFunc
	}{
		{
			name:   "provider",
			path:   "/api/admin/oauth",
			body:   `{"displayName":"Untrusted provider","status":1}`,
			action: handlers.SaveOAuthConfig(),
		},
		{
			name:   "email authentication",
			path:   "/api/admin/settings/emailauth",
			body:   `{"isEmailAuthAllowed":false}`,
			action: handlers.UpdateEmailAuthAllowed(),
		},
	} {
		t.Run(handler.name, func(t *testing.T) {
			response, err := f.requestWithParams(handler.action, http.MethodPost,
				"http://localhost:3000"+handler.path, handler.body, nil)
			if err != nil || response.Code != http.StatusForbidden {
				t.Fatalf("authentication write: status=%d body=%s err=%v", response.Code, response.Body.String(), err)
			}
		})
	}

	if got := workflowCount(t, "SELECT COUNT(*) FROM oauth_providers WHERE tenant_id = $1", f.tenant.ID); got != 0 {
		t.Fatalf("delegated editor created %d authentication providers", got)
	}
}

func TestRolePermissionWriteRechecksAuthorityAfterWaiting(t *testing.T) {
	for _, revoked := range []string{
		"demoted",
		"blocked",
		"deleted",
		"tenant locked",
		"tenant disabled",
		"delegation revoked",
		"demoted with delegated access",
	} {
		t.Run(revoked, func(t *testing.T) {
			f := newPostWorkflow(t)
			if err := bus.Dispatch(f.ctx,
				rolePermissionChange(enum.RoleCollaborator, entity.ManageRolePermissions, true),
				&cmd.ChangeUserRole{UserID: 2, Role: enum.RoleCollaborator},
			); err != nil {
				t.Fatal(err)
			}
			if revoked == "demoted with delegated access" {
				if err := bus.Dispatch(f.ctx,
					rolePermissionChange(enum.RoleModerator, entity.ManageRolePermissions, true),
					rolePermissionChange(enum.RoleModerator, entity.ManageTags, true),
				); err != nil {
					t.Fatal(err)
				}
			}

			editor := &query.GetUserByID{UserID: 2}
			if err := bus.Dispatch(f.ctx, editor); err != nil {
				t.Fatal(err)
			}
			requestTenant := *f.tenant
			requestCtx, cancel := context.WithTimeout(withTenant(withUser(f.ctx, editor.Result), &requestTenant), 10*time.Second)
			defer cancel()

			lock, err := dbx.BeginTx(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback()

			var id int
			if err := lock.Scalar(&id, "SELECT id FROM tenants WHERE id = $1 FOR UPDATE", f.tenant.ID); err != nil {
				t.Fatal(err)
			}
			completed := make(chan error, 1)
			command := rolePermissionChange(enum.RoleHelper, entity.ManageTags, true)
			go func() {
				completed <- bus.Dispatch(requestCtx, command)
			}()
			waitForPermissionWriteLock(t, completed)

			revokeCtx, stopRevoke := context.WithTimeout(context.WithValue(f.ctx, app.TransactionCtxKey, lock), 2*time.Second)
			defer stopRevoke()
			switch revoked {
			case "demoted":
				err = bus.Dispatch(revokeCtx, &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleVisitor})
			case "demoted with delegated access":
				err = bus.Dispatch(revokeCtx, &cmd.ChangeUserRole{UserID: 2, Role: enum.RoleModerator})
			case "blocked":
				err = bus.Dispatch(revokeCtx, &cmd.BlockUser{UserID: 2})
			case "deleted":
				_, err = dbx.Connection().ExecContext(requestCtx, "UPDATE users SET status = $1 WHERE id = $2", enum.UserDeleted, 2)
			case "tenant locked":
				_, err = lock.Execute("UPDATE tenants SET status = $1 WHERE id = $2", enum.TenantLocked, f.tenant.ID)
			case "tenant disabled":
				_, err = lock.Execute("UPDATE tenants SET status = $1 WHERE id = $2", enum.TenantDisabled, f.tenant.ID)
			case "delegation revoked":
				_, err = lock.Execute("UPDATE tenants SET role_permissions = '{}' WHERE id = $1", f.tenant.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := lock.Commit(); err != nil {
				t.Fatal(err)
			}

			err = <-completed
			if revoked == "demoted with delegated access" {
				if err != nil {
					t.Fatal(err)
				}
				if command.Result.BaseLocks[enum.RoleModerator][entity.ManageTags] != "You can only change roles below your own" {
					t.Fatal("saved response advertised locks from the editor's previous role")
				}
				if !requestTenant.RolePermissions.Grants(enum.RoleHelper, entity.ManageTags) {
					t.Fatal("a still-authorized edit was lost")
				}
				return
			}

			result, ok := errors.Cause(err).(*validate.Result)
			if !ok || result.Authorized {
				t.Fatalf("revoked request was not denied: %+v", result)
			}
			if got := workflowCount(t, `
				SELECT COUNT(*) FROM tenants
				WHERE id = $1 AND role_permissions @> '{"helper":{"manageTags":true}}'
			`, f.tenant.ID); got != 0 {
				t.Fatal("revoked request persisted its permission change")
			}
		})
	}
}

func waitForPermissionWriteLock(t *testing.T, completed <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `
		SELECT COUNT(*) FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock'
		  AND query LIKE '%SELECT role_permissions%'
	`) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("permission write finished before waiting: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("permission write never reached the tenant lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
