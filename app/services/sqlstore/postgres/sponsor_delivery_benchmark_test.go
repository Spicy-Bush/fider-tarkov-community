package postgres_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
)

func BenchmarkSponsorAllocationHistory(b *testing.B) {
	for _, history := range []int{0, 10000} {
		b.Run(fmt.Sprintf("expired=%d", history), func(b *testing.B) {
			f := newPostWorkflow(b)
			now := time.Now().UTC()
			campaign := &cmd.SaveSponsorCampaign{
				SubmissionID: rand.String(32),
				Campaign: entity.SponsorCampaign{
					Name:       "Current booking",
					Advertiser: "Current sponsor",
					State:      "booked",
					StartAt:    now.Add(-time.Hour),
					EndAt:      now.Add(time.Hour),
					PageTypes:  []string{"home"},
					Bookings:   []entity.SponsorBooking{{PlacementID: "strip_desktop", Share: 100}},
				},
				Creative: &entity.SponsorCreative{
					State: "approved",
					SponsorArtwork: entity.SponsorArtwork{
						Headline: "Current artwork", Destination: "https://example.com/current",
					},
				},
			}
			placement := &cmd.SaveSponsorPlacement{SubmissionID: rand.String(32),
				Placement: entity.SponsorPlacement{
					ID: "strip_desktop", Enabled: true, Position: "navigation", Empty: "none",
				},
			}

			if err := bus.Dispatch(f.ctx, campaign, placement); err != nil {
				b.Fatal(err)
			}

			expired := campaign.Campaign
			expired.StartAt = now.Add(-48 * time.Hour)
			expired.EndAt = now.Add(-24 * time.Hour)
			encoded, err := json.Marshal(expired)
			if err != nil {
				b.Fatal(err)
			}

			_, err = dbx.Connection().Exec(`
				WITH history AS (
					INSERT INTO sponsor_campaigns (tenant_id, data, state, start_at, end_at)
					SELECT $1, $2::jsonb - ARRAY['id','revision','state','startAt','endAt','confirmBy'], 'booked', $4, $5
					FROM generate_series(1, $3)
					RETURNING tenant_id, id
				)
				INSERT INTO sponsor_creatives (tenant_id, campaign_id, data)
				SELECT tenant_id, id, '{"state":"approved","headline":"Expired artwork","destination":"https://example.com/expired"}'::jsonb
				FROM history
			`, f.tenant.ID, encoded, history, expired.StartAt, expired.EndAt)
			if err != nil {
				b.Fatal(err)
			}

			request := &cmd.AllocateSponsors{
				Context: entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
				Opportunities: []entity.SponsorOpportunity{{InstanceID: "strip", PlacementID: "strip_desktop"}},
			}
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if err := bus.Dispatch(f.ctx, request); err != nil {
					b.Fatal(err)
				}

				selection := request.Result["strip"]
				if selection.Kind != "sponsor" || selection.Creative.Headline != "Current artwork" {
					b.Fatalf("unexpected delivery: %+v", selection)
				}
			}
		})
	}
}
