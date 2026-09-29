package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"

	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
)

func storedRolePermissions(t *testing.T, tenantID int) string {
	t.Helper()
	var stored string
	Expect(trx.Scalar(&stored, "SELECT role_permissions::text FROM tenants WHERE id = $1", tenantID)).IsNil()
	return stored
}

func rolePermissionChange(role enum.Role, permission entity.Permission, granted bool) *cmd.UpdateRolePermissions {
	return &cmd.UpdateRolePermissions{SubmissionID: rand.String(32), Changes: []entity.RolePermissionChange{{Role: role, Permission: permission, Granted: granted}}}
}

func TestRolePermissionsStorage_SaveReloadReset(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	adminCtx := withTenant(withUser(ctx, jonSnow), demoTenant)
	Expect(storedRolePermissions(t, demoTenant.ID)).Equals("{}")

	grant := rolePermissionChange(enum.RoleHelper, entity.ManageReportReasons, true)
	Expect(bus.Dispatch(adminCtx, grant)).IsNil()
	Expect(demoTenant.RolePermissions.Grants(enum.RoleHelper, entity.ManageReportReasons)).IsTrue()
	Expect(storedRolePermissions(t, demoTenant.ID)).Equals(`{"helper": {"manageReports": true, "manageReportReasons": true}}`)

	reloaded := &query.GetTenantByDomain{Domain: "demo"}
	Expect(bus.Dispatch(ctx, reloaded)).IsNil()
	Expect(reloaded.Result.RolePermissions.Grants(enum.RoleHelper, entity.ManageReports)).IsTrue()
	Expect(reloaded.Result.RolePermissions.Grants(enum.RoleModerator, entity.ManageReports)).IsTrue()
	Expect(reloaded.Result.RolePermissions.Grants(enum.RoleVisitor, entity.ManageReports)).IsFalse()

	other := &query.GetTenantByDomain{Domain: "avengers"}
	Expect(bus.Dispatch(ctx, other)).IsNil()
	Expect(other.Result.RolePermissions.Grants(enum.RoleHelper, entity.ManageReports)).IsFalse()

	revoke := rolePermissionChange(enum.RoleHelper, entity.ManageReports, false)
	Expect(bus.Dispatch(adminCtx, revoke)).IsNil()
	Expect(storedRolePermissions(t, demoTenant.ID)).Equals("{}")
}

func TestRolePermissionsStorage_BlockedChangeKeepsStoredState(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	adminCtx := withTenant(withUser(ctx, jonSnow), demoTenant)
	Expect(bus.Dispatch(adminCtx, rolePermissionChange(enum.RoleCollaborator, entity.ManageRolePermissions, true))).IsNil()
	before := storedRolePermissions(t, demoTenant.ID)
	_, err := trx.Execute("UPDATE users SET role = $1 WHERE id = $2", enum.RoleCollaborator, aryaStark.ID)
	Expect(err).IsNil()

	editor := &entity.User{ID: aryaStark.ID, Role: enum.RoleCollaborator, Status: enum.UserActive, Tenant: demoTenant}
	editorCtx := withTenant(withUser(ctx, editor), demoTenant)

	for _, test := range []struct {
		ctx     context.Context
		change  *cmd.UpdateRolePermissions
		message string
	}{
		{editorCtx, rolePermissionChange(enum.RoleModerator, entity.ManageBilling, true), "You can only change permissions you have"},
		{editorCtx, rolePermissionChange(enum.RoleCollaborator, entity.ManageTags, false), "You can only change roles below your own"},
		{adminCtx, rolePermissionChange(enum.RoleAdministrator, entity.ManageTags, false), "Administrators have every permission"},
	} {
		Expect(bus.Dispatch(test.ctx, test.change)).IsNil()
		Expect(test.change.Result.Blocked).Equals(test.message)
	}

	Expect(storedRolePermissions(t, demoTenant.ID)).Equals(before)

	Expect(bus.Dispatch(editorCtx, rolePermissionChange(enum.RoleHelper, entity.ManageTags, true))).IsNil()
	Expect(demoTenant.RolePermissions.Grants(enum.RoleHelper, entity.ManageTags)).IsTrue()
}

func TestRolePermissionsStorage_AuthorizesAgainstStoredPermissions(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	stale := *demoTenant
	stale.RolePermissions = entity.RolePermissions{enum.RoleCollaborator: {entity.ManageRolePermissions: true}}
	editor := &entity.User{ID: aryaStark.ID, Role: enum.RoleCollaborator, Status: enum.UserActive, Tenant: &stale}

	err := bus.Dispatch(withTenant(withUser(ctx, editor), &stale), rolePermissionChange(enum.RoleVisitor, entity.CreatePosts, false))
	result, ok := errors.Cause(err).(*validate.Result)
	Expect(ok).IsTrue()
	Expect(result.Authorized).IsFalse()
	Expect(storedRolePermissions(t, demoTenant.ID)).Equals("{}")
}

func TestRolePermissionsStorage_ParsesStoredOverrides(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	_, err := trx.Execute(`UPDATE tenants SET role_permissions = '{"ghost": {"manageBilling": true}, "administrator": {"manageBilling": false}, "helper": {"retiredPermission": true, "manageReports": true}}' WHERE id = $1`, demoTenant.ID)
	Expect(err).IsNil()

	reloaded := &query.GetTenantByDomain{Domain: "demo"}
	Expect(bus.Dispatch(ctx, reloaded)).IsNil()
	Expect(reloaded.Result.RolePermissions).Equals(entity.RolePermissions{enum.RoleHelper: {entity.ManageReports: true}})

	for _, damaged := range []string{`[]`, `{"helper": {"manageReports": "yes"}}`} {
		_, err := trx.Execute("UPDATE tenants SET role_permissions = $1 WHERE id = $2", damaged, demoTenant.ID)
		Expect(err).IsNil()

		reloaded := &query.GetTenantByDomain{Domain: "demo"}
		Expect(bus.Dispatch(ctx, reloaded)).IsNotNil()
		Expect(reloaded.Result).IsNil()
	}
}

func TestRolePermissionsStorage_ConcurrentSavesKeepBothChanges(t *testing.T) {
	f := newPostWorkflow(t)
	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	// Requests must not share mutable tenant snapshots.
	withOwnTenant := func(ctx context.Context) context.Context {
		copy := *f.tenant
		return context.WithValue(ctx, app.TenantCtxKey, &copy)
	}

	first, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback()
	firstCtx := withOwnTenant(context.WithValue(ctx, app.TransactionCtxKey, first))
	if err := bus.Dispatch(firstCtx, rolePermissionChange(enum.RoleHelper, entity.ManageTags, true)); err != nil {
		t.Fatal(err)
	}

	completed := make(chan error, 1)
	go func() {
		completed <- bus.Dispatch(withOwnTenant(ctx), rolePermissionChange(enum.RoleVisitor, entity.ViewPostVotes, true))
	}()

	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `
		SELECT COUNT(*) FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock'
		  AND query LIKE '%role_permissions%'
	`) == 0 {
		select {
		case err := <-completed:
			t.Fatalf("second save did not wait for the first: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("second save never reached the row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}

	reloaded := &query.GetTenantByDomain{Domain: "demo"}
	if err := bus.Dispatch(context.WithValue(ctx, app.TenantCtxKey, nil), reloaded); err != nil {
		t.Fatal(err)
	}
	if !reloaded.Result.RolePermissions.Grants(enum.RoleHelper, entity.ManageTags) || !reloaded.Result.RolePermissions.Grants(enum.RoleVisitor, entity.ViewPostVotes) {
		t.Fatalf("a concurrent save was lost: %v", reloaded.Result.RolePermissions)
	}
}

func TestRolePermissionReceiptsProtectRetriesAndIndependentEdits(t *testing.T) {
	f := newPostWorkflow(t)
	apply := func(id string, changes ...entity.RolePermissionChange) entity.RolePermissionUpdate {
		t.Helper()
		command := &cmd.UpdateRolePermissions{SubmissionID: id, Changes: changes}
		if err := bus.Dispatch(f.ctx, command); err != nil {
			t.Fatal(err)
		}
		return command.Result
	}
	granted := func(state entity.RolePermissionUpdate, role enum.Role, permission entity.Permission) bool {
		return slices.Contains(state.Permissions[role], permission)
	}

	grant := entity.RolePermissionChange{Role: enum.RoleHelper, Permission: entity.ManageReports, Granted: true}
	first := apply("grant", grant)
	if !granted(first, grant.Role, grant.Permission) {
		t.Fatal("initial grant was not saved")
	}

	independent := entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ViewPostVotes, Granted: true}
	apply("independent", independent)
	retry := apply("grant", grant)
	if !granted(retry, grant.Role, grant.Permission) || !granted(retry, independent.Role, independent.Permission) {
		t.Fatal("retry lost an independent change")
	}

	revoke := grant
	revoke.Granted = false
	apply("revoke", revoke)
	retry = apply("grant", grant)
	if granted(retry, grant.Role, grant.Permission) {
		t.Fatal("accepted retry undid a later revocation")
	}

	firstSave := apply("new-intent", grant)
	if !granted(firstSave, grant.Role, grant.Permission) {
		t.Fatal("a new authorized grant was rejected because of old activity")
	}

	response := entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ManageReportReasons, Granted: true}
	dependent := apply("response", response)
	if !granted(dependent, response.Role, entity.ManageReports) {
		t.Fatal("required permission was not saved with its dependent")
	}

	apply("revoke-response", entity.RolePermissionChange{Role: response.Role, Permission: entity.ManageReports})
	retry = apply("response", response)
	if granted(retry, response.Role, response.Permission) || granted(retry, response.Role, entity.ManageReports) {
		t.Fatal("accepted retry restored a revoked dependency closure")
	}

	changedRequest := &cmd.UpdateRolePermissions{SubmissionID: "grant", Changes: []entity.RolePermissionChange{revoke}}
	if err := bus.Dispatch(f.ctx, changedRequest); errors.Cause(err) != app.ErrConflict {
		t.Fatalf("identity reuse changed the accepted request: %v", err)
	}

	if _, err := dbx.Connection().Exec("UPDATE users SET status = $1 WHERE id = $2", enum.UserBlocked, f.user.ID); err != nil {
		t.Fatal(err)
	}
	acceptedRetry := &cmd.UpdateRolePermissions{SubmissionID: "grant", Changes: []entity.RolePermissionChange{grant}}
	err := bus.Dispatch(f.ctx, acceptedRetry)
	rejected, ok := errors.Cause(err).(*validate.Result)
	if !ok || rejected.Authorized {
		t.Fatalf("receipt bypassed the actor's current authority: %v", err)
	}
}

func BenchmarkRolePermissionSaveHTTP(b *testing.B) {
	f := newPostWorkflow(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		granted := i%2 == 0
		body, err := json.Marshal(actions.UpdateRolePermissions{
			Changes:      []entity.RolePermissionChange{{Role: enum.RoleVisitor, Permission: entity.LockPosts, Granted: granted}},
			SubmissionID: fmt.Sprintf("permission-%d", i),
		})
		if err != nil {
			b.Fatal(err)
		}

		response, err := f.requestWithParams(handlers.UpdateRolePermissions(), http.MethodPut,
			"http://localhost:3000/api/admin/permissions", string(body), nil)
		if err != nil || response.Code != http.StatusOK {
			b.Fatalf("save: %v, status %d", err, response.Code)
		}

		var result entity.RolePermissionUpdate
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			b.Fatal(err)
		}
		if result.Blocked != "" || slices.Contains(result.Permissions[enum.RoleVisitor], entity.LockPosts) != granted {
			b.Fatalf("save did not complete: %+v", result)
		}
	}
}
