package postgres_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres"
)

func TestRealtimeAccessUsesCurrentStoredPermissions(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	viewerCtx := withUser(ctx, jonSnow)
	check := func(ctx context.Context, reports, queue bool) {
		t.Helper()
		viewer := query.RealtimeViewer{}
		if tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant); tenant != nil {
			viewer.TenantID = tenant.ID
		}
		if user, _ := ctx.Value(app.UserCtxKey).(*entity.User); user != nil {
			viewer.UserID = user.ID
		}
		access := &query.GetRealtimeAccess{Viewers: []query.RealtimeViewer{viewer}}
		if err := bus.Dispatch(ctx, access); err != nil {
			t.Fatal(err)
		}

		if len(access.Result) != 1 || access.Result[0].Reports != reports || access.Result[0].Queue != queue {
			t.Fatalf("unexpected access: %+v; want reports=%v queue=%v", access.Result, reports, queue)
		}
	}

	for _, test := range []struct {
		role    enum.Role
		reports bool
		queue   bool
	}{
		{role: enum.RoleVisitor},
		{role: enum.RoleHelper, queue: true},
		{role: enum.RoleModerator, reports: true, queue: true},
		{role: enum.RoleCollaborator, reports: true, queue: true},
		{role: enum.RoleAdministrator, reports: true, queue: true},
	} {
		t.Run(test.role.String(), func(t *testing.T) {
			if _, err := trx.Execute("UPDATE users SET role = $1 WHERE id = $2", test.role, jonSnow.ID); err != nil {
				t.Fatal(err)
			}

			check(viewerCtx, test.reports, test.queue)
		})
	}

	if _, err := trx.Execute("UPDATE users SET role = $1 WHERE id = $2", enum.RoleModerator, jonSnow.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`UPDATE tenants SET role_permissions = $1 WHERE id = $2`, `{"moderator":{"manageReports":false,"manageQueue":false}}`, demoTenant.ID); err != nil {
		t.Fatal(err)
	}

	check(viewerCtx, false, false)

	if _, err := trx.Execute(`UPDATE tenants SET role_permissions = '{}' WHERE id = $1`, demoTenant.ID); err != nil {
		t.Fatal(err)
	}

	check(viewerCtx, true, true)

	for _, status := range []enum.UserStatus{enum.UserBlocked, enum.UserDeleted, enum.UserActive} {
		if _, err := trx.Execute("UPDATE users SET status = $1 WHERE id = $2", status, jonSnow.ID); err != nil {
			t.Fatal(err)
		}

		check(viewerCtx, status == enum.UserActive, status == enum.UserActive)
	}

	for _, status := range []enum.TenantStatus{enum.TenantLocked, enum.TenantDisabled, enum.TenantPending, enum.TenantActive} {
		if _, err := trx.Execute("UPDATE tenants SET status = $1 WHERE id = $2", status, demoTenant.ID); err != nil {
			t.Fatal(err)
		}

		check(viewerCtx, status == enum.TenantActive, status == enum.TenantActive)
	}

	check(withTenant(viewerCtx, avengersTenant), false, false)
	check(withTenant(ctx, demoTenant), false, false)
	check(withUser(ctx, &entity.User{ID: -1, Tenant: demoTenant, Role: enum.RoleAdministrator, Status: enum.UserActive}), false, false)
}

func TestRealtimeBatchPreservesTenantAndUserBoundaries(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	if _, err := trx.Execute(`UPDATE users SET role=$1 WHERE id=$2`, enum.RoleHelper, aryaStark.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`UPDATE users SET status=$1 WHERE id=$2`, enum.UserBlocked, sansaStark.ID); err != nil {
		t.Fatal(err)
	}

	access := &query.GetRealtimeAccess{Viewers: []query.RealtimeViewer{
		{TenantID: demoTenant.ID, UserID: aryaStark.ID},
		{TenantID: demoTenant.ID, UserID: tonyStark.ID},
		{TenantID: avengersTenant.ID, UserID: tonyStark.ID},
		{TenantID: demoTenant.ID, UserID: sansaStark.ID},
		{TenantID: demoTenant.ID, UserID: -1},
		{TenantID: demoTenant.ID, UserID: jonSnow.ID},
		{TenantID: demoTenant.ID, UserID: aryaStark.ID},
	}}
	if err := bus.Dispatch(ctx, access); err != nil {
		t.Fatal(err)
	}
	want := []query.RealtimeAccess{
		{Queue: true}, {}, {Reports: true, Queue: true}, {}, {},
		{Reports: true, Queue: true}, {Queue: true},
	}
	if len(access.Result) != len(want) {
		t.Fatalf("missing viewer positions: %+v", access.Result)
	}
	for index, expected := range want {
		if access.Result[index] != expected {
			t.Fatalf("viewer %d: got %+v want %+v", index, access.Result[index], expected)
		}
	}
}

func BenchmarkRealtimeAccess(b *testing.B) {
	bus.Init(postgres.Service{})
	request := web.Request{URL: &url.URL{Scheme: "http", Host: "demo.test.fider.io"}}
	ctx := context.WithValue(context.Background(), app.RequestCtxKey, request)
	tenant := &query.GetTenantByDomain{Domain: "demo"}
	if err := bus.Dispatch(ctx, tenant); err != nil {
		b.Fatal(err)
	}

	ctx = withTenant(ctx, tenant.Result)
	user := &query.GetUserByEmail{Email: "jon.snow@got.com"}
	if err := bus.Dispatch(ctx, user); err != nil {
		b.Fatal(err)
	}
	ctx = withUser(ctx, user.Result)

	b.Run("current_access", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			access := &query.GetRealtimeAccess{Viewers: []query.RealtimeViewer{{TenantID: tenant.Result.ID, UserID: user.Result.ID}}}
			if err := bus.Dispatch(ctx, access); err != nil {
				b.Fatal(err)
			}
			if !access.Result[0].Queue || !access.Result[0].Reports {
				b.Fatal("administrator lost access")
			}
		}
	})

	b.Run("reload_identity", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			currentTenant := &query.GetTenantByDomain{Domain: "demo"}
			currentUser := &query.GetUserByID{UserID: user.Result.ID}
			if err := bus.Dispatch(ctx, currentTenant, currentUser); err != nil {
				b.Fatal(err)
			}
			if !entity.Can(currentUser.Result, currentTenant.Result, entity.ManageQueue) || !entity.Can(currentUser.Result, currentTenant.Result, entity.ManageReports) {
				b.Fatal("administrator lost access")
			}
		}
	})
}
