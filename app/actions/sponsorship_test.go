package actions

import (
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestNormalizeCreativeImageURLs(t *testing.T) {
	got := NormalizeCreativeImageURLs(map[string]string{
		"feed_native":  " https://a.test/f.jpg ",
		"nope":         "https://a.test/x.jpg",
		"sidebar_top":  "",
		"pages_header": "https://a.test/p.jpg",
	})
	if len(got) != 2 || got["feed_native"] != "https://a.test/f.jpg" || got["pages_header"] != "https://a.test/p.jpg" {
		t.Fatalf("unexpected: %#v", got)
	}
	if got := NormalizeCreativeImageURLs(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil input should yield empty map, got %#v", got)
	}
}

func TestValidateCampaignFields_RequiresAdvertiser(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	result := validateCampaignFields(
		validate.Success(),
		"Internal PO-1",
		"",
		"feed_native",
		"https://cdn.example/a.jpg",
		nil,
		"",
		"https://example.com",
		start,
		end,
		100,
		"all",
	)
	if result.Ok {
		t.Fatal("expected failure when advertiser empty")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "advertiser" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected advertiser field failure, got %#v", result.Errors)
	}

	ok := validateCampaignFields(
		validate.Success(),
		"Internal PO-1",
		"Acme Corp",
		"feed_native",
		"https://cdn.example/a.jpg",
		nil,
		"",
		"https://example.com",
		start,
		end,
		100,
		"all",
	)
	if !ok.Ok {
		t.Fatalf("expected success with advertiser set, got %#v", ok.Errors)
	}
}
