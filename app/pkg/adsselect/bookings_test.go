package adsselect

import (
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

func TestSchedulePurchasedShares(t *testing.T) {
	for share := 1; share <= 100; share++ {
		queue := Schedule([]int{share, 100 - share})
		allocated := 0
		for _, winner := range queue {
			if winner == 0 {
				allocated++
			}
		}

		if allocated != share {
			t.Fatalf("purchased %d percent, allocated %d", share, allocated)
		}
	}
}

func TestBookingAvailability(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	booking := func(id, share int) *entity.SponsorCampaign {
		return &entity.SponsorCampaign{
			ID: id, Name: "Retailer", Advertiser: "One", Category: "retail",
			State: "booked", StartAt: now, EndAt: now.Add(24 * time.Hour),
			PageTypes: []string{"post"},
			Bookings:  []entity.SponsorBooking{{PlacementID: "post_mobile", Share: share}},
		}
	}

	cases := []struct {
		name     string
		change   func(*entity.SponsorCampaign, *entity.SponsorCampaign)
		conflict bool
	}{
		{name: "overbooking", conflict: true},
		{name: "exact capacity", change: func(a, b *entity.SponsorCampaign) { b.Bookings[0].Share = 40 }},
		{name: "disjoint dates", change: func(a, b *entity.SponsorCampaign) { b.StartAt = a.EndAt; b.EndAt = b.StartAt.Add(time.Hour) }},
		{name: "disjoint countries", change: func(a, b *entity.SponsorCampaign) { a.Countries = []string{"AU"}; b.Countries = []string{"NZ"} }},
		{name: "wildcard country", change: func(a, b *entity.SponsorCampaign) { a.Countries = []string{"AU"} }, conflict: true},
		{name: "paused reserves capacity", change: func(a, b *entity.SponsorCampaign) { b.State = "paused" }, conflict: true},
		{name: "expired reservation", change: func(a, b *entity.SponsorCampaign) { b.State = "reserved"; b.ConfirmBy = &now }},
		{name: "exclusive competitor", change: func(a, b *entity.SponsorCampaign) { a.Exclusive = true; b.Advertiser = "Two"; b.Bookings[0].Share = 1 }, conflict: true},
		{name: "same advertiser", change: func(a, b *entity.SponsorCampaign) { a.Exclusive = true; b.Bookings[0].Share = 1 }},
		{name: "disjoint placement", change: func(a, b *entity.SponsorCampaign) { b.Bookings[0].PlacementID = "post_desktop" }},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a, b := booking(1, 60), booking(2, 60)
			if test.change != nil {
				test.change(a, b)
			}

			err := CheckAvailability(a, []*entity.SponsorCampaign{b}, now)
			if (err != nil) != test.conflict {
				t.Fatalf("conflict=%v, error=%v", test.conflict, err)
			}
		})
	}
}
