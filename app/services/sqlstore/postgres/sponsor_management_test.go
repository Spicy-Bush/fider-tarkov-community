package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestSponsorManagementPages(t *testing.T) {
	f := newPostWorkflow(t)
	seedSponsorHistory(t, f, 52)
	first := &query.GetSponsorManagement{}
	if err := bus.Dispatch(f.ctx, first); err != nil {
		t.Fatal(err)
	}
	if len(first.Result.Campaigns) != 25 || len(first.Result.Creatives) != 0 || !first.Result.NextPage {
		t.Fatalf("first page loaded history: %+v", first.Result)
	}

	next := &query.GetSponsorManagement{Browse: query.SponsorBrowse{Page: 2}}
	if err := bus.Dispatch(f.ctx, next); err != nil {
		t.Fatal(err)
	}
	if len(next.Result.Campaigns) != 25 || next.Result.Campaigns[0].ID >= first.Result.Campaigns[24].ID {
		t.Fatal("booking pages overlap or are out of order")
	}

	last := &query.GetSponsorManagement{Browse: query.SponsorBrowse{Page: 3}}
	if err := bus.Dispatch(f.ctx, last); err != nil {
		t.Fatal(err)
	}
	if len(last.Result.Campaigns) != 2 || last.Result.NextPage {
		t.Fatal("last page did not end")
	}

	search := &query.GetSponsorManagement{Browse: query.SponsorBrowse{Search: "Booking 52"}}
	if err := bus.Dispatch(f.ctx, search); err != nil || len(search.Result.Campaigns) != 1 {
		t.Fatalf("could not find an older booking: %v", err)
	}

	id := first.Result.Campaigns[0].ID
	_, err := dbx.Connection().Exec(`
		INSERT INTO sponsor_creatives (tenant_id, campaign_id, data)
		SELECT $1, $2, '{"state":"draft","headline":"Older artwork"}'::jsonb FROM generate_series(1, 26)
	`, f.tenant.ID, id)
	if err != nil {
		t.Fatal(err)
	}

	detail := &query.GetSponsorManagement{Browse: query.SponsorBrowse{CampaignID: id}}
	if err := bus.Dispatch(f.ctx, detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Result.Campaigns) != 1 || len(detail.Result.Creatives) != 25 || !detail.Result.NextArtwork {
		t.Fatal("campaign detail loaded all artwork")
	}
	lastCreative := detail.Result.Creatives[24].ID
	detail.Browse.ArtworkPage = 2
	if err := bus.Dispatch(f.ctx, detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Result.Creatives) != 2 || detail.Result.NextArtwork || detail.Result.Creatives[0].ID >= lastCreative {
		t.Fatal("artwork pages overlap or do not end")
	}

	foreign := withTenant(withUser(f.ctx, tonyStark), avengersTenant)
	if err := bus.Dispatch(foreign, detail); err != app.ErrNotFound {
		t.Fatalf("another tenant read the campaign: %v", err)
	}
	anonymous := context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil))
	if err := bus.Dispatch(anonymous, first); err == nil {
		t.Fatal("anonymous reader accessed sponsorship management")
	}

	for _, role := range entity.PermissionRoles {
		if _, err := dbx.Connection().Exec("UPDATE users SET role=$1 WHERE id=$2", role, f.user.ID); err != nil {
			t.Fatal(err)
		}

		err := bus.Dispatch(f.ctx, &query.GetSponsorManagement{})
		allowed := role == enum.RoleAdministrator || role == enum.RoleCollaborator
		if (err == nil) != allowed {
			t.Fatalf("management access for %s: %v", role, err)
		}
	}
}

func seedSponsorHistory(t testing.TB, f postWorkflow, count int) {
	t.Helper()
	_, err := dbx.Connection().Exec(`
		WITH campaigns AS (
			INSERT INTO sponsor_campaigns (tenant_id, state, start_at, end_at, data)
			SELECT $1, 'booked', now()-interval '2 days', now()-interval '1 day',
			    jsonb_build_object('name', 'Booking ' || n, 'advertiser', 'Sponsor', 'bookings', '[]'::jsonb)
			FROM generate_series(1, $2) n RETURNING tenant_id, id
		)
		INSERT INTO sponsor_creatives (tenant_id, campaign_id, data)
		SELECT tenant_id, id, '{"state":"approved","headline":"Artwork","destination":"https://example.com"}'::jsonb
		FROM campaigns
	`, f.tenant.ID, count)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkSponsorManagementHistory(b *testing.B) {
	for _, sample := range []struct{ count, page int }{{25, 1}, {10000, 1}, {10000, 400}} {
		b.Run(fmt.Sprintf("bookings=%d/page=%d", sample.count, sample.page), func(b *testing.B) {
			f := newPostWorkflow(b)
			seedSponsorHistory(b, f, sample.count)
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				request := &query.GetSponsorManagement{Browse: query.SponsorBrowse{Page: sample.page}}
				if err := bus.Dispatch(f.ctx, request); err != nil {
					b.Fatal(err)
				}
				if len(request.Result.Campaigns) != 25 || len(request.Result.Creatives) != 0 {
					b.Fatal("management did not return one page of bookings")
				}
			}
		})
	}
}
