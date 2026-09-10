package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func listSponsorshipPackages(ctx context.Context, q *query.ListSponsorshipPackages) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = []*entity.SponsorshipPackage{}
		err := trx.Select(&q.Result, `
			SELECT id, slug, name, description, slots, duration_days, sort, created_at
			FROM sponsorship_packages
			WHERE tenant_id = $1
			ORDER BY sort ASC, id ASC`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to list sponsorship packages")
		}
		return nil
	})
}

func getSponsorshipPackageByID(ctx context.Context, q *query.GetSponsorshipPackageByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		pkg := &entity.SponsorshipPackage{}
		err := trx.Get(pkg, `
			SELECT id, slug, name, description, slots, duration_days, sort, created_at
			FROM sponsorship_packages WHERE id = $1 AND tenant_id = $2`, q.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get sponsorship package")
		}
		q.Result = pkg
		return nil
	})
}

func createSponsorshipPackage(ctx context.Context, c *cmd.CreateSponsorshipPackage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var id int
		now := time.Now().UTC()
		err := trx.Get(&id, `
			INSERT INTO sponsorship_packages (tenant_id, slug, name, description, slots, duration_days, sort, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			tenant.ID, c.Slug, c.Name, c.Description, c.Slots, c.DurationDays, c.Sort, now)
		if err != nil {
			return errors.Wrap(err, "failed to create sponsorship package")
		}
		c.Result = &entity.SponsorshipPackage{
			ID: id, Slug: c.Slug, Name: c.Name, Description: c.Description,
			Slots: c.Slots, DurationDays: c.DurationDays, Sort: c.Sort, CreatedAt: now,
		}
		return nil
	})
}

func updateSponsorshipPackage(ctx context.Context, c *cmd.UpdateSponsorshipPackage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE sponsorship_packages
			SET slug=$1, name=$2, description=$3, slots=$4, duration_days=$5, sort=$6
			WHERE id=$7 AND tenant_id=$8`,
			c.Slug, c.Name, c.Description, c.Slots, c.DurationDays, c.Sort, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update sponsorship package")
		}
		c.Result = &entity.SponsorshipPackage{
			ID: c.ID, Slug: c.Slug, Name: c.Name, Description: c.Description,
			Slots: c.Slots, DurationDays: c.DurationDays, Sort: c.Sort,
		}
		return nil
	})
}

func deleteSponsorshipPackage(ctx context.Context, c *cmd.DeleteSponsorshipPackage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`DELETE FROM sponsorship_packages WHERE id=$1 AND tenant_id=$2`, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete sponsorship package")
		}
		return nil
	})
}

type dbCampaign struct {
	ID            int           `db:"id"`
	Name          string        `db:"name"`
	Advertiser    string        `db:"advertiser"`
	StartAt       time.Time     `db:"start_at"`
	EndAt         time.Time     `db:"end_at"`
	Weight        int           `db:"weight"`
	Locale        string        `db:"locale"`
	Enabled       bool          `db:"enabled"`
	Clicks        int           `db:"clicks"`
	PackageID     sql.NullInt64 `db:"package_id"`
	ConfigVersion int           `db:"config_version"`
	CreatedAt     time.Time     `db:"created_at"`
	UpdatedAt     time.Time     `db:"updated_at"`
}

func (r *dbCampaign) toModel() *entity.SponsorshipCampaign {
	c := &entity.SponsorshipCampaign{
		ID: r.ID, Name: r.Name, Advertiser: r.Advertiser,
		StartAt: r.StartAt.UTC(), EndAt: r.EndAt.UTC(),
		Weight: r.Weight, Locale: r.Locale, Enabled: r.Enabled, Clicks: r.Clicks,
		ConfigVersion: r.ConfigVersion,
		CreatedAt:     r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.PackageID.Valid {
		id := int(r.PackageID.Int64)
		c.PackageID = &id
	}
	return c
}

const campaignSelect = `
	SELECT id, name, advertiser,
	       start_at, end_at, weight, locale, enabled, clicks, package_id, config_version, created_at, updated_at
	FROM sponsorship_campaigns`

func listSponsorshipCampaigns(ctx context.Context, q *query.ListSponsorshipCampaigns) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		rows := []*dbCampaign{}
		err := trx.Select(&rows, campaignSelect+` WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY start_at DESC, id DESC`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to list sponsorship campaigns")
		}
		q.Result = make([]*entity.SponsorshipCampaign, len(rows))
		for i, row := range rows {
			q.Result[i] = row.toModel()
		}
		return nil
	})
}

func getSponsorshipCampaignByID(ctx context.Context, q *query.GetSponsorshipCampaignByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		row := &dbCampaign{}
		err := trx.Get(row, campaignSelect+` WHERE id = $1 AND tenant_id = $2`, q.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get sponsorship campaign")
		}
		q.Result = row.toModel()
		return nil
	})
}

func createSponsorshipCampaign(ctx context.Context, c *cmd.CreateSponsorshipCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		now := time.Now().UTC()
		start := c.StartAt.UTC()
		end := c.EndAt.UTC()
		row := &dbCampaign{}
		err := trx.Get(row, `
			INSERT INTO sponsorship_campaigns (
				tenant_id, name, advertiser,
				start_at, end_at, weight, locale, enabled, clicks, package_id, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,$9,$10,$10)
			RETURNING id, name, advertiser, start_at, end_at, weight, locale, enabled, clicks,
			          package_id, config_version, created_at, updated_at`,
			tenant.ID, c.Name, c.Advertiser,
			start, end, c.Weight, c.Locale, c.Enabled, c.PackageID, now)
		if err != nil {
			return errors.Wrap(err, "failed to create sponsorship campaign")
		}
		c.Result = row.toModel()
		c.AssignmentResults = []*entity.CampaignAssignment{}

		if c.Version == nil {
			if len(c.Assignments) > 0 {
				return errors.New("assignments require an initial version on create")
			}
			return nil
		}

		verRow := &dbCreativeVersion{}
		err = trx.Get(verRow, `
			INSERT INTO creative_versions (tenant_id, campaign_id, version_no, image_url, html, click_url)
			VALUES ($1, $2, 1, $3, $4, $5)
			RETURNING id, campaign_id, version_no, image_url, html, click_url, created_at`,
			tenant.ID, c.Result.ID, c.Version.ImageURL, c.Version.HTML, c.Version.ClickURL)
		if err != nil {
			return errors.Wrap(err, "failed to create initial creative version")
		}
		c.VersionResult = verRow.toModel()

		for _, a := range c.Assignments {
			vid := a.CreativeVersionID
			if vid <= 0 {
				vid = c.VersionResult.ID
			}
			if vid != c.VersionResult.ID {
				return app.ErrNotFound
			}
			var id int
			err := trx.Get(&id, `
				INSERT INTO campaign_assignments (tenant_id, campaign_id, placement_id, creative_version_id)
				VALUES ($1, $2, $3, $4)
				RETURNING id`,
				tenant.ID, c.Result.ID, a.PlacementID, vid)
			if err != nil {
				return errors.Wrap(err, "failed to insert initial campaign assignment")
			}
			c.AssignmentResults = append(c.AssignmentResults, &entity.CampaignAssignment{
				ID: id, CampaignID: c.Result.ID, PlacementID: a.PlacementID,
				CreativeVersionID: vid,
			})
		}
		return nil
	})
}

func updateSponsorshipCampaign(ctx context.Context, c *cmd.UpdateSponsorshipCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		now := time.Now().UTC()
		start := c.StartAt.UTC()
		end := c.EndAt.UTC()
		if c.ConfigVersion <= 0 {
			return app.ErrConflict
		}
		row := &dbCampaign{}
		err := trx.Get(row, `
			UPDATE sponsorship_campaigns SET
				name=$1, advertiser=$2,
				start_at=$3, end_at=$4, weight=$5, locale=$6, enabled=$7, package_id=$8, updated_at=$9,
				config_version = config_version + 1
			WHERE id=$10 AND tenant_id=$11 AND config_version=$12 AND deleted_at IS NULL
			RETURNING id, name, advertiser, start_at, end_at, weight, locale, enabled, clicks,
			          package_id, config_version, created_at, updated_at`,
			c.Name, c.Advertiser,
			start, end, c.Weight, c.Locale, c.Enabled, c.PackageID, now, c.ID, tenant.ID, c.ConfigVersion)
		if err != nil {
			if errors.Cause(err) == app.ErrNotFound {
				return app.ErrConflict
			}
			return errors.Wrap(err, "failed to update sponsorship campaign")
		}
		c.Result = row.toModel()
		return nil
	})
}

func deleteSponsorshipCampaign(ctx context.Context, c *cmd.DeleteSponsorshipCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		// Soft-delete: keep creative_versions (RESTRICT FK). Selection skips deleted_at.
		_, err := trx.Execute(`
			UPDATE sponsorship_campaigns
			SET deleted_at = $3, enabled = false, updated_at = $3
			WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`,
			c.ID, tenant.ID, time.Now().UTC())
		if err != nil {
			return errors.Wrap(err, "failed to soft-delete sponsorship campaign")
		}
		return nil
	})
}

func incrementSponsorshipClick(ctx context.Context, c *cmd.IncrementSponsorshipClick) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE sponsorship_campaigns SET clicks = clicks + 1, updated_at = $3
			WHERE id=$1 AND tenant_id=$2`, c.ID, tenant.ID, time.Now().UTC())
		if err != nil {
			return errors.Wrap(err, "failed to increment sponsorship click")
		}
		return nil
	})
}
