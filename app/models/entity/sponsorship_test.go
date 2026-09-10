package entity

import "testing"

func TestImageURLForSlot_PrefersMapThenLegacy(t *testing.T) {
	c := &SponsorshipCampaign{
		CreativeImageURL: "https://legacy.example/a.jpg",
		CreativeImageURLs: map[string]string{
			"feed_native": "https://cdn.example/feed.jpg",
			"sidebar_top": "",
		},
	}
	if got := c.ImageURLForSlot("feed_native"); got != "https://cdn.example/feed.jpg" {
		t.Fatalf("feed_native: got %q", got)
	}
	if got := c.ImageURLForSlot("sidebar_top"); got != "https://legacy.example/a.jpg" {
		t.Fatalf("sidebar_top empty map entry should fall back, got %q", got)
	}
	if got := c.ImageURLForSlot("pages_header"); got != "https://legacy.example/a.jpg" {
		t.Fatalf("pages_header missing key should fall back, got %q", got)
	}
	if got := (*SponsorshipCampaign)(nil).ImageURLForSlot("feed_native"); got != "" {
		t.Fatalf("nil campaign: got %q", got)
	}
}
