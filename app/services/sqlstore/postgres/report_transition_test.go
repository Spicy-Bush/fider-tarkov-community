package postgres_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestReportTransitionsPreserveClaimsAndResolutions(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	var reportID int
	if err := trx.Scalar(&reportID, `
		INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, status, created_at)
		VALUES ($1, $2, 'post', 1, 'spam', 'pending', NOW()) RETURNING id
	`, jonSnow.Tenant.ID, jonSnow.ID); err != nil {
		t.Fatal(err)
	}

	moderator := *sansaStark
	moderator.Role = enum.RoleModerator
	other := context.WithValue(sansaStarkCtx, app.UserCtxKey, &moderator)
	claim := &cmd.AssignReport{ReportID: reportID, AssignToID: jonSnow.ID}
	if err := bus.Dispatch(jonSnowCtx, claim); err != nil {
		t.Fatal(err)
	}

	for _, command := range []any{
		&cmd.AssignReport{ReportID: reportID, AssignToID: moderator.ID},
		&cmd.UnassignReport{ReportID: reportID},
	} {
		if err := bus.Dispatch(other, command); errors.Cause(err) != app.ErrConflict {
			t.Fatalf("another moderator changed a claim: %v", err)
		}
	}

	if err := bus.Dispatch(jonSnowCtx, &cmd.UnassignReport{ReportID: reportID}, claim); err != nil {
		t.Fatal(err)
	}
	resolution := &cmd.ResolveReport{ReportID: reportID, Status: enum.ReportStatusResolved, ResolutionNote: "Reviewed"}
	if err := bus.Dispatch(other, resolution); err != nil {
		t.Fatal(err)
	}

	for _, command := range []any{claim, &cmd.UnassignReport{ReportID: reportID}, resolution} {
		if err := bus.Dispatch(jonSnowCtx, command); errors.Cause(err) != app.ErrConflict {
			t.Fatalf("terminal report was overwritten: %v", err)
		}
	}

	read := &query.GetReportByID{ReportID: reportID}
	if err := bus.Dispatch(jonSnowCtx, read); err != nil {
		t.Fatal(err)
	}
	if read.Result.Status != enum.ReportStatusResolved || read.Result.ResolvedBy.ID != moderator.ID || read.Result.ResolutionNote != "Reviewed" {
		t.Fatalf("resolution changed: %+v", read.Result)
	}
	if err := bus.Dispatch(jonSnowCtx, &cmd.ResolveReport{ReportID: reportID, Status: enum.ReportStatusPending}); err == nil {
		t.Fatal("accepted an invalid resolution status")
	}
}
