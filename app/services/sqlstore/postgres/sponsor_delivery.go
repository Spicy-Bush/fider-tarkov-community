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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/lib/pq"
)

type sponsorCandidate struct {
	campaign *entity.SponsorCampaign
	creative *entity.SponsorCreative
	share    int
}

type sponsorOpportunity struct {
	ID          string `db:"id" json:"id"`
	PlacementID string `db:"placement_id" json:"placement"`
	InstanceID  string `db:"instance_id" json:"instance"`
	CampaignID  int    `db:"campaign_id" json:"campaign"`
	CreativeID  int    `db:"creative_id" json:"creative"`
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
	if len(c.PageID) != 32 || c.ExpiresAt.Before(time.Now()) {
		return validate.Failed("A current sponsorship page is required.")
	}

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

		if _, err := trx.Execute(`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, fmt.Sprintf("sponsor-page:%d:%s", tenant.ID, c.PageID)); err != nil {
			return err
		}

		var saved []*sponsorOpportunity
		instanceIDs := make([]string, 0, len(c.Opportunities))
		for _, opportunity := range c.Opportunities {
			instanceIDs = append(instanceIDs, opportunity.InstanceID)
		}
		if err := trx.Select(&saved, `
			SELECT id, instance_id, placement_id, COALESCE(campaign_id,0) AS campaign_id, COALESCE(creative_id,0) AS creative_id
			FROM sponsor_opportunities WHERE tenant_id=$1 AND page_id=$2 AND instance_id=ANY($3)
		`, tenant.ID, c.PageID, pq.Array(instanceIDs)); err != nil {
			return err
		}
		previous := make(map[string]*sponsorOpportunity, len(saved))
		for _, row := range saved {
			previous[row.PlacementID+":"+row.InstanceID] = row
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
		var receipts []*sponsorOpportunity
		for _, placement := range placements.Result {
			if !placement.Enabled || placement.Device != c.Context.Device || (placement.PageType != "all" && placement.PageType != c.Context.PageType) {
				continue
			}

			var instances []string
			for _, opportunity := range c.Opportunities {
				if opportunity.PlacementID == placement.ID {
					if _, exists := previous[placement.ID+":"+opportunity.InstanceID]; exists {
						continue
					}

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
				receipt := &sponsorOpportunity{
					ID: rand.String(32), PlacementID: placement.ID, InstanceID: instance,
				}
				winner := queue[(cursor+int64(offset))%100]
				if winner < len(candidates) {
					candidate := candidates[winner]
					receipt.CampaignID = candidate.campaign.ID
					receipt.CreativeID = candidate.creative.ID
					counts[winner]++
				}

				previous[placement.ID+":"+instance] = receipt
				receipts = append(receipts, receipt)
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

		for _, placement := range placements.Result {
			if !placement.Enabled || placement.Device != c.Context.Device || (placement.PageType != "all" && placement.PageType != c.Context.PageType) {
				continue
			}

			for _, opportunity := range c.Opportunities {
				receipt := previous[placement.ID+":"+opportunity.InstanceID]
				if receipt == nil || opportunity.PlacementID != placement.ID {
					continue
				}

				selection := entity.SponsorSelection{Kind: placement.Empty, Placement: placement}
				if receipt.CampaignID != 0 {
					selection.Kind = "none"
				}

				for _, candidate := range byPlacement[placement.ID] {
					if candidate.campaign.ID != receipt.CampaignID || candidate.creative.ID != receipt.CreativeID {
						continue
					}

					expires := min(candidate.campaign.EndAt.Unix(), c.ExpiresAt.Unix())
					if candidate.creative.EndAt != nil {
						expires = min(expires, candidate.creative.EndAt.Unix())
					}
					if candidate.creative.OfferCode != "" && candidate.creative.OfferExpires != nil {
						expires = min(expires, candidate.creative.OfferExpires.Unix())
					}

					click := adsselect.Click{
						OpportunityID: receipt.ID, TenantID: tenant.ID,
						Destination: candidate.creative.Destination, Expires: expires,
					}
					token, err := click.Token(env.Config.JWTSecret)
					if err != nil {
						return err
					}

					expiresAt := time.Unix(expires, 0)
					selection.Kind = "sponsor"
					selection.Advertiser = candidate.campaign.Advertiser
					selection.Creative = &candidate.creative.SponsorArtwork
					selection.ExpiresAt = &expiresAt
					selection.ClickURL = "/sponsorship/click?token=" + token
					break
				}

				c.Result[opportunity.InstanceID] = selection
			}
		}

		if len(receipts) > 0 {
			encoded, err := json.Marshal(receipts)
			if err != nil {
				return err
			}
			if _, err := trx.Execute(`
				INSERT INTO sponsor_opportunities (tenant_id, id, page_id, placement_id, instance_id, expires_at, campaign_id, creative_id, day)
				SELECT $1, id, $2, placement, instance, $3, NULLIF(campaign,0), NULLIF(creative,0), $5
				FROM jsonb_to_recordset($4) AS r(id text, placement text, instance text, campaign integer, creative integer)
			`, tenant.ID, c.PageID, c.ExpiresAt, string(encoded), now.Format("2006-01-02")); err != nil {
				return err
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
		_, err := trx.Execute(`
			WITH consumed AS (
				UPDATE sponsor_opportunities SET clicked=true
				WHERE tenant_id=$1 AND id=$2 AND NOT clicked AND expires_at > NOW()
				RETURNING campaign_id, creative_id, placement_id, day
			)
			UPDATE sponsor_allocations a SET clicks=a.clicks+1
			FROM consumed c
			WHERE a.tenant_id=$1 AND a.campaign_id=c.campaign_id AND a.creative_id=c.creative_id
			  AND a.placement_id=c.placement_id AND a.day=c.day AND a.allocated>0
		`, tenant.ID, c.Click.OpportunityID)
		return err
	})
}

func purgeSponsorOpportunities(ctx context.Context, c *cmd.PurgeSponsorOpportunities) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`DELETE FROM sponsor_opportunities WHERE expires_at < NOW()`)
		return err
	})
}
