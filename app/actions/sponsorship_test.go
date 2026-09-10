package actions

import (
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestValidateCampaignFields_RequiresAdvertiser(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	result := validateCampaignFields(
		validate.Success(),
		"Internal PO-1",
		"",
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
		start,
		end,
		100,
		"all",
	)
	if !ok.Ok {
		t.Fatalf("expected success with advertiser set, got %#v", ok.Errors)
	}
}

func TestValidateCampaignFields_GraphOnlyNoCreativesRequired(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	ok := validateCampaignFields(validate.Success(), "PO", "Acme", start, end, 10, "en")
	if !ok.Ok {
		t.Fatalf("slim campaign should validate without creatives/slots: %#v", ok.Errors)
	}
}
