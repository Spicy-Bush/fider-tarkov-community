package postgres

import (
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

func TestSponsorCropStaysWithItsImage(t *testing.T) {
	base := entity.SponsorCreative{SponsorArtwork: entity.SponsorArtwork{ImageKey: "files/original"}}
	edited, saved := base, base
	edited.BannerCrop = &entity.SponsorImageCrop{X: 20, Y: 50, Zoom: 2}
	saved.ImageKey = "files/replacement"

	merged, conflicts := mergeSponsorCreative(base, edited, saved)
	if !conflicts.Image || len(conflicts.Creative) != 0 {
		t.Fatalf("crop crossed image identities without a choice: conflicts=%v", conflicts)
	}

	if merged.ImageKey != edited.ImageKey || *merged.BannerCrop != *edited.BannerCrop {
		t.Fatalf("draft crop no longer belongs to the edited image: %+v", merged)
	}

	merged, conflicts = mergeSponsorCreative(base, base, saved)
	if conflicts.Any() || merged.ImageKey != saved.ImageKey || merged.BannerCrop != nil {
		t.Fatalf("unchanged image did not adopt the saved replacement: %+v, %v", merged, conflicts)
	}
}

func TestSponsorMergeIndependentPlacementsAndDates(t *testing.T) {
	now := time.Now().UTC()
	base := entity.SponsorCampaign{
		ID: 1, Revision: 1, StartAt: now, EndAt: now.Add(time.Hour),
		Bookings: []entity.SponsorBooking{
			{PlacementID: "strip_desktop", Share: 25},
			{PlacementID: "feed_desktop", Share: 25},
		},
	}
	edited, saved := base, base
	edited.Bookings = []entity.SponsorBooking{{PlacementID: "feed_desktop", Share: 25}}
	saved.Bookings = []entity.SponsorBooking{
		{PlacementID: "strip_desktop", Share: 25},
		{PlacementID: "feed_desktop", Share: 50},
	}
	saved.Revision = 2
	saved.StartAt = now.In(time.FixedZone("test", 3600))
	edited.Notes = "Keep my notes"

	merged, conflicts := mergeSponsorCampaign(base, edited, saved)
	if conflicts.Any() || len(merged.Bookings) != 1 || merged.Bookings[0].Share != 50 {
		t.Fatalf("independent placements or equivalent dates conflicted: %+v, %+v", merged, conflicts)
	}
	if merged.Revision != 2 || merged.Notes != edited.Notes {
		t.Fatal("merge lost current revision or local notes")
	}

	saved.Bookings[0].Share = 75
	_, conflicts = mergeSponsorCampaign(base, edited, saved)
	if len(conflicts.Bookings) != 1 || conflicts.Bookings[0] != "strip_desktop" {
		t.Fatalf("removal did not conflict with an edited share: %+v", conflicts)
	}
}
