package postgres

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

type dbAdPlacement struct {
	ID          string        `db:"id"`
	Name        string        `db:"name"`
	Description string        `db:"description"`
	Kind        string        `db:"kind"`
	MaxWidth    sql.NullInt64 `db:"max_width"`
	MaxHeight   sql.NullInt64 `db:"max_height"`
	Sort        int           `db:"sort"`
	Enabled     bool          `db:"enabled"`
}

func (r *dbAdPlacement) toModel() *entity.AdPlacement {
	p := &entity.AdPlacement{
		ID: r.ID, Name: r.Name, Description: r.Description, Kind: r.Kind,
		Sort: r.Sort, Enabled: r.Enabled,
	}
	if r.MaxWidth.Valid {
		v := int(r.MaxWidth.Int64)
		p.MaxWidth = &v
	}
	if r.MaxHeight.Valid {
		v := int(r.MaxHeight.Int64)
		p.MaxHeight = &v
	}
	return p
}

func listAdPlacements(ctx context.Context, q *query.ListAdPlacements) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = []*entity.AdPlacement{}
		rows := []*dbAdPlacement{}
		err := trx.Select(&rows, `
			SELECT id, name, description, kind, max_width, max_height, sort, enabled
			FROM ad_placements
			ORDER BY sort ASC, id ASC`)
		if err != nil {
			return errors.Wrap(err, "failed to list ad placements")
		}
		q.Result = make([]*entity.AdPlacement, len(rows))
		for i, row := range rows {
			q.Result[i] = row.toModel()
		}
		return nil
	})
}

type dbAdCandidate struct {
	CampaignID        int    `db:"campaign_id"`
	PlacementID       string `db:"placement_id"`
	CreativeVersionID int    `db:"creative_version_id"`
	Weight            int    `db:"weight"`
	Advertiser        string `db:"advertiser"`
}

func getActiveAdCandidates(ctx context.Context, q *query.GetActiveAdCandidates) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = []adsselect.Candidate{}
		if len(q.PlacementIDs) == 0 {
			return nil
		}
		locale := q.Locale
		if locale == "" {
			locale = "all"
		}
		now := q.Now.UTC()
		rows := []*dbAdCandidate{}
		err := trx.Select(&rows, `
			SELECT c.id AS campaign_id,
			       a.placement_id,
			       a.creative_version_id,
			       c.weight,
			       c.advertiser
			FROM sponsorship_campaigns c
			INNER JOIN campaign_assignments a
			  ON a.tenant_id = c.tenant_id AND a.campaign_id = c.id
			WHERE c.tenant_id = $1
			  AND c.enabled = true
			  AND c.start_at <= $2
			  AND c.end_at > $2
			  AND (c.locale = 'all' OR c.locale = $3)
			  AND a.placement_id = ANY($4)
			ORDER BY c.weight DESC, c.id ASC`,
			tenant.ID, now, locale, pq.Array(q.PlacementIDs))
		if err != nil {
			return errors.Wrap(err, "failed to query active ad candidates")
		}
		q.Result = make([]adsselect.Candidate, len(rows))
		for i, row := range rows {
			q.Result[i] = adsselect.Candidate{
				CampaignID:        row.CampaignID,
				PlacementID:       row.PlacementID,
				CreativeVersionID: row.CreativeVersionID,
				Weight:            row.Weight,
				Advertiser:        row.Advertiser,
			}
		}
		return nil
	})
}

type dbCreativeVersion struct {
	ID         int          `db:"id"`
	CampaignID int          `db:"campaign_id"`
	VersionNo  int          `db:"version_no"`
	ImageURL   string       `db:"image_url"`
	HTML       string       `db:"html"`
	ClickURL   string       `db:"click_url"`
	CreatedAt  sql.NullTime `db:"created_at"`
}

func (r *dbCreativeVersion) toModel() *entity.CreativeVersion {
	v := &entity.CreativeVersion{
		ID: r.ID, CampaignID: r.CampaignID, VersionNo: r.VersionNo,
		ImageURL: r.ImageURL, HTML: r.HTML, ClickURL: r.ClickURL,
	}
	if r.CreatedAt.Valid {
		v.CreatedAt = r.CreatedAt.Time.UTC()
	}
	return v
}

func getCreativeVersionsByIDs(ctx context.Context, q *query.GetCreativeVersionsByIDs) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = map[int]*entity.CreativeVersion{}
		if len(q.IDs) == 0 {
			return nil
		}
		rows := []*dbCreativeVersion{}
		err := trx.Select(&rows, `
			SELECT id, campaign_id, version_no, image_url, html, click_url, created_at
			FROM creative_versions
			WHERE tenant_id = $1 AND id = ANY($2)`,
			tenant.ID, pq.Array(q.IDs))
		if err != nil {
			return errors.Wrap(err, "failed to get creative versions")
		}
		for _, row := range rows {
			q.Result[row.ID] = row.toModel()
		}
		return nil
	})
}

func createCreativeVersion(ctx context.Context, c *cmd.CreateCreativeVersion) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		row := &dbCreativeVersion{}
		err := trx.Get(row, `
			INSERT INTO creative_versions (tenant_id, campaign_id, version_no, image_url, html, click_url)
			VALUES (
				$1, $2,
				(SELECT COALESCE(MAX(version_no), 0) + 1 FROM creative_versions WHERE tenant_id = $1 AND campaign_id = $2),
				$3, $4, $5
			)
			RETURNING id, campaign_id, version_no, image_url, html, click_url, created_at`,
			tenant.ID, c.CampaignID, c.ImageURL, c.HTML, c.ClickURL)
		if err != nil {
			return errors.Wrap(err, "failed to create creative version")
		}
		c.Result = row.toModel()
		return nil
	})
}

func upsertCampaignAssignment(ctx context.Context, c *cmd.UpsertCampaignAssignment) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var id int
		err := trx.Get(&id, `
			INSERT INTO campaign_assignments (tenant_id, campaign_id, placement_id, creative_version_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, campaign_id, placement_id)
			DO UPDATE SET creative_version_id = EXCLUDED.creative_version_id
			RETURNING id`,
			tenant.ID, c.CampaignID, c.PlacementID, c.CreativeVersionID)
		if err != nil {
			return errors.Wrap(err, "failed to upsert campaign assignment")
		}
		c.Result = &entity.CampaignAssignment{
			ID: id, CampaignID: c.CampaignID, PlacementID: c.PlacementID,
			CreativeVersionID: c.CreativeVersionID,
		}
		return nil
	})
}

func listCreativeVersionsByCampaign(ctx context.Context, q *query.ListCreativeVersionsByCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = []*entity.CreativeVersion{}
		rows := []*dbCreativeVersion{}
		err := trx.Select(&rows, `
			SELECT id, campaign_id, version_no, image_url, html, click_url, created_at
			FROM creative_versions
			WHERE tenant_id = $1 AND campaign_id = $2
			ORDER BY version_no DESC`,
			tenant.ID, q.CampaignID)
		if err != nil {
			return errors.Wrap(err, "failed to list creative versions")
		}
		q.Result = make([]*entity.CreativeVersion, len(rows))
		for i, row := range rows {
			q.Result[i] = row.toModel()
		}
		return nil
	})
}

func listCampaignAssignmentsByCampaign(ctx context.Context, q *query.ListCampaignAssignmentsByCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = []*entity.CampaignAssignment{}
		type row struct {
			ID                int    `db:"id"`
			CampaignID        int    `db:"campaign_id"`
			PlacementID       string `db:"placement_id"`
			CreativeVersionID int    `db:"creative_version_id"`
		}
		rows := []*row{}
		err := trx.Select(&rows, `
			SELECT id, campaign_id, placement_id, creative_version_id
			FROM campaign_assignments
			WHERE tenant_id = $1 AND campaign_id = $2
			ORDER BY placement_id ASC`,
			tenant.ID, q.CampaignID)
		if err != nil {
			return errors.Wrap(err, "failed to list campaign assignments")
		}
		q.Result = make([]*entity.CampaignAssignment, len(rows))
		for i, r := range rows {
			q.Result[i] = &entity.CampaignAssignment{
				ID: r.ID, CampaignID: r.CampaignID, PlacementID: r.PlacementID,
				CreativeVersionID: r.CreativeVersionID,
			}
		}
		return nil
	})
}

func deleteCampaignAssignment(ctx context.Context, c *cmd.DeleteCampaignAssignment) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			DELETE FROM campaign_assignments
			WHERE tenant_id = $1 AND campaign_id = $2 AND placement_id = $3`,
			tenant.ID, c.CampaignID, c.PlacementID)
		if err != nil {
			return errors.Wrap(err, "failed to delete campaign assignment")
		}
		return nil
	})
}
