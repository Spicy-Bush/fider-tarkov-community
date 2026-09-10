package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/rand"
	"strings"
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
	ID                   int           `db:"id"`
	Name                 string        `db:"name"`
	Advertiser           string        `db:"advertiser"`
	Slots                string        `db:"slots"`
	CreativeImageURL     string        `db:"creative_image_url"`
	CreativeImageURLsRaw string        `db:"creative_image_urls"`
	CreativeHTML         string        `db:"creative_html"`
	ClickURL             string        `db:"click_url"`
	StartAt              time.Time     `db:"start_at"`
	EndAt                time.Time     `db:"end_at"`
	Weight               int           `db:"weight"`
	Locale               string        `db:"locale"`
	Enabled              bool          `db:"enabled"`
	Clicks               int           `db:"clicks"`
	PackageID            sql.NullInt64 `db:"package_id"`
	ConfigVersion        int           `db:"config_version"`
	CreatedAt            time.Time     `db:"created_at"`
	UpdatedAt            time.Time     `db:"updated_at"`
}

func parseCreativeImageURLs(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" || raw == "null" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = map[string]string{}
	}
	return out
}

func marshalCreativeImageURLs(m map[string]string) (string, error) {
	if m == nil {
		m = map[string]string{}
	}
	clean := map[string]string{}
	for k, v := range m {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		clean[k] = v
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

func (r *dbCampaign) toModel() *entity.SponsorshipCampaign {
	c := &entity.SponsorshipCampaign{
		ID: r.ID, Name: r.Name, Advertiser: r.Advertiser, Slots: r.Slots,
		CreativeImageURL:  r.CreativeImageURL,
		CreativeImageURLs: parseCreativeImageURLs(r.CreativeImageURLsRaw),
		CreativeHTML:      r.CreativeHTML,
		ClickURL:          r.ClickURL, StartAt: r.StartAt.UTC(), EndAt: r.EndAt.UTC(),
		Weight: r.Weight, Locale: r.Locale, Enabled: r.Enabled, Clicks: r.Clicks,
		ConfigVersion: r.ConfigVersion,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.PackageID.Valid {
		id := int(r.PackageID.Int64)
		c.PackageID = &id
	}
	return c
}

const campaignSelect = `
	SELECT id, name, advertiser, slots, creative_image_url, creative_image_urls, creative_html, click_url,
	       start_at, end_at, weight, locale, enabled, clicks, package_id, config_version, created_at, updated_at
	FROM sponsorship_campaigns`

func listSponsorshipCampaigns(ctx context.Context, q *query.ListSponsorshipCampaigns) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		rows := []*dbCampaign{}
		err := trx.Select(&rows, campaignSelect+` WHERE tenant_id = $1 ORDER BY start_at DESC, id DESC`, tenant.ID)
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
		var id int
		now := time.Now().UTC()
		start := c.StartAt.UTC()
		end := c.EndAt.UTC()
		urlsJSON, err := marshalCreativeImageURLs(c.CreativeImageURLs)
		if err != nil {
			return errors.Wrap(err, "failed to encode creative image urls")
		}
		err = trx.Get(&id, `
			INSERT INTO sponsorship_campaigns (
				tenant_id, name, advertiser, slots, creative_image_url, creative_image_urls, creative_html, click_url,
				start_at, end_at, weight, locale, enabled, clicks, package_id, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,$13,0,$14,$15,$15) RETURNING id`,
			tenant.ID, c.Name, c.Advertiser, c.Slots, c.CreativeImageURL, urlsJSON, c.CreativeHTML, c.ClickURL,
			start, end, c.Weight, c.Locale, c.Enabled, c.PackageID, now)
		if err != nil {
			return errors.Wrap(err, "failed to create sponsorship campaign")
		}
		c.Result = &entity.SponsorshipCampaign{
			ID: id, Name: c.Name, Advertiser: c.Advertiser, Slots: c.Slots,
			CreativeImageURL: c.CreativeImageURL, CreativeImageURLs: parseCreativeImageURLs(urlsJSON),
			CreativeHTML: c.CreativeHTML,
			ClickURL:     c.ClickURL, StartAt: start, EndAt: end,
			Weight: c.Weight, Locale: c.Locale, Enabled: c.Enabled, Clicks: 0,
			PackageID: c.PackageID, ConfigVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		return nil
	})
}

func updateSponsorshipCampaign(ctx context.Context, c *cmd.UpdateSponsorshipCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		now := time.Now().UTC()
		start := c.StartAt.UTC()
		end := c.EndAt.UTC()
		urlsJSON, err := marshalCreativeImageURLs(c.CreativeImageURLs)
		if err != nil {
			return errors.Wrap(err, "failed to encode creative image urls")
		}
		var rows int64
		if c.ConfigVersion > 0 {
			rows, err = trx.Execute(`
				UPDATE sponsorship_campaigns SET
					name=$1, advertiser=$2, slots=$3, creative_image_url=$4, creative_image_urls=$5::jsonb, creative_html=$6, click_url=$7,
					start_at=$8, end_at=$9, weight=$10, locale=$11, enabled=$12, package_id=$13, updated_at=$14,
					config_version = config_version + 1
				WHERE id=$15 AND tenant_id=$16 AND config_version=$17`,
				c.Name, c.Advertiser, c.Slots, c.CreativeImageURL, urlsJSON, c.CreativeHTML, c.ClickURL,
				start, end, c.Weight, c.Locale, c.Enabled, c.PackageID, now, c.ID, tenant.ID, c.ConfigVersion)
			if err != nil {
				return errors.Wrap(err, "failed to update sponsorship campaign")
			}
			if rows == 0 {
				return app.ErrConflict
			}
			c.Result = &entity.SponsorshipCampaign{
				ID: c.ID, Name: c.Name, Advertiser: c.Advertiser, Slots: c.Slots,
				CreativeImageURL: c.CreativeImageURL, CreativeImageURLs: parseCreativeImageURLs(urlsJSON),
				CreativeHTML: c.CreativeHTML,
				ClickURL:     c.ClickURL, StartAt: start, EndAt: end,
				Weight: c.Weight, Locale: c.Locale, Enabled: c.Enabled,
				PackageID: c.PackageID, ConfigVersion: c.ConfigVersion + 1, UpdatedAt: now,
			}
			return nil
		}
		// Dual-read: legacy admin clients omit config_version — still bump OCC token.
		rows, err = trx.Execute(`
			UPDATE sponsorship_campaigns SET
				name=$1, advertiser=$2, slots=$3, creative_image_url=$4, creative_image_urls=$5::jsonb, creative_html=$6, click_url=$7,
				start_at=$8, end_at=$9, weight=$10, locale=$11, enabled=$12, package_id=$13, updated_at=$14,
				config_version = config_version + 1
			WHERE id=$15 AND tenant_id=$16`,
			c.Name, c.Advertiser, c.Slots, c.CreativeImageURL, urlsJSON, c.CreativeHTML, c.ClickURL,
			start, end, c.Weight, c.Locale, c.Enabled, c.PackageID, now, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update sponsorship campaign")
		}
		if rows == 0 {
			return app.ErrNotFound
		}
		c.Result = &entity.SponsorshipCampaign{
			ID: c.ID, Name: c.Name, Advertiser: c.Advertiser, Slots: c.Slots,
			CreativeImageURL: c.CreativeImageURL, CreativeImageURLs: parseCreativeImageURLs(urlsJSON),
			CreativeHTML: c.CreativeHTML,
			ClickURL:     c.ClickURL, StartAt: start, EndAt: end,
			Weight: c.Weight, Locale: c.Locale, Enabled: c.Enabled,
			PackageID: c.PackageID, UpdatedAt: now,
		}
		return nil
	})
}

func deleteSponsorshipCampaign(ctx context.Context, c *cmd.DeleteSponsorshipCampaign) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`DELETE FROM sponsorship_campaigns WHERE id=$1 AND tenant_id=$2`, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete sponsorship campaign")
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

func campaignHasSlot(slotsCSV, slotID string) bool {
	for _, s := range strings.Split(slotsCSV, ",") {
		if strings.TrimSpace(s) == slotID {
			return true
		}
	}
	return false
}

func pickWeighted(rows []*dbCampaign) *dbCampaign {
	if len(rows) == 0 {
		return nil
	}
	total := 0
	for _, row := range rows {
		w := row.Weight
		if w < 1 {
			w = 1
		}
		total += w
	}
	pick := rand.Intn(total)
	running := 0
	for _, row := range rows {
		w := row.Weight
		if w < 1 {
			w = 1
		}
		running += w
		if pick < running {
			return row
		}
	}
	return rows[len(rows)-1]
}

func getActiveSponsorshipForSlot(ctx context.Context, q *query.GetActiveSponsorshipForSlot) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = nil
		rows := []*dbCampaign{}
		now := time.Now().UTC()
		err := trx.Select(&rows, campaignSelect+`
			WHERE tenant_id = $1
			  AND enabled = true
			  AND start_at <= $2
			  AND end_at > $2
			  AND (locale = 'all' OR locale = $3)
			ORDER BY weight DESC, id ASC`, tenant.ID, now, q.Locale)
		if err != nil {
			return errors.Wrap(err, "failed to query active sponsorship")
		}
		matched := make([]*dbCampaign, 0)
		for _, row := range rows {
			if campaignHasSlot(row.Slots, q.SlotID) {
				matched = append(matched, row)
			}
		}
		picked := pickWeighted(matched)
		if picked == nil {
			return nil
		}
		q.Result = picked.toModel()
		return nil
	})
}

func getActiveSponsorshipForSlots(ctx context.Context, q *query.GetActiveSponsorshipForSlots) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = map[string]*entity.SponsorshipCampaign{}
		for _, slot := range q.SlotIDs {
			q.Result[slot] = nil
		}
		if len(q.SlotIDs) == 0 {
			return nil
		}
		rows := []*dbCampaign{}
		now := time.Now().UTC()
		err := trx.Select(&rows, campaignSelect+`
			WHERE tenant_id = $1
			  AND enabled = true
			  AND start_at <= $2
			  AND end_at > $2
			  AND (locale = 'all' OR locale = $3)
			ORDER BY weight DESC, id ASC`, tenant.ID, now, q.Locale)
		if err != nil {
			return errors.Wrap(err, "failed to query active sponsorships")
		}
		for _, slot := range q.SlotIDs {
			matched := make([]*dbCampaign, 0)
			for _, row := range rows {
				if campaignHasSlot(row.Slots, slot) {
					matched = append(matched, row)
				}
			}
			if picked := pickWeighted(matched); picked != nil {
				q.Result[slot] = picked.toModel()
			}
		}
		return nil
	})
}
