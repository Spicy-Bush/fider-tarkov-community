package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
)

func TestSponsorClickDuringReportingFailure(t *testing.T) {
	click := adsselect.Click{
		OpportunityID: strings.Repeat("a", 32),
		TenantID:      mock.DemoTenant.ID,
		Destination:   "https://example.com/offer?utm_campaign=launch#details",
		Expires:       time.Now().Add(time.Hour).Unix(),
	}
	token, err := click.Token(env.Config.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}

	for _, unavailable := range []bool{true, false} {
		server := mock.NewServer().OnTenant(mock.DemoTenant).WithURL("/sponsorship/click?token=" + token)
		bus.AddHandler(func(ctx context.Context, command *cmd.RecordSponsorClick) error {
			if command.Click.OpportunityID != click.OpportunityID {
				t.Fatal("retry changed its opportunity")
			}

			if unavailable {
				return errors.New("reporting database unavailable")
			}

			return nil
		})

		status, response := server.Execute(handlers.SponsorClick())
		if status != http.StatusTemporaryRedirect || response.Header().Get("Location") != click.Destination {
			t.Fatalf("reporting failure blocked redirect: status=%d destination=%s", status, response.Header().Get("Location"))
		}
	}
}
