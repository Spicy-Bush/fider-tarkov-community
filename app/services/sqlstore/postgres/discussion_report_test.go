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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestDiscussionPrivatePageReportOperations(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	page := &cmd.CreatePage{
		Title: "Restricted report",
		Slug: "restricted-report",
		Content: "Private Page",
		Status: entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPrivate,
		AllowedRoles: []string{"visitor"},
		AllowComments: true,
	}
	if err := bus.Dispatch(jonSnowCtx, page, &cmd.CreateReportReason{Title: "spam"}); err != nil {
		t.Fatal(err)
	}
	comment := &cmd.CreateComment{
		PageID:       page.Result.ID,
		Content:      "Reported comment",
		SubmissionID: "reported-comment",
	}
	if err := bus.Dispatch(jonSnowCtx, comment); err != nil {
		t.Fatal(err)
	}
	report := &cmd.CreateReport{ReportedType: enum.ReportTypeComment, ReportedID: comment.Result.ID, Reason: "spam"}
	if err := bus.Dispatch(aryaStarkCtx, report); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute("UPDATE pages SET allowed_roles = '[]' WHERE id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}
	moderator := *sansaStark
	moderator.Role = enum.RoleModerator
	ctx := context.WithValue(sansaStarkCtx, app.UserCtxKey, &moderator)

	read := func(wantVisible bool) {
		t.Helper()
		list := &query.ListReports{Page: 1, PerPage: 100}
		pending := &query.CountPendingReports{}
		if err := bus.Dispatch(ctx, list, pending); err != nil {
			t.Fatal(err)
		}
		want := 0
		if wantVisible {
			want = 1
		}
		if len(list.Result) != want || list.Total != want || pending.Result != want {
			t.Fatalf("visible=%v: list=%d total=%d pending=%d", wantVisible, len(list.Result), list.Total, pending.Result)
		}
		get := &query.GetReportByID{ReportID: report.Result}
		err := bus.Dispatch(ctx, get)
		if wantVisible {
			if err != nil || get.Result.ID != report.Result || get.Result.PageSlug != page.Result.Slug {
				t.Fatalf("visible report lookup failed: %v %+v", err, get.Result)
			}
		} else if errors.Cause(err) != app.ErrNotFound {
			t.Fatalf("restricted report lookup returned %v", err)
		}
	}
	read(false)

	operations := []struct {
		name string
		command any
	}{
		{name: "assign", command: &cmd.AssignReport{ReportID: report.Result, AssignToID: moderator.ID}},
		{name: "unassign", command: &cmd.UnassignReport{ReportID: report.Result}},
		{name: "resolve", command: &cmd.ResolveReport{ReportID: report.Result, Status: enum.ReportStatusResolved, ResolutionNote: "Reviewed"}},
		{name: "delete", command: &cmd.DeleteReport{ReportID: report.Result}},
	}
	for _, operation := range operations {
		t.Run("denied "+operation.name, func(t *testing.T) {
			if _, err := trx.Execute("UPDATE reports SET status = 'in_review', assigned_to = $1, assigned_at = NOW() WHERE id = $2", jonSnow.ID, report.Result); err != nil {
				t.Fatal(err)
			}
			var before, after string
			if err := trx.Scalar(&before, "SELECT row_to_json(r)::text FROM reports r WHERE id = $1", report.Result); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(ctx, operation.command); errors.Cause(err) != app.ErrNotFound {
				t.Errorf("restricted mutation returned %v", err)
			}
			if err := trx.Scalar(&after, "SELECT row_to_json(r)::text FROM reports r WHERE id = $1", report.Result); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Error("denied mutation changed the persisted report")
			}
		})
	}

	if _, err := trx.Execute("UPDATE pages SET allowed_roles = '[\"moderator\"]' WHERE id = $1", page.Result.ID); err != nil {
		t.Fatal(err)
	}
	read(true)
	for _, operation := range operations {
		if err := bus.Dispatch(ctx, operation.command); err != nil {
			t.Fatalf("%s failed after access was granted: %v", operation.name, err)
		}
		get := &query.GetReportByID{ReportID: report.Result}
		err := bus.Dispatch(ctx, get)
		if operation.name == "delete" {
			if errors.Cause(err) != app.ErrNotFound {
				t.Fatalf("deleted report still readable: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		switch operation.name {
		case "assign":
			if get.Result.AssignedTo == nil || get.Result.AssignedTo.ID != moderator.ID || get.Result.Status != enum.ReportStatusInReview {
				t.Fatal("assignment did not persist the moderator and status")
			}
		case "unassign":
			if get.Result.AssignedTo != nil || get.Result.Status != enum.ReportStatusPending {
				t.Fatal("unassignment did not clear the assignee and reset status")
			}
		case "resolve":
			if get.Result.Status != enum.ReportStatusResolved || get.Result.ResolvedBy == nil || get.Result.ResolvedBy.ID != moderator.ID || get.Result.ResolutionNote != "Reviewed" {
				t.Fatal("resolution did not persist status, actor and note")
			}
		}
	}
}

func TestDiscussionReportVisibilityMatchesPagePolicy(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	page := &cmd.CreatePage{
		Title: "Visibility matrix",
		Slug: "visibility-matrix",
		Content: "Page content",
		Status: entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
		AllowComments: true,
	}
	if err := bus.Dispatch(jonSnowCtx, page, &cmd.CreateReportReason{Title: "spam"}); err != nil {
		t.Fatal(err)
	}
	comment := &cmd.CreateComment{
		PageID:       page.Result.ID,
		Content:      "Comment",
		SubmissionID: "comment",
	}
	if err := bus.Dispatch(jonSnowCtx, comment); err != nil {
		t.Fatal(err)
	}
	report := &cmd.CreateReport{ReportedType: enum.ReportTypeComment, ReportedID: comment.Result.ID, Reason: "spam"}
	if err := bus.Dispatch(aryaStarkCtx, report); err != nil {
		t.Fatal(err)
	}

	statuses := []entity.PageStatus{entity.PageStatusDraft, entity.PageStatusPublished, entity.PageStatusUnpublished, entity.PageStatusScheduled}
	visibilities := []entity.PageVisibility{entity.PageVisibilityPublic, entity.PageVisibilityUnlisted, entity.PageVisibilityPrivate}
	roles := []enum.Role{0, enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}
	for _, status := range statuses {
		for _, visibility := range visibilities {
			for _, allowed := range []bool{false, true} {
				allowedJSON := "[]"
				page.Result.AllowedRoles = nil
				if allowed {
					allowedJSON = `["visitor","helper","moderator"]`
					page.Result.AllowedRoles = []string{"visitor", "helper", "moderator"}
				}
				page.Result.Status = status
				page.Result.Visibility = visibility
				if _, err := trx.Execute("UPDATE pages SET status = $1, visibility = $2, allowed_roles = $3 WHERE id = $4", status, visibility, allowedJSON, page.Result.ID); err != nil {
					t.Fatal(err)
				}
				for _, role := range roles {
					t.Run(fmt.Sprintf("%s/%s/allowed=%v/role=%d", status, visibility, allowed, role), func(t *testing.T) {
						var viewer *entity.User
						if role != 0 {
							copy := *sansaStark
							copy.Role = role
							viewer = &copy
						}
						ctx := context.WithValue(sansaStarkCtx, app.UserCtxKey, viewer)
						want := role == enum.RoleAdministrator || role == enum.RoleCollaborator ||
							(status == entity.PageStatusPublished && (visibility != entity.PageVisibilityPrivate || (role != 0 && allowed)))
						if page.Result.CanView(viewer) != want {
							t.Fatal("entity visibility disagrees with the expected role/status policy")
						}
						get := &query.GetReportByID{ReportID: report.Result}
						err := bus.Dispatch(ctx, get)
						if want && err != nil {
							t.Fatalf("SQL concealed a visible owner: %v", err)
						}
						if !want && errors.Cause(err) != app.ErrNotFound {
							t.Fatalf("SQL admitted an inaccessible owner: %v", err)
						}
						list := &query.ListReports{Page: 1, PerPage: 100}
						pending := &query.CountPendingReports{}
						if err := bus.Dispatch(ctx, list, pending); err != nil {
							t.Fatal(err)
						}
						wantCount := 0
						if want {
							wantCount = 1
						}
						if len(list.Result) != wantCount || list.Total != wantCount || pending.Result != wantCount {
							t.Fatalf("list/count visibility differs: list=%d total=%d pending=%d want=%d", len(list.Result), list.Total, pending.Result, wantCount)
						}
					})
				}
			}
		}
	}
}
