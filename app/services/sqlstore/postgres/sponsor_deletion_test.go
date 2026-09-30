package postgres_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestSponsorDeletionPreservesReportsAndSharedImages(t *testing.T) {
	f := newPostWorkflow(t)
	image := uploadMediaFixture(t, f.ctx, "shared-sponsor-image", "Shared image", "attachments/")
	campaign := sponsorCampaign(t, f.ctx, 100)
	var artwork []*cmd.SaveSponsorCampaign

	for i := range 2 {
		save := &cmd.SaveSponsorCampaign{
			SubmissionID: fmt.Sprintf("artwork-%d", i),
			Campaign:     *campaign,
			Creative: &entity.SponsorCreative{
				State: "approved",
				SponsorArtwork: entity.SponsorArtwork{
					Headline:    fmt.Sprintf("Artwork %d", i),
					Destination: "https://example.com/offer",
					ImageKey:    image.BlobKey,
					LogoKey:     image.BlobKey,
				},
			},
		}
		if err := bus.Dispatch(f.ctx, save); err != nil {
			t.Fatal(err)
		}

		campaign = save.Result
		artwork = append(artwork, save)
	}

	placement := &cmd.SaveSponsorPlacement{SubmissionID: "enable-strip", Placement: entity.SponsorPlacements[0]}
	placement.Placement.Enabled = true
	placement.Placement.Empty = "none"
	allocation := &cmd.AllocateSponsors{
		PageID: rand.String(32), ExpiresAt: time.Now().Add(time.Hour),
		Context:       entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
		Opportunities: []entity.SponsorOpportunity{{InstanceID: "strip", PlacementID: "strip_desktop"}},
	}
	if err := bus.Dispatch(f.ctx, placement, allocation); err != nil {
		t.Fatal(err)
	}
	if allocation.Result["strip"].Creative.Headline != "Artwork 1" {
		t.Fatal("latest artwork was not serving before deletion")
	}

	for range 2 {
		response, err := f.requestWithParams(api.DeleteSponsorCreative(), http.MethodDelete,
			"/api/sponsorship/artwork", "", web.StringMap{"id": fmt.Sprint(artwork[1].CreativeResult.ID)})
		if err != nil || response.Code != http.StatusNoContent {
			t.Fatalf("artwork deletion or retry failed: %d, %v", response.Code, err)
		}
	}

	allocation.PageID = rand.String(32)
	if err := bus.Dispatch(f.ctx, allocation); err != nil {
		t.Fatal(err)
	}
	if allocation.Result["strip"].Creative.Headline != "Artwork 0" {
		t.Fatal("remaining approved artwork did not take over")
	}

	management := &query.GetSponsorManagement{Browse: query.SponsorBrowse{CampaignID: campaign.ID}}
	if err := bus.Dispatch(f.ctx, management); err != nil || len(management.Result.Creatives) != 1 {
		t.Fatalf("deleted artwork remains in management: %+v, %v", management.Result, err)
	}
	if count := workflowCount(t, `SELECT count(*) FROM media_references WHERE tenant_id=$1 AND kind='sponsor' AND key=$2`, f.tenant.ID, image.BlobKey); count != 2 {
		t.Fatalf("deleting artwork damaged shared image references: %d", count)
	}

	staleEdit := &cmd.SaveSponsorCampaign{
		SubmissionID: "stale-artwork-edit", Campaign: *campaign, Creative: artwork[1].CreativeResult,
	}
	if err := bus.Dispatch(f.ctx, staleEdit); err != app.ErrNotFound {
		t.Fatalf("stale editor restored deleted artwork: %v", err)
	}
	if err := bus.Dispatch(f.ctx, artwork[1]); err != app.ErrNotFound {
		t.Fatalf("replayed save restored deleted artwork: %v", err)
	}

	for range 2 {
		response, err := f.requestWithParams(api.DeleteSponsorCampaign(), http.MethodDelete,
			"/api/sponsorship/bookings", "", web.StringMap{"id": fmt.Sprint(campaign.ID)})
		if err != nil || response.Code != http.StatusNoContent {
			t.Fatalf("campaign deletion or retry failed: %d, %v", response.Code, err)
		}
	}

	if err := bus.Dispatch(f.ctx, allocation); err != nil || allocation.Result["strip"].Kind != "none" {
		t.Fatalf("deleted campaign still serves: %+v, %v", allocation.Result, err)
	}
	if err := bus.Dispatch(f.ctx, management); err != app.ErrNotFound {
		t.Fatalf("deleted campaign detail is available: %v", err)
	}
	management.Browse.CampaignID = 0
	if err := bus.Dispatch(f.ctx, management); err != nil || len(management.Result.Campaigns) != 0 {
		t.Fatalf("deleted campaign remains in bookings: %+v, %v", management.Result, err)
	}

	report := &query.GetSponsorReport{CampaignID: campaign.ID}
	if err := bus.Dispatch(f.ctx, report); err != nil || len(report.Allocations) != 2 {
		t.Fatalf("deletion lost historical reporting: %+v, %v", report, err)
	}
	if len(report.Changes) != 4 || report.Changes[3].State != "deleted" {
		t.Fatalf("deletion retries duplicated booking history: %+v", report.Changes)
	}
	for _, row := range report.Allocations {
		if row.Allocated != 1 {
			t.Fatalf("deletion changed historical delivery: %+v", row)
		}
	}

	if count := workflowCount(t, `SELECT count(*) FROM media_references WHERE tenant_id=$1 AND kind='sponsor' AND key=$2`, f.tenant.ID, image.BlobKey); count != 0 {
		t.Fatalf("deleted artwork still protects the image: %d", count)
	}
	if count := workflowCount(t, `SELECT count(*) FROM blobs WHERE tenant_id=$1 AND key=$2`, f.tenant.ID, image.BlobKey); count != 1 {
		t.Fatal("deleting a campaign removed its image from the library")
	}
	staleEdit.SubmissionID = "stale-campaign-edit"
	staleEdit.Creative = nil
	if err := bus.Dispatch(f.ctx, staleEdit); err != app.ErrNotFound {
		t.Fatalf("stale editor restored a deleted campaign: %v", err)
	}

	sponsorCampaign(t, f.ctx, 100)
}

func TestSponsorDeletionAuthorisation(t *testing.T) {
	f := newPostWorkflow(t)
	seedSponsorHistory(t, f, 1)
	var campaignID, artworkID int
	if err := dbx.Connection().QueryRow(`SELECT campaign_id, id FROM sponsor_creatives WHERE tenant_id=$1`, f.tenant.ID).Scan(&campaignID, &artworkID); err != nil {
		t.Fatal(err)
	}

	otherTenant := &query.GetTenantByDomain{Domain: "avengers"}
	if err := bus.Dispatch(f.ctx, otherTenant); err != nil {
		t.Fatal(err)
	}
	foreign := withTenant(f.ctx, otherTenant.Result)
	otherUser := &query.GetUserByEmail{Email: "tony.stark@avengers.com"}
	if err := bus.Dispatch(foreign, otherUser); err != nil {
		t.Fatal(err)
	}
	foreign = withUser(foreign, otherUser.Result)

	for _, kind := range []string{"campaign", "artwork"} {
		var command any = &cmd.DeleteSponsorCampaign{ID: campaignID}
		if kind == "artwork" {
			command = &cmd.DeleteSponsorCreative{ID: artworkID}
		}

		anonymous := context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil))
		if err := bus.Dispatch(anonymous, command); err == nil {
			t.Fatalf("anonymous user deleted %s", kind)
		}
		if err := bus.Dispatch(foreign, command); err != nil {
			t.Fatalf("absent %s deletion was not idempotent: %v", kind, err)
		}
	}
	if count := workflowCount(t, `SELECT count(*) FROM sponsor_campaigns WHERE id=$1 AND state='booked'`, campaignID); count != 1 {
		t.Fatal("foreign tenant deleted the campaign")
	}
	if count := workflowCount(t, `SELECT count(*) FROM sponsor_creatives WHERE id=$1 AND data->>'state'='approved'`, artworkID); count != 1 {
		t.Fatal("foreign tenant deleted the artwork")
	}

	for _, role := range entity.PermissionRoles {
		if _, err := dbx.Connection().Exec("UPDATE users SET role=$1 WHERE id=$2", role, f.user.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := dbx.Connection().Exec(`UPDATE sponsor_campaigns SET state='booked' WHERE id=$1`, campaignID); err != nil {
			t.Fatal(err)
		}
		if _, err := dbx.Connection().Exec(`UPDATE sponsor_creatives SET data=jsonb_set(data,'{state}','"approved"') WHERE id=$1`, artworkID); err != nil {
			t.Fatal(err)
		}

		allowed := role == enum.RoleAdministrator || role == enum.RoleCollaborator
		artworkErr := bus.Dispatch(f.ctx, &cmd.DeleteSponsorCreative{ID: artworkID})
		campaignErr := bus.Dispatch(f.ctx, &cmd.DeleteSponsorCampaign{ID: campaignID})
		if (artworkErr == nil) != allowed || (campaignErr == nil) != allowed {
			t.Fatalf("%s deletion permission: artwork=%v, campaign=%v", role, artworkErr, campaignErr)
		}
		deleted := workflowCount(t, `SELECT count(*) FROM sponsor_campaigns WHERE id=$1 AND state='deleted'`, campaignID)
		if (deleted == 1) != allowed {
			t.Fatalf("%s changed the campaign despite rejection", role)
		}
	}

	for _, handler := range []web.HandlerFunc{api.DeleteSponsorCampaign(), api.DeleteSponsorCreative()} {
		for _, id := range []string{"0", "-1", "invalid"} {
			response, err := f.requestWithParams(handler, http.MethodDelete, "/api/sponsorship", "", web.StringMap{"id": id})
			if err != nil || response.Code != http.StatusBadRequest {
				t.Fatalf("invalid deletion ID %q: %d, %v", id, response.Code, err)
			}
		}
	}
}
