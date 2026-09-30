package postgres_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
)

func TestSponsorCampaignWithImagesIsAtomicAndRecoverable(t *testing.T) {
	f := newPostWorkflow(t)
	admin := f.ctx

	var content bytes.Buffer
	if err := png.Encode(&content, image.NewNRGBA(image.Rect(0, 0, 32, 20))); err != nil {
		t.Fatal(err)
	}

	upload := &cmd.UploadSponsorImage{
		Name:         "Sponsor image.png",
		Content:      content.Bytes(),
		SubmissionID: "sponsor-image-recovery",
	}
	if err := bus.Dispatch(admin, upload); err != nil {
		t.Fatal(err)
	}
	key := upload.Result.BlobKey

	if err := bus.Dispatch(admin, upload); err != nil || upload.Result.BlobKey != key {
		t.Fatalf("upload retry changed the image: %+v, %v", upload.Result, err)
	}

	now := time.Now().UTC()
	create := &cmd.SaveSponsorCampaign{
		SubmissionID: "sponsor-booking-with-images",
		Campaign: entity.SponsorCampaign{
			Name:          "Booking and artwork",
			Advertiser:    "Local retailer",
			State:         "booked",
			StartAt:       now,
			EndAt:         now.Add(24 * time.Hour),
			PageTypes:     []string{"home"},
			Bookings:      []entity.SponsorBooking{{PlacementID: "home_desktop", Share: 50}},
			Currency:      "AUD",
			PaymentStatus: "unpaid",
		},
		Creative: &entity.SponsorCreative{
			State: "approved",
			SponsorArtwork: entity.SponsorArtwork{
				Headline:    "Community support",
				Destination: "https://example.com/offer",
				ImageKey:    key,
				LogoKey:     "files/unavailable-logo",
			},
		},
	}
	if err := bus.Dispatch(admin, create); err == nil {
		t.Fatal("accepted a creative with an unavailable logo")
	}

	management := &query.GetSponsorManagement{}
	if err := bus.Dispatch(admin, management); err != nil {
		t.Fatal(err)
	}
	if len(management.Result.Campaigns) != 0 || len(management.Result.Creatives) != 0 {
		t.Fatal("a failed creative left part of the booking committed")
	}

	create.Creative.LogoKey = key
	if err := bus.Dispatch(admin, create); err != nil {
		t.Fatal(err)
	}
	bookingID, creativeID := create.Result.ID, create.CreativeResult.ID

	if err := bus.Dispatch(admin, create); err != nil {
		t.Fatal(err)
	}
	if create.Result.ID != bookingID || create.CreativeResult.ID != creativeID || create.CreativeResult.CampaignID != bookingID {
		t.Fatal("retry created a second booking or creative")
	}

	var references int
	err := dbx.Connection().QueryRow(`
		SELECT count(*) FROM media_references
		WHERE tenant_id=$1 AND kind='sponsor' AND id=$2 AND key=$3 AND scope='active'
	`, f.tenant.ID, creativeID, key).Scan(&references)
	if err != nil || references != 2 {
		t.Fatalf("expected both artwork fields to protect the upload: references=%d error=%v", references, err)
	}

	create.Creative.Headline = "Different operation"
	if err := bus.Dispatch(admin, create); err != app.ErrConflict {
		t.Fatalf("changed creative reused the accepted save: %v", err)
	}
}

func TestSponsorClickReporting(t *testing.T) {
	f := newPostWorkflow(t)
	campaign := sponsorCampaign(t, f.ctx, 100)
	destination := "https://example.com/offer?utm_campaign=launch%20week#details"
	creative := &cmd.SaveSponsorCampaign{
		SubmissionID: "click-artwork",
		Campaign:     *campaign,
		Creative: &entity.SponsorCreative{
			CampaignID: campaign.ID, State: "approved",
			SponsorArtwork: entity.SponsorArtwork{Headline: "Sponsor", Destination: destination},
		},
	}
	placement := &cmd.SaveSponsorPlacement{SubmissionID: "click-placement", Placement: entity.SponsorPlacements[0]}
	placement.Placement.Enabled = true
	placement.Placement.Empty = "none"
	if err := bus.Dispatch(f.ctx, creative, placement); err != nil {
		t.Fatal(err)
	}

	allocation := &cmd.AllocateSponsors{
		Context:       entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
		Opportunities: []entity.SponsorOpportunity{{InstanceID: "strip", PlacementID: placement.Placement.ID}},
	}
	if err := bus.Dispatch(f.ctx, allocation); err != nil {
		t.Fatal(err)
	}

	link := allocation.Result["strip"].ClickURL
	if link == "" {
		t.Fatal("allocated sponsorship has no click link")
	}

	response, err := f.requestWithParams(handlers.SponsorClick(), http.MethodGet, link, "", nil)
	if err != nil || response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != destination {
		t.Fatalf("click did not retain its destination: %d %s, %v", response.Code, response.Header().Get("Location"), err)
	}

	report := &query.GetSponsorReport{CampaignID: campaign.ID}
	if err := bus.Dispatch(f.ctx, report); err != nil {
		t.Fatal(err)
	}

	if len(report.Allocations) != 1 || report.Allocations[0].Clicks != 1 || report.Allocations[0].Allocated != 1 {
		t.Fatalf("click changed delivery counts or was not attributed: %+v", report.Allocations)
	}
}

func sponsorCampaign(t *testing.T, ctx context.Context, share int) *entity.SponsorCampaign {
	t.Helper()
	now := time.Now().UTC()
	command := &cmd.SaveSponsorCampaign{
		SubmissionID: rand.String(32),
		Campaign: entity.SponsorCampaign{
			Name: "Community booking", Advertiser: "Retailer", Category: "retail", State: "booked",
			StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
			PageTypes: []string{"home"}, Currency: "AUD", PaymentStatus: "unpaid",
			Bookings: []entity.SponsorBooking{{PlacementID: "strip_desktop", Share: share}},
		},
	}
	if err := bus.Dispatch(ctx, command); err != nil {
		t.Fatal(err)
	}

	return command.Result
}

func TestSponsorArtworkReplacement(t *testing.T) {
	f := newPostWorkflow(t)
	first := uploadMediaFixture(t, f.ctx, "first-sponsor-image", "First image", "attachments/")
	second := uploadMediaFixture(t, f.ctx, "second-sponsor-image", "Second image", "attachments/")
	campaign := sponsorCampaign(t, f.ctx, 100)
	create := &cmd.SaveSponsorCampaign{
		SubmissionID: "approved-artwork",
		Campaign:     *campaign,
		Creative: &entity.SponsorCreative{
			CampaignID: campaign.ID,
			State:      "approved",
			SponsorArtwork: entity.SponsorArtwork{
				ImageKey:    first.BlobKey,
				Destination: "https://example.com",
			},
		},
	}
	if err := bus.Dispatch(f.ctx, create); err != nil {
		t.Fatal(err)
	}

	replacement := *create.CreativeResult
	replacement.ImageKey = second.BlobKey
	replacement.State = "review"
	edit := &cmd.SaveSponsorCampaign{
		SubmissionID: "replace-booking-artwork",
		Campaign:     *create.Result,
		Creative:     &replacement,
	}
	if err := bus.Dispatch(f.ctx, edit); err != nil {
		t.Fatal(err)
	}
	if edit.CreativeResult.ID == create.CreativeResult.ID {
		t.Fatal("replacement overwrote the approved artwork")
	}

	replacementID := edit.CreativeResult.ID
	if err := bus.Dispatch(f.ctx, edit); err != nil || edit.CreativeResult.ID != replacementID {
		t.Fatalf("lost save response duplicated the replacement: %v", err)
	}

	placement := &cmd.SaveSponsorPlacement{SubmissionID: rand.String(32),
		Placement: entity.SponsorPlacement{ID: "strip_desktop", Enabled: true, Position: "navigation", Empty: "none"},
	}
	selection := &cmd.AllocateSponsors{
		Context:       entity.SponsorContext{PageType: "home", Device: "desktop", Language: "en"},
		Opportunities: []entity.SponsorOpportunity{{InstanceID: "strip", PlacementID: "strip_desktop"}},
	}
	if err := bus.Dispatch(f.ctx, placement, selection); err != nil {
		t.Fatal(err)
	}
	if selection.Result["strip"].Creative.ImageKey != first.BlobKey {
		t.Fatal("pending replacement interrupted approved delivery")
	}

	approved := *edit.CreativeResult
	approved.State = "approved"
	publish := &cmd.SaveSponsorCampaign{
		SubmissionID: "approve-replacement",
		Campaign:     *edit.Result,
		Creative:     &approved,
	}
	if err := bus.Dispatch(f.ctx, publish, selection); err != nil {
		t.Fatal(err)
	}
	if selection.Result["strip"].Creative.ImageKey != second.BlobKey {
		t.Fatal("approved replacement was not delivered")
	}

	for _, failure := range []string{"stale artwork", "unavailable image", "wrong campaign"} {
		t.Run(failure, func(t *testing.T) {
			artwork := *publish.CreativeResult
			switch failure {
			case "stale artwork":
				artwork.Revision--
			case "unavailable image":
				artwork.ImageKey = "attachments/missing"
			case "wrong campaign":
				artwork.ID = create.CreativeResult.ID
				artwork.CampaignID++
			}

			failed := &cmd.SaveSponsorCampaign{
				SubmissionID: rand.String(32), Campaign: *publish.Result, Creative: &artwork,
			}
			if failure == "wrong campaign" {
				failed.Campaign.ID = artwork.CampaignID
			}
			if err := bus.Dispatch(f.ctx, failed); err == nil {
				t.Fatal("accepted invalid artwork")
			}
		})
	}

	invalid := *publish.CreativeResult
	invalid.ImageKey = "attachments/missing"
	failedEdit := &cmd.SaveSponsorCampaign{SubmissionID: "failed-edit", Campaign: *publish.Result, Creative: &invalid}
	failedEdit.Campaign.Name = "Must not be saved"
	if err := bus.Dispatch(f.ctx, failedEdit); err == nil {
		t.Fatal("accepted an unavailable replacement image")
	}

	management := &query.GetSponsorManagement{Browse: query.SponsorBrowse{CampaignID: campaign.ID}}
	if err := bus.Dispatch(f.ctx, management); err != nil {
		t.Fatal(err)
	}
	if len(management.Result.Creatives) != 2 || management.Result.Campaigns[0].Name != campaign.Name || management.Result.Campaigns[0].Revision != publish.Result.Revision {
		t.Fatal("failed edit changed the campaign or artwork")
	}
}

func TestSponsorDeliverySharesAndRecovery(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	admin := withTenant(withUser(ctx, jonSnow), demoTenant)
	campaign := sponsorCampaign(t, admin, 50)
	creative := &cmd.SaveSponsorCampaign{
		SubmissionID: rand.String(32),
		Campaign:     *campaign,
		Creative: &entity.SponsorCreative{
			CampaignID: campaign.ID, State: "approved",
			SponsorArtwork: entity.SponsorArtwork{Headline: "Support the community", Destination: "https://example.com/offer"},
		},
	}
	placement := &cmd.SaveSponsorPlacement{SubmissionID: rand.String(32),
		Placement: entity.SponsorPlacement{ID: "strip_desktop", Enabled: true, Position: "navigation", Empty: "none"},
	}
	if err := bus.Dispatch(admin, creative, placement); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(admin, creative); err != nil {
		t.Fatalf("lost creative response could not recover: %v", err)
	}

	allocated := 0
	for n := 0; n < 100; n++ {
		request := &cmd.AllocateSponsors{
			Context:       entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
			Opportunities: []entity.SponsorOpportunity{{InstanceID: "strip", PlacementID: "strip_desktop"}},
		}
		if err := bus.Dispatch(demoTenantCtx, request); err != nil {
			t.Fatal(err)
		}

		if request.Result["strip"].Kind == "sponsor" {
			allocated++
		}
	}

	if allocated != 50 {
		t.Fatalf("a 50 percent booking received %d of 100 allocations", allocated)
	}

	report := &query.GetSponsorReport{CampaignID: campaign.ID}
	if err := bus.Dispatch(admin, report); err != nil {
		t.Fatal(err)
	}
	if len(report.Allocations) != 1 || report.Allocations[0].Eligible != 100 || report.Allocations[0].Allocated != 50 {
		t.Fatalf("unexpected allocation report: %+v", report.Allocations)
	}

	paused := *creative.Result
	paused.State = "paused"
	pause := &cmd.SaveSponsorCampaign{Campaign: paused, SubmissionID: rand.String(32)}
	if err := bus.Dispatch(admin, pause); err != nil {
		t.Fatal(err)
	}

	request := &cmd.AllocateSponsors{
		Context:       entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
		Opportunities: []entity.SponsorOpportunity{{InstanceID: "strip", PlacementID: "strip_desktop"}},
	}
	if err := bus.Dispatch(demoTenantCtx, request); err != nil || request.Result["strip"].Kind != "none" {
		t.Fatalf("paused campaign delivered: %+v, %v", request.Result, err)
	}

	if err := bus.Dispatch(admin, pause); err != nil || pause.Result.Revision != creative.Result.Revision+1 {
		t.Fatalf("retry did not recover the saved booking: %+v, %v", pause.Result, err)
	}
}

func TestSponsorOwnershipAndCapacity(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	admin := withTenant(withUser(ctx, jonSnow), demoTenant)
	campaign := sponsorCampaign(t, admin, 60)
	other := *campaign
	other.ID = 0
	other.Advertiser = "Another retailer"
	command := &cmd.SaveSponsorCampaign{Campaign: other, SubmissionID: rand.String(32)}
	if err := bus.Dispatch(admin, command); err == nil {
		t.Fatal("accepted overbooking")
	}

	foreign := withTenant(withUser(ctx, tonyStark), avengersTenant)
	creative := &cmd.SaveSponsorCampaign{
		SubmissionID: rand.String(32),
		Campaign:     *campaign,
		Creative: &entity.SponsorCreative{
			CampaignID: campaign.ID, State: "approved",
			SponsorArtwork: entity.SponsorArtwork{Headline: "Invalid owner", Destination: "https://example.com"},
		},
	}
	if err := bus.Dispatch(foreign, creative); err != app.ErrNotFound {
		t.Fatalf("cross-tenant campaign returned %v", err)
	}
}

func TestSponsorImageAcrossStripAndFeed(t *testing.T) {
	f := newPostWorkflow(t)
	image := uploadMediaFixture(t, f.ctx, "shared-sponsor-artwork", "Artwork", "attachments/")
	campaign := sponsorCampaign(t, f.ctx, 100)
	campaign.Bookings = append(campaign.Bookings,
		entity.SponsorBooking{PlacementID: "feed_desktop", Share: 25},
		entity.SponsorBooking{PlacementID: "feed_mobile", Share: 75},
	)
	crop := entity.SponsorImageCrop{X: 30, Y: 70, Zoom: 1.5}
	save := &cmd.SaveSponsorCampaign{
		SubmissionID: "shared-artwork-booking",
		Campaign:     *campaign,
		Creative: &entity.SponsorCreative{
			CampaignID: campaign.ID,
			State:      "approved",
			SponsorArtwork: entity.SponsorArtwork{
				ImageKey:    image.BlobKey,
				Destination: "https://example.com/offer",
				BannerCrop:  &crop,
			},
		},
	}
	if err := bus.Dispatch(f.ctx, save); err != nil {
		t.Fatal(err)
	}

	for _, placement := range entity.SponsorPlacements {
		if placement.ID != "strip_desktop" && placement.Position != "feed" {
			continue
		}

		placement.Enabled = true
		placement.Empty = "adsense"
		placement.AdSenseSlotID = "1234567890"
		if err := bus.Dispatch(f.ctx, &cmd.SaveSponsorPlacement{SubmissionID: rand.String(32), Placement: placement}); err != nil {
			t.Fatal(err)
		}

		allocated := 0
		for batch := 0; batch < 4; batch++ {
			request := &cmd.AllocateSponsors{
				Context: entity.SponsorContext{PageType: "home", Device: placement.Device, Language: "en"},
			}
			for i := 0; i < 25; i++ {
				request.Opportunities = append(request.Opportunities, entity.SponsorOpportunity{
					InstanceID:  fmt.Sprintf("slot-%d-%d", batch, i),
					PlacementID: placement.ID,
				})
			}

			if err := bus.Dispatch(f.ctx, request); err != nil {
				t.Fatal(err)
			}

			for _, selection := range request.Result {
				if selection.Kind != "sponsor" {
					if selection.Kind != "adsense" || selection.Placement.AdSenseSlotID != "1234567890" {
						t.Fatalf("unbooked share lost its AdSense unit: %+v", selection)
					}
					continue
				}

				allocated++
				if selection.Creative.ImageKey != image.BlobKey || selection.Creative.BannerCrop == nil || *selection.Creative.BannerCrop != crop {
					t.Fatalf("%s changed the saved artwork or crop: %+v", placement.ID, selection.Creative)
				}
			}
		}

		want := map[string]int{"strip_desktop": 100, "feed_desktop": 25, "feed_mobile": 75}[placement.ID]
		if allocated != want {
			t.Fatalf("%s allocated %d of 100 opportunities, expected %d", placement.ID, allocated, want)
		}
	}
}

func BenchmarkSponsorCampaignWithImage(b *testing.B) {
	f := newPostWorkflow(b)
	stored := uploadMediaFixture(b, f.ctx, "benchmark-artwork", "Artwork", "attachments/")
	now := time.Now().UTC()
	campaign := entity.SponsorCampaign{
		Name:          "Image booking benchmark",
		Advertiser:    "Local sponsor",
		State:         "draft",
		StartAt:       now,
		EndAt:         now.Add(24 * time.Hour),
		PageTypes:     []string{"home"},
		Bookings:      []entity.SponsorBooking{{PlacementID: "home_desktop", Share: 50}},
		Currency:      "AUD",
		PaymentStatus: "unpaid",
	}
	creative := entity.SponsorCreative{
		State: "draft",
		SponsorArtwork: entity.SponsorArtwork{
			Headline:    "Community sponsor",
			Destination: "https://example.com/offer",
			ImageKey:    stored.BlobKey,
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		save := &cmd.SaveSponsorCampaign{
			SubmissionID: rand.String(32),
			Campaign:     campaign,
			Creative:     &creative,
		}
		if err := bus.Dispatch(f.ctx, save); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSponsorConcurrentEditsMergeAndRecover(t *testing.T) {
	f := newPostWorkflow(t)
	baseline := sponsorCampaign(t, f.ctx, 25)
	first := &cmd.SaveSponsorCampaign{Campaign: *baseline, SubmissionID: "edit-name"}
	first.Campaign.Name = "Renamed booking"
	if err := bus.Dispatch(f.ctx, first); err != nil {
		t.Fatal(err)
	}

	independent := &cmd.SaveSponsorCampaign{Campaign: *baseline, BaseCampaign: baseline, SubmissionID: "edit-notes"}
	independent.Campaign.Notes = "Artwork approved by sponsor"
	if err := bus.Dispatch(f.ctx, independent); err != nil {
		t.Fatal(err)
	}

	if independent.Result.Name != first.Campaign.Name || independent.Result.Notes != independent.Campaign.Notes || independent.Conflicts.Any() {
		t.Fatalf("independent edits were lost: %+v, conflicts=%v", independent.Result, independent.Conflicts)
	}

	revision := independent.Result.Revision
	if err := bus.Dispatch(f.ctx, independent); err != nil || independent.Result.Revision != revision {
		t.Fatalf("lost acknowledgement repeated the edit: %+v, %v", independent.Result, err)
	}

	conflicting := &cmd.SaveSponsorCampaign{Campaign: *baseline, BaseCampaign: baseline, SubmissionID: "conflicting-name"}
	conflicting.Campaign.Name = "My booking name"
	conflicting.Campaign.Currency = "NZD"
	if err := bus.Dispatch(f.ctx, conflicting); err != nil {
		t.Fatal(err)
	}

	if len(conflicting.Conflicts.Campaign) != 1 || conflicting.Conflicts.Campaign[0] != "name" {
		t.Fatalf("wrong conflicts: %v", conflicting.Conflicts)
	}

	if conflicting.Result.Revision != revision || conflicting.Result.Currency != "AUD" || conflicting.DraftCampaign.Currency != "NZD" || conflicting.DraftCampaign.Notes != independent.Campaign.Notes {
		t.Fatalf("conflict lost work or partially committed: saved=%+v draft=%+v", conflicting.Result, conflicting.DraftCampaign)
	}

	resolved := &cmd.SaveSponsorCampaign{
		Campaign: *conflicting.DraftCampaign, BaseCampaign: conflicting.Result, SubmissionID: "resolved-name",
	}
	if err := bus.Dispatch(f.ctx, resolved); err != nil {
		t.Fatal(err)
	}

	if resolved.Result.Name != "My booking name" || resolved.Result.Currency != "NZD" || resolved.Result.Notes != independent.Campaign.Notes {
		t.Fatalf("resolution lost edits: %+v", resolved.Result)
	}
}

func TestSponsorSettingsRetriesReadCurrentState(t *testing.T) {
	f := newPostWorkflow(t)
	create := &cmd.CreateSponsorshipPackage{
		SubmissionID: "new-package", Slug: "community", Name: "Community", DurationDays: 28,
	}
	if err := bus.Dispatch(f.ctx, create); err != nil {
		t.Fatal(err)
	}

	id := create.Result.ID
	update := &cmd.UpdateSponsorshipPackage{
		SubmissionID: "rename-package", ID: id, Slug: "community", Name: "Community sponsor", DurationDays: 28,
	}
	if err := bus.Dispatch(f.ctx, update); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, create); err != nil || create.Result.ID != id || create.Result.Name != update.Name {
		t.Fatalf("create retry duplicated or reverted the package: %+v, %v", create.Result, err)
	}

	response, err := f.requestWithParams(api.CreateSponsorshipPackage(), http.MethodPost, "/api/sponsorship/packages",
		`{"submissionId":"duplicate-slug","slug":"community","name":"Another package","durationDays":28}`, nil)
	if err != nil || response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate slug did not allow correction: status=%d error=%v", response.Code, err)
	}

	placement := &cmd.SaveSponsorPlacement{SubmissionID: "enable-strip", Placement: entity.SponsorPlacements[0]}
	placement.Placement.Enabled = true
	placement.Placement.Empty = "none"
	if err := bus.Dispatch(f.ctx, placement); err != nil {
		t.Fatal(err)
	}

	disable := &cmd.SaveSponsorPlacement{SubmissionID: "disable-strip", Placement: placement.Placement}
	disable.Placement.Enabled = false
	if err := bus.Dispatch(f.ctx, disable); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, placement); err != nil || placement.Result.Enabled {
		t.Fatalf("retry reverted a later setting: %+v, %v", placement.Result, err)
	}

	placement.Placement.Empty = "kofi"
	if err := bus.Dispatch(f.ctx, placement); err != app.ErrConflict {
		t.Fatalf("a receipt accepted changed input: %v", err)
	}

	post := &cmd.AddNewPost{Title: "Excluded discussion", Description: "Discussion"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	exclude := &cmd.SaveSponsorExclusion{
		SubmissionID: "exclude-discussion", Excluded: true,
		Exclusion: entity.SponsorExclusion{PageType: "post", ID: post.Result.ID, Reason: "Advertiser discussion"},
	}
	if err := bus.Dispatch(f.ctx, exclude); err != nil {
		t.Fatal(err)
	}

	restore := &cmd.SaveSponsorExclusion{SubmissionID: "restore-discussion", Exclusion: exclude.Exclusion}
	if err := bus.Dispatch(f.ctx, restore); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, exclude); err != nil || len(exclude.Result) != 0 {
		t.Fatalf("retry re-applied a removed exclusion: %+v, %v", exclude.Result, err)
	}
}

func TestSponsorCombinedEditsKeepInvalidDatesEditable(t *testing.T) {
	f := newPostWorkflow(t)
	baseline := sponsorCampaign(t, f.ctx, 25)
	laterStart := &cmd.SaveSponsorCampaign{Campaign: *baseline, SubmissionID: "later-start"}
	laterStart.Campaign.StartAt = baseline.StartAt.Add(10 * time.Hour)
	if err := bus.Dispatch(f.ctx, laterStart); err != nil {
		t.Fatal(err)
	}

	earlierEnd := &cmd.SaveSponsorCampaign{Campaign: *baseline, BaseCampaign: baseline, SubmissionID: "earlier-end"}
	earlierEnd.Campaign.EndAt = baseline.StartAt.Add(time.Hour)
	if err := bus.Dispatch(f.ctx, earlierEnd); err != nil {
		t.Fatal(err)
	}

	if earlierEnd.DraftCampaign == nil || len(earlierEnd.Problems) == 0 || earlierEnd.Result.Revision != laterStart.Result.Revision {
		t.Fatalf("invalid merged dates were committed or lost: %+v", earlierEnd)
	}

	repaired := &cmd.SaveSponsorCampaign{
		Campaign: *earlierEnd.DraftCampaign, BaseCampaign: earlierEnd.Result, SubmissionID: "repaired-dates",
	}
	repaired.Campaign.EndAt = repaired.Campaign.StartAt.Add(time.Hour)
	if err := bus.Dispatch(f.ctx, repaired); err != nil || repaired.DraftCampaign != nil {
		t.Fatalf("could not repair the combined dates: %+v, %v", repaired, err)
	}
}
