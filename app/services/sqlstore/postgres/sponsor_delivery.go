package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/lib/pq"
)

type sponsorCandidate struct {
	campaign *entity.SponsorCampaign
	creative *entity.SponsorCreative
	share    int
}

func readSponsorCandidates(trx *dbx.Trx, tenantID int, viewer entity.SponsorContext, placements []string, now time.Time) (map[string][]sponsorCandidate, error) {
	var rows []*struct {
		PlacementID  string    `db:"placement_id"`
		Share        int       `db:"share"`
		CampaignID   int       `db:"campaign_id"`
		Advertiser   string    `db:"advertiser"`
		CampaignEnd  time.Time `db:"campaign_end"`
		CreativeID   int       `db:"creative_id"`
		CreativeData []byte    `db:"creative_data"`
		ImageKey     string    `db:"image_key"`
		LogoKey      string    `db:"logo_key"`
	}

	err := trx.Select(&rows, `
		SELECT booking->>'placementId' AS placement_id, (booking->>'share')::int AS share,
		       campaign.id AS campaign_id, campaign.data->>'advertiser' AS advertiser, campaign.end_at AS campaign_end,
		       artwork.id AS creative_id, artwork.data AS creative_data, artwork.image_key, artwork.logo_key
		FROM sponsor_campaigns campaign
		CROSS JOIN LATERAL jsonb_array_elements(campaign.data->'bookings') booking
		JOIN LATERAL (
			SELECT creative.id, creative.data, creative.image_key, creative.logo_key
			FROM sponsor_creatives creative
			WHERE creative.tenant_id=campaign.tenant_id AND creative.campaign_id=campaign.id
			  AND creative.data->>'state'='approved'
			  AND creative.data->>'language' IN ('', $3)
			  AND creative.data->>'device' IN ('', $5)
			  AND ((creative.data->>'startAt') IS NULL OR (creative.data->>'startAt')::timestamptz <= $6)
			  AND ((creative.data->>'endAt') IS NULL OR (creative.data->>'endAt')::timestamptz > $6)
			  AND (creative.data->>'offerCode'='' OR (creative.data->>'offerExpires')::timestamptz > $6)
			ORDER BY COALESCE((creative.data->>'startAt')::timestamptz, campaign.start_at) DESC, creative.id DESC
			LIMIT 1
		) artwork ON true
		WHERE campaign.tenant_id=$1 AND campaign.state='booked'
		  AND campaign.start_at <= $6 AND campaign.end_at > $6
		  AND campaign.data->'pageTypes' ? $2
		  AND (campaign.data->'languages' IN ('null'::jsonb, '[]'::jsonb) OR campaign.data->'languages' ? $3)
		  AND (campaign.data->'countries' IN ('null'::jsonb, '[]'::jsonb) OR campaign.data->'countries' ? $4)
		  AND booking->>'placementId'=ANY($7)
		ORDER BY placement_id, campaign.id
	`, tenantID, viewer.PageType, viewer.Language, viewer.Country, viewer.Device, now, pq.Array(placements))
	if err != nil {
		return nil, err
	}

	result := make(map[string][]sponsorCandidate)
	for _, row := range rows {
		campaign := &entity.SponsorCampaign{ID: row.CampaignID, Advertiser: row.Advertiser, EndAt: row.CampaignEnd}
		creative := new(entity.SponsorCreative)

		if err := json.Unmarshal(row.CreativeData, creative); err != nil {
			return nil, err
		}

		creative.ID = row.CreativeID
		creative.ImageKey = row.ImageKey
		creative.LogoKey = row.LogoKey
		result[row.PlacementID] = append(result[row.PlacementID], sponsorCandidate{campaign, creative, row.Share})
	}

	return result, nil
}

func allocateSponsors(ctx context.Context, c *cmd.AllocateSponsors) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := lockSponsorConfiguration(trx, tenant.ID, false); err != nil {
			return err
		}

		placements := &query.GetSponsorPlacements{}
		if err := getSponsorPlacements(ctx, placements); err != nil {
			return err
		}

		c.Result = make(map[string]entity.SponsorSelection, len(c.Opportunities))
		for _, opportunity := range c.Opportunities {
			c.Result[opportunity.InstanceID] = entity.SponsorSelection{Kind: "none"}
		}

		var allowed bool
		if err := trx.Scalar(&allowed, `
			SELECT NOT EXISTS (
				SELECT 1 FROM sponsor_exclusions WHERE tenant_id=$1 AND page_type=$2 AND content_id=$3
			) AND CASE $2
				WHEN 'home' THEN $3=0
				WHEN 'post' THEN EXISTS (
					SELECT 1 FROM visible_posts_for($1,false,0) p
					WHERE p.id=$3 AND NOT p.moderation_pending
				)
				WHEN 'page' THEN EXISTS (
					SELECT 1 FROM pages WHERE tenant_id=$1 AND id=$3 AND status='published' AND visibility IN ('public','unlisted')
				)
				ELSE false
			END
		`, tenant.ID, c.Context.PageType, c.Context.ID); err != nil {
			return err
		}

		if !allowed {
			return nil
		}

		requested := make([]string, 0, len(c.Opportunities))
		for _, opportunity := range c.Opportunities {
			requested = append(requested, opportunity.PlacementID)
		}

		now := time.Now().UTC()
		byPlacement, err := readSponsorCandidates(trx, tenant.ID, c.Context, requested, now)
		if err != nil {
			return err
		}

		var campaignIDs, creativeIDs, eligible, allocated, expected []int64
		var placementIDs []string
		for _, placement := range placements.Result {
			if !placement.Enabled || placement.Device != c.Context.Device || (placement.PageType != "all" && placement.PageType != c.Context.PageType) {
				continue
			}

			var instances []string
			for _, opportunity := range c.Opportunities {
				if opportunity.PlacementID == placement.ID {
					instances = append(instances, opportunity.InstanceID)
				}
			}
			if len(instances) == 0 {
				continue
			}

			candidates := byPlacement[placement.ID]
			shares := make([]int, 0, len(candidates)+1)
			var signature strings.Builder
			remaining := 100
			for _, candidate := range candidates {
				shares = append(shares, candidate.share)
				remaining -= candidate.share
				fmt.Fprintf(&signature, "%d:%d;", candidate.campaign.ID, candidate.share)
			}
			if remaining < 0 {
				return fmt.Errorf("sponsor capacity invariant failed for %s", placement.ID)
			}
			shares = append(shares, remaining)

			var cursor int64
			if len(candidates) > 0 {
				err := trx.Scalar(&cursor, `
					INSERT INTO sponsor_allocation_groups (tenant_id, day, placement_id, eligibility, next_slot)
					VALUES ($1,$2,$3,$4,$5)
					ON CONFLICT (tenant_id, day, placement_id, eligibility)
					DO UPDATE SET next_slot=sponsor_allocation_groups.next_slot+EXCLUDED.next_slot
					RETURNING next_slot-$5
				`, tenant.ID, now.Format("2006-01-02"), placement.ID, fmt.Sprintf("%x", sha256.Sum256([]byte(signature.String()))), len(instances))
				if err != nil {
					return err
				}
			}

			queue := adsselect.Schedule(shares)
			counts := make([]int64, len(candidates))
			for offset, instance := range instances {
				selection := entity.SponsorSelection{Kind: placement.Empty, Placement: placement}
				winner := queue[(cursor+int64(offset))%100]
				if winner < len(candidates) {
					candidate := candidates[winner]
					counts[winner]++
					expires := candidate.campaign.EndAt
					deadlines := []*time.Time{candidate.creative.EndAt}
					if candidate.creative.OfferCode != "" {
						deadlines = append(deadlines, candidate.creative.OfferExpires)
					}

					for _, deadline := range deadlines {
						if deadline != nil && deadline.Before(expires) {
							expires = *deadline
						}
					}

					selection.Kind = "sponsor"
					selection.Advertiser = candidate.campaign.Advertiser
					selection.Creative = &candidate.creative.SponsorArtwork
					selection.ExpiresAt = &expires
					click := adsselect.Click{
						TenantID: tenant.ID, CampaignID: candidate.campaign.ID, CreativeID: candidate.creative.ID,
						PlacementID: placement.ID, Day: now.Format("2006-01-02"),
						Destination: candidate.creative.Destination, Expires: expires.Unix(),
					}
					token, err := click.Token(env.Config.JWTSecret)
					if err != nil {
						return err
					}

					selection.ClickURL = "/sponsorship/click?token=" + token
				}

				c.Result[instance] = selection
			}

			for i, candidate := range candidates {
				campaignIDs = append(campaignIDs, int64(candidate.campaign.ID))
				creativeIDs = append(creativeIDs, int64(candidate.creative.ID))
				placementIDs = append(placementIDs, placement.ID)
				eligible = append(eligible, int64(len(instances)))
				allocated = append(allocated, counts[i])
				expected = append(expected, int64(len(instances)*candidate.share))
			}
		}

		if len(campaignIDs) == 0 {
			return nil
		}

		_, err = trx.Execute(`
			INSERT INTO sponsor_allocations (tenant_id, day, campaign_id, placement_id, creative_id, eligible, allocated, expected)
			SELECT $1,$2,campaign,placement,creative,eligible,allocated,expected
			FROM unnest($3::bigint[],$4::text[],$5::bigint[],$6::bigint[],$7::bigint[],$8::bigint[])
			AS r(campaign,placement,creative,eligible,allocated,expected)
			ON CONFLICT (tenant_id, day, campaign_id, placement_id, creative_id) DO UPDATE
			SET eligible=sponsor_allocations.eligible+EXCLUDED.eligible,
			    allocated=sponsor_allocations.allocated+EXCLUDED.allocated,
			    expected=sponsor_allocations.expected+EXCLUDED.expected
		`, tenant.ID, now.Format("2006-01-02"), pq.Array(campaignIDs), pq.Array(placementIDs), pq.Array(creativeIDs), pq.Array(eligible), pq.Array(allocated), pq.Array(expected))
		return err
	})
}

func recordSponsorClick(ctx context.Context, c *cmd.RecordSponsorClick) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		click := c.Click
		_, err := trx.Execute(`
			UPDATE sponsor_allocations SET clicks=clicks+1
			WHERE tenant_id=$1 AND campaign_id=$2 AND creative_id=$3 AND placement_id=$4
			  AND day=$5 AND allocated>0
		`, tenant.ID, click.CampaignID, click.CreativeID, click.PlacementID, click.Day)
		return err
	})
}
