package adsselect

import (
	"strings"
	"testing"
	"time"
)

func TestSponsorClickLink(t *testing.T) {
	now := time.Now()
	click := Click{
		OpportunityID: strings.Repeat("a", 32),
		TenantID:      12,
		Destination: "https://example.com/offer?utm_source=tarkov.community&utm_campaign=launch%20week#details",
		Expires:     now.Add(time.Hour).Unix(),
	}
	token, err := click.Token("test-secret")
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := ReadClick(token, "test-secret", 12, now)
	if err != nil || *decoded != click {
		t.Fatalf("link changed its destination or attribution: %+v, %v", decoded, err)
	}

	for _, test := range []struct {
		name   string
		token  string
		tenant int
		now    time.Time
	}{
		{"other tenant", token, 13, now},
		{"expired", token, 12, now.Add(time.Hour)},
		{"changed payload", "A" + token[1:], 12, now},
		{"missing signature", strings.Split(token, ".")[0], 12, now},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReadClick(test.token, "test-secret", test.tenant, test.now); err == nil {
				t.Fatal("accepted an invalid link")
			}
		})
	}
}
