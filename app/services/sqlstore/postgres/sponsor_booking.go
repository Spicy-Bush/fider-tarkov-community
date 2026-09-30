package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type sponsorRecord struct {
	ID        int          `db:"id"`
	Revision  int          `db:"revision"`
	State     string       `db:"state"`
	StartAt   time.Time    `db:"start_at"`
	EndAt     time.Time    `db:"end_at"`
	ConfirmBy sql.NullTime `db:"confirm_by"`
	Data      []byte       `db:"data"`
}

func sponsorReceipt(tenantID, userID int, kind, submissionID string, value any) (commandReceipt, error) {
	encoded, err := json.Marshal(value)
	return commandReceipt{
		TenantID: tenantID, UserID: userID, Kind: kind, SubmissionID: submissionID,
		Fingerprint: fmt.Sprintf("%x", sha256.Sum256(encoded)),
	}, err
}

func (row sponsorRecord) campaign() (*entity.SponsorCampaign, error) {
	campaign := new(entity.SponsorCampaign)
	if err := json.Unmarshal(row.Data, campaign); err != nil {
		return nil, err
	}

	campaign.ID = row.ID
	campaign.Revision = row.Revision
	campaign.State = row.State
	campaign.StartAt = row.StartAt.UTC()
	campaign.EndAt = row.EndAt.UTC()
	if row.ConfirmBy.Valid {
		deadline := row.ConfirmBy.Time.UTC()
		campaign.ConfirmBy = &deadline
	}

	for _, values := range []*[]string{&campaign.Countries, &campaign.Languages, &campaign.PageTypes} {
		if *values == nil {
			*values = []string{}
		}
	}

	if campaign.Bookings == nil {
		campaign.Bookings = []entity.SponsorBooking{}
	}

	return campaign, nil
}

func readSponsorCampaigns(trx *dbx.Trx, tenantID int) ([]*entity.SponsorCampaign, error) {
	var rows []*sponsorRecord
	if err := trx.Select(&rows, `
		SELECT id, revision, state, start_at, end_at, confirm_by, data
		FROM sponsor_campaigns WHERE tenant_id=$1 ORDER BY id
	`, tenantID); err != nil {
		return nil, err
	}

	result := make([]*entity.SponsorCampaign, 0, len(rows))
	for _, row := range rows {
		campaign, err := row.campaign()
		if err != nil {
			return nil, err
		}

		result = append(result, campaign)
	}

	return result, nil
}

func readSponsorCampaign(trx *dbx.Trx, tenantID, id int) (*entity.SponsorCampaign, error) {
	var row sponsorRecord
	if err := trx.Get(&row, `
		SELECT id, revision, state, start_at, end_at, confirm_by, data
		FROM sponsor_campaigns WHERE tenant_id=$1 AND id=$2
	`, tenantID, id); err != nil {
		return nil, err
	}

	return row.campaign()
}

func lockSponsorConfiguration(trx *dbx.Trx, tenantID int, writing bool) error {
	key := fmt.Sprintf("sponsorship:%d", tenantID)
	statement := `SELECT pg_advisory_xact_lock_shared(hashtextextended($1, 0))`
	if writing {
		statement = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`
	}

	_, err := trx.Execute(statement, key)
	return err
}

func saveSponsorCampaign(ctx context.Context, c *cmd.SaveSponsorCampaign) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		operation, err := json.Marshal(struct {
			Campaign     entity.SponsorCampaign
			Creative     *entity.SponsorCreative
			BaseCampaign *entity.SponsorCampaign
			BaseCreative *entity.SponsorCreative
		}{c.Campaign, c.Creative, c.BaseCampaign, c.BaseCreative})
		if err != nil {
			return err
		}

		receipt := commandReceipt{
			TenantID: tenant.ID, UserID: user.ID, Kind: "sponsor-booking", SubmissionID: c.SubmissionID,
			Fingerprint: fmt.Sprintf("%x", sha256.Sum256(operation)),
		}
		var saved struct {
			CampaignID int
			CreativeID int
		}
		replayed, err := receipt.read(trx, &saved)
		if err != nil {
			return err
		}

		if replayed {
			c.Result, err = readSponsorCampaign(trx, tenant.ID, saved.CampaignID)
			if err != nil || saved.CreativeID == 0 {
				return err
			}

			c.CreativeResult, err = readSponsorCreative(trx, tenant.ID, saved.CreativeID)

			return err
		}

		if err := lockSponsorConfiguration(trx, tenant.ID, true); err != nil {
			return err
		}

		campaign, creative := c.Campaign, c.Creative
		stale := false
		c.Conflicts = entity.SponsorConflicts{}
		c.Problems = nil
		c.DraftCampaign, c.DraftCreative = nil, nil
		if campaign.ID != 0 {
			current, err := readSponsorCampaign(trx, tenant.ID, campaign.ID)
			if err != nil {
				return err
			}

			c.Result = current
			if current.Revision != campaign.Revision {
				if c.BaseCampaign == nil || c.BaseCampaign.ID != campaign.ID || c.BaseCampaign.Revision != campaign.Revision {
					return app.ErrConflict
				}

				campaign, c.Conflicts = mergeSponsorCampaign(*c.BaseCampaign, campaign, *current)
				stale = true
			}
		}

		if creative != nil && creative.ID != 0 {
			current, err := readSponsorCreative(trx, tenant.ID, creative.ID)
			if err != nil {
				return err
			}

			if current.CampaignID != campaign.ID {
				return app.ErrNotFound
			}

			c.CreativeResult = current
			if current.Revision != creative.Revision {
				if c.BaseCreative == nil || c.BaseCreative.ID != creative.ID || c.BaseCreative.Revision != creative.Revision {
					return app.ErrConflict
				}

				merged, conflicts := mergeSponsorCreative(*c.BaseCreative, *creative, *current)

				creative = &merged
				c.Conflicts.Creative = conflicts.Creative
				c.Conflicts.Image = conflicts.Image
				stale = true
			}
		}

		if c.Conflicts.Any() {
			c.DraftCampaign, c.DraftCreative = &campaign, creative
			return nil
		}

		if stale {
			action := actions.SaveSponsorCampaign{Campaign: campaign, Creative: creative, SubmissionID: c.SubmissionID}
			if result := action.Validate(ctx, user); !result.Ok {
				c.DraftCampaign, c.DraftCreative = &campaign, creative
				for _, problem := range result.Errors {
					c.Problems = append(c.Problems, problem.Message)
				}
				return nil
			}
		}

		encoded, err := json.Marshal(campaign)
		if err != nil {
			return err
		}

		now := time.Now()
		if adsselect.Reserves(&campaign, now) {
			campaigns, err := readSponsorCampaigns(trx, tenant.ID)
			if err != nil {
				return err
			}

			if err := adsselect.CheckAvailability(&campaign, campaigns, now); err != nil {
				if stale {
					c.DraftCampaign, c.DraftCreative = &campaign, creative
					c.Problems = []string{err.Error()}
					return nil
				}

				return validate.Failed(err.Error())
			}
		}

		var savedID int
		if campaign.ID == 0 {
			err = trx.Scalar(&savedID, `
				INSERT INTO sponsor_campaigns (tenant_id, data, state, start_at, end_at, confirm_by)
				VALUES ($1, $2::jsonb - ARRAY['id','revision','state','startAt','endAt','confirmBy'], $3, $4, $5, $6)
				RETURNING id
			`, tenant.ID, encoded, campaign.State, campaign.StartAt, campaign.EndAt, campaign.ConfirmBy)
		} else {
			savedID = campaign.ID
			var changed int64
			changed, err = trx.Execute(`
				UPDATE sponsor_campaigns
				SET data=$4::jsonb - ARRAY['id','revision','state','startAt','endAt','confirmBy'], revision=revision+1,
				    state=$5, start_at=$6, end_at=$7, confirm_by=$8
				WHERE tenant_id=$1 AND id=$2 AND revision=$3
			`, tenant.ID, savedID, campaign.Revision, encoded, campaign.State, campaign.StartAt, campaign.EndAt, campaign.ConfirmBy)
			if err == nil && changed == 0 {
				return app.ErrConflict
			}
		}

		if err != nil {
			return err
		}

		c.Result, err = readSponsorCampaign(trx, tenant.ID, savedID)
		if err != nil {
			return err
		}

		if _, err := trx.Execute(`
			INSERT INTO sponsor_changes (tenant_id, campaign_id, revision, state)
			VALUES ($1,$2,$3,$4)
		`, tenant.ID, savedID, c.Result.Revision, c.Result.State); err != nil {
			return err
		}

		saved.CampaignID = savedID
		if creative != nil {
			creative := *creative
			creative.CampaignID = savedID
			c.CreativeResult, err = writeSponsorCreative(trx, tenant.ID, creative)
			if err != nil {
				return err
			}

			saved.CreativeID = c.CreativeResult.ID
		}

		return receipt.save(trx, saved)
	})
}

func getSponsorPlacements(ctx context.Context, q *query.GetSponsorPlacements) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var settings []*struct {
			AdSenseSlotID string `db:"adsense_slot_id"`
			ID            string `db:"placement_id"`
			Enabled       bool   `db:"enabled"`
			Position      string `db:"position"`
			Every         int    `db:"every"`
			Empty         string `db:"empty"`
		}
		if err := trx.Select(&settings, `
			SELECT placement_id, enabled, position, every, empty, adsense_slot_id FROM sponsor_placement_settings WHERE tenant_id=$1
		`, tenant.ID); err != nil {
			return err
		}

		q.Result = append([]entity.SponsorPlacement{}, entity.SponsorPlacements...)
		for i := range q.Result {
			q.Result[i].Empty = "none"
			for _, saved := range settings {
				if saved.ID == q.Result[i].ID {
					q.Result[i].Enabled = saved.Enabled
					q.Result[i].Position = saved.Position
					q.Result[i].Every = saved.Every
					q.Result[i].Empty = saved.Empty
					q.Result[i].AdSenseSlotID = saved.AdSenseSlotID
					break
				}
			}
		}

		return nil
	})
}

func saveSponsorPlacement(ctx context.Context, c *cmd.SaveSponsorPlacement) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		receipt, err := sponsorReceipt(tenant.ID, user.ID, "sponsor-placement", c.SubmissionID, c.Placement)
		if err != nil {
			return err
		}

		replayed, err := receipt.read(trx, nil)
		if err != nil {
			return err
		}

		if replayed {
			placements := &query.GetSponsorPlacements{}
			if err := getSponsorPlacements(ctx, placements); err != nil {
				return err
			}

			for _, placement := range placements.Result {
				if placement.ID == c.Placement.ID {
					c.Result = placement
					return nil
				}
			}

			return app.ErrNotFound
		}

		if err := lockSponsorConfiguration(trx, tenant.ID, true); err != nil {
			return err
		}

		p := c.Placement
		_, err = trx.Execute(`
			INSERT INTO sponsor_placement_settings (tenant_id, placement_id, enabled, position, every, empty, adsense_slot_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (tenant_id, placement_id) DO UPDATE
			SET enabled=EXCLUDED.enabled, position=EXCLUDED.position, every=EXCLUDED.every, empty=EXCLUDED.empty, adsense_slot_id=EXCLUDED.adsense_slot_id
		`, tenant.ID, p.ID, p.Enabled, p.Position, p.Every, p.Empty, p.AdSenseSlotID)
		c.Result = p
		if err != nil {
			return err
		}

		return receipt.save(trx, nil)
	})
}

func getSponsorManagement(ctx context.Context, q *query.GetSponsorManagement) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		browse := q.Browse
		browse.Page = max(1, browse.Page)
		browse.ArtworkPage = max(1, browse.ArtworkPage)
		q.Result = query.SponsorManagement{
			Browse:    browse,
			Campaigns: []*entity.SponsorCampaign{},
			Creatives: []*entity.SponsorCreative{},
		}

		if browse.CampaignID == 0 {
			var rows []*sponsorRecord
			if err := trx.Select(&rows, `
				SELECT campaign.id, revision, state, start_at, end_at, confirm_by, data
				FROM sponsor_campaigns campaign
				JOIN (
					SELECT id FROM sponsor_campaigns
					WHERE tenant_id=$1 AND ($2='' OR data->>'name' ILIKE '%' || $2 || '%'
					    OR data->>'advertiser' ILIKE '%' || $2 || '%')
					ORDER BY id DESC LIMIT 26 OFFSET $3
				) page ON page.id=campaign.id
				WHERE campaign.tenant_id=$1 ORDER BY campaign.id DESC
			`, tenant.ID, browse.Search, (browse.Page-1)*25); err != nil {
				return err
			}

			q.Result.NextPage = len(rows) > 25
			for _, row := range rows[:min(25, len(rows))] {
				campaign, err := row.campaign()
				if err != nil {
					return err
				}
				q.Result.Campaigns = append(q.Result.Campaigns, campaign)
			}
		} else {
			campaign, err := readSponsorCampaign(trx, tenant.ID, browse.CampaignID)
			if err != nil {
				return err
			}

			creatives, err := readSponsorCreatives(trx, tenant.ID, campaign.ID, browse.ArtworkPage)
			if err != nil {
				return err
			}

			q.Result.Campaigns = append(q.Result.Campaigns, campaign)
			q.Result.NextArtwork = len(creatives) > 25
			q.Result.Creatives = creatives[:min(25, len(creatives))]
		}

		placements := &query.GetSponsorPlacements{}
		if err := getSponsorPlacements(ctx, placements); err != nil {
			return err
		}

		q.Result.Placements = placements.Result
		q.Result.Exclusions = []*entity.SponsorExclusion{}
		return trx.Select(&q.Result.Exclusions, `
			SELECT page_type, content_id, reason FROM sponsor_exclusions WHERE tenant_id=$1 ORDER BY page_type, content_id
		`, tenant.ID)
	})
}

func getSponsorReport(ctx context.Context, q *query.GetSponsorReport) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		q.Allocations = []*entity.SponsorAllocation{}
		if err := trx.Select(&q.Allocations, `
			SELECT day::text, campaign_id, placement_id, creative_id, eligible, allocated, expected, clicks
			FROM sponsor_allocations WHERE tenant_id=$1 AND campaign_id=$2
			ORDER BY day DESC, placement_id, creative_id
		`, tenant.ID, q.CampaignID); err != nil {
			return err
		}

		q.Changes = []*entity.SponsorChange{}
		return trx.Select(&q.Changes, `
			SELECT campaign_id, state, at FROM sponsor_changes WHERE tenant_id=$1 AND campaign_id=$2 ORDER BY revision
		`, tenant.ID, q.CampaignID)
	})
}
