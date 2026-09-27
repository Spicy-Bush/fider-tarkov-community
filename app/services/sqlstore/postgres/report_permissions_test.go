package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestReportUsersProjectCurrentTargetPermissions(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Reported profile", Description: "Report profile projection"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	var reportID int
	err := dbx.Connection().QueryRow(`
		INSERT INTO reports (
			tenant_id, reporter_id, reported_type, reported_id, reason, status,
			assigned_to, assigned_at, resolved_by, resolved_at, created_at
		) VALUES ($1, 2, 'post', $2, 'Profile projection', 'resolved', 2, NOW(), 2, NOW(), NOW())
		RETURNING id
	`, f.tenant.ID, post.Result.ID).Scan(&reportID)
	if err != nil {
		t.Fatal(err)
	}

	read := func(t *testing.T, ctx context.Context) []*entity.Report {
		t.Helper()
		detail := &query.GetReportByID{ReportID: reportID}
		list := &query.ListReports{Page: 1, PerPage: 10}
		if err := bus.Dispatch(ctx, detail, list); err != nil {
			t.Fatal(err)
		}
		if len(list.Result) != 1 || list.Total != 1 {
			t.Fatalf("expected one report, got %d rows and total %d", len(list.Result), list.Total)
		}
		return []*entity.Report{detail.Result, list.Result[0]}
	}

	roles := []enum.Role{
		enum.RoleVisitor,
		enum.RoleHelper,
		enum.RoleModerator,
		enum.RoleCollaborator,
		enum.RoleAdministrator,
	}
	statuses := []enum.UserStatus{enum.UserActive, enum.UserBlocked, enum.UserDeleted}
	viewers := []enum.Role{enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}
	for _, role := range roles {
		for _, status := range statuses {
			if _, err := dbx.Connection().Exec("UPDATE users SET role = $1, status = $2 WHERE id = 2", role, status); err != nil {
				t.Fatal(err)
			}
			for _, viewerRole := range viewers {
				t.Run(fmt.Sprintf("%s views %s %s", viewerRole, status, role), func(t *testing.T) {
					viewer := *f.user
					viewer.Role = viewerRole
					ctx := context.WithValue(f.ctx, app.UserCtxKey, &viewer)
					target := &entity.User{ID: 2, Role: role, Status: status}
					want := target.AllowedActions(&viewer, f.tenant)

					for _, report := range read(t, ctx) {
						for _, user := range []*entity.User{report.Reporter, report.AssignedTo, report.ResolvedBy} {
							if user == nil || user.ID != 2 || user.Role != role || user.Status != status {
								t.Fatalf("report user omitted current target identity: %+v", user)
							}
							if user.Permissions != want {
								t.Fatalf("report user permissions = %+v, want %+v", user.Permissions, want)
							}
						}
					}
				})
			}
		}
	}

	var otherTenantUserID int
	err = dbx.Connection().QueryRow(`
		INSERT INTO users (tenant_id, name, email, role, status, avatar_type, avatar_bkey, created_at)
		SELECT id, 'Other tenant reporter', '', $2, $3, 1, '', NOW()
		FROM tenants WHERE id <> $1 LIMIT 1
		RETURNING id
	`, f.tenant.ID, enum.RoleVisitor, enum.UserActive).Scan(&otherTenantUserID)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		userID any
	}{
		{name: "absent users", userID: nil},
		{name: "other tenant users", userID: otherTenantUserID},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := dbx.Connection().Exec(`
				UPDATE reports SET reporter_id = $2, assigned_to = $2, resolved_by = $2 WHERE id = $1
			`, reportID, test.userID); err != nil {
				t.Fatal(err)
			}
			for _, report := range read(t, f.ctx) {
				if report.Reporter != nil || report.AssignedTo != nil || report.ResolvedBy != nil {
					t.Fatal("report published an absent or foreign-tenant user")
				}
			}
		})
	}
}
