package actions_test

import (
	"context"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

func TestSponsorCampaignCreativeValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*actions.SaveSponsorCampaign)
		valid  bool
	}{
		{"new booking with artwork", func(*actions.SaveSponsorCampaign) {}, true},
		{"booking without artwork", func(a *actions.SaveSponsorCampaign) { a.Creative = nil }, true},
		{"missing headline", func(a *actions.SaveSponsorCampaign) { a.Creative.Headline = " " }, false},
		{"image without copy", func(a *actions.SaveSponsorCampaign) {
			a.Creative.Headline = ""
			a.Creative.ImageKey = "attachments/banner.webp"
		}, true},
		{"banner crop", func(a *actions.SaveSponsorCampaign) {
			a.Creative.ImageKey = "attachments/banner.webp"
			a.Creative.BannerCrop = &entity.SponsorImageCrop{X: 25, Y: 75, Zoom: 1.5}
		}, true},
		{"crop outside image", func(a *actions.SaveSponsorCampaign) {
			a.Creative.BannerCrop = &entity.SponsorImageCrop{X: 101, Y: 50, Zoom: 1}
		}, false},
		{"crop leaves empty space", func(a *actions.SaveSponsorCampaign) {
			a.Creative.BannerCrop = &entity.SponsorImageCrop{X: 50, Y: 50, Zoom: 0.5}
		}, false},
		{"destination with campaign tags", func(a *actions.SaveSponsorCampaign) {
			a.Creative.Destination = "https://example.com/offer?sku=42&utm_source=tarkov.community&utm_campaign=launch%20week#details"
		}, true},
		{"non-web destination", func(a *actions.SaveSponsorCampaign) { a.Creative.Destination = "mailto:sponsor@example.com" }, false},
		{"existing creative", func(a *actions.SaveSponsorCampaign) { a.Creative.ID = 10 }, false},
		{"existing booking", func(a *actions.SaveSponsorCampaign) {
			a.Campaign.ID = 10
			a.Campaign.Revision = 1
		}, false},
		{"edit booking and artwork", func(a *actions.SaveSponsorCampaign) {
			a.Campaign.ID = 10
			a.Campaign.Revision = 1
			a.Creative.CampaignID = 10
			a.Creative.ID = 20
			a.Creative.Revision = 1
		}, true},
		{"foreign creative owner", func(a *actions.SaveSponsorCampaign) { a.Creative.CampaignID = 10 }, false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			action := &actions.SaveSponsorCampaign{
				SubmissionID: "new-booking-with-artwork",
				Campaign: entity.SponsorCampaign{
					Name:          "Community sponsor",
					Advertiser:    "Retailer",
					State:         "draft",
					StartAt:       time.Now(),
					EndAt:         time.Now().Add(time.Hour),
					Bookings:      []entity.SponsorBooking{{PlacementID: "home_desktop", Share: 50}},
					PageTypes:     []string{"home"},
					Currency:      "AUD",
					PaymentStatus: "unpaid",
				},
				Creative: &entity.SponsorCreative{
					State: "draft",
					SponsorArtwork: entity.SponsorArtwork{
						Headline:    "A sponsor",
						Destination: "https://example.com",
					},
				},
			}
			test.change(action)
			result := action.Validate(context.Background(), nil)
			if result.Ok != test.valid {
				t.Fatalf("valid=%v, expected %v: %+v", result.Ok, test.valid, result)
			}
		})
	}
}

func TestSponsorPlacementAdSense(t *testing.T) {
	for _, test := range []struct {
		name  string
		empty string
		slot  string
		valid bool
	}{
		{"configured", "adsense", "1234567890", true},
		{"missing unit", "adsense", "", false},
		{"invalid unit", "adsense", "123<script>", false},
		{"disabled fill", "none", "", true},
		{"retained unit", "kofi", "1234567890", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			action := &actions.SaveSponsorPlacement{SubmissionID: "placement-validation", Placement: entity.SponsorPlacements[0]}
			action.Placement.Empty = test.empty
			action.Placement.AdSenseSlotID = test.slot
			if result := action.Validate(context.Background(), nil); result.Ok != test.valid {
				t.Fatalf("valid=%v, expected %v: %+v", result.Ok, test.valid, result)
			}
		})
	}
}
