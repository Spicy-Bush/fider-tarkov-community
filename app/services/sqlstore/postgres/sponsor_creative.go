package postgres

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/lib/pq"
)

type sponsorCreativeRecord struct {
	ID         int    `db:"id"`
	CampaignID int    `db:"campaign_id"`
	Revision   int    `db:"revision"`
	ImageKey   string `db:"image_key"`
	LogoKey    string `db:"logo_key"`
	Data       []byte `db:"data"`
}

func (row *sponsorCreativeRecord) creative() (*entity.SponsorCreative, error) {
	creative := new(entity.SponsorCreative)
	if err := json.Unmarshal(row.Data, creative); err != nil {
		return nil, err
	}

	creative.ID = row.ID
	creative.CampaignID = row.CampaignID
	creative.Revision = row.Revision
	creative.ImageKey = row.ImageKey
	creative.LogoKey = row.LogoKey
	return creative, nil
}

func readSponsorCreative(trx *dbx.Trx, tenantID, id int) (*entity.SponsorCreative, error) {
	var row sponsorCreativeRecord
	if err := trx.Get(&row, `
		SELECT id, campaign_id, revision, image_key, logo_key, data
		FROM sponsor_creatives WHERE tenant_id=$1 AND id=$2
	`, tenantID, id); err != nil {
		return nil, err
	}

	return row.creative()
}

func readSponsorCreatives(trx *dbx.Trx, tenantID, campaignID, page int) ([]*entity.SponsorCreative, error) {
	var rows []*sponsorCreativeRecord
	if err := trx.Select(&rows, `
		SELECT id, campaign_id, revision, image_key, logo_key, data
		FROM sponsor_creatives WHERE tenant_id=$1 AND campaign_id=$2
		ORDER BY id DESC LIMIT 26 OFFSET $3
	`, tenantID, campaignID, (page-1)*25); err != nil {
		return nil, err
	}

	result := make([]*entity.SponsorCreative, 0, len(rows))
	for _, row := range rows {
		creative, err := row.creative()
		if err != nil {
			return nil, err
		}

		result = append(result, creative)
	}

	return result, nil
}

func writeSponsorCreative(trx *dbx.Trx, tenantID int, creative entity.SponsorCreative) (*entity.SponsorCreative, error) {
	if creative.ID != 0 {
		previous, err := readSponsorCreative(trx, tenantID, creative.ID)
		if err != nil {
			return nil, err
		}

		if previous.CampaignID != creative.CampaignID {
			return nil, app.ErrNotFound
		}

		if previous.Revision != creative.Revision {
			return nil, app.ErrConflict
		}

		before, after := *previous, creative
		before.State, after.State = "", ""
		before.ReviewReason, after.ReviewReason = "", ""
		if previous.State == "approved" && !reflect.DeepEqual(before, after) {
			// Retain approved artwork while its replacement awaits approval.
			creative.ID = 0
		}
	}

	keys := []string{}
	for _, key := range []string{creative.ImageKey, creative.LogoKey} {
		if key != "" {
			keys = append(keys, key)
		}
	}

	var available []*struct {
		Key string `db:"key"`
	}
	if err := trx.Select(&available, `
		SELECT key FROM media_assets
		WHERE tenant_id=$1 AND key=ANY($2) AND is_public AND storage_source=$3
		  AND deleted_at IS NULL AND deletion_requested_at IS NULL
		ORDER BY key FOR SHARE
	`, tenantID, pq.Array(keys), blob.StorageSource()); err != nil {
		return nil, err
	}

	for _, key := range keys {
		found := false
		for _, stored := range available {
			found = found || stored.Key == key
		}

		if !found {
			return nil, validate.Failed("Choose a public image from this site's library.")
		}
	}

	encoded, err := json.Marshal(creative)
	if err != nil {
		return nil, err
	}

	if creative.ID == 0 {
		creative.Revision = 1
		err = trx.Scalar(&creative.ID, `
			INSERT INTO sponsor_creatives (tenant_id, campaign_id, image_key, logo_key, data)
			VALUES ($1,$2,$3,$4,$5::jsonb - ARRAY['id','revision','campaignId','imageKey','logoKey']) RETURNING id
		`, tenantID, creative.CampaignID, creative.ImageKey, creative.LogoKey, encoded)
	} else {
		creative.Revision++
		_, err = trx.Execute(`
			UPDATE sponsor_creatives SET revision=$3, image_key=$4, logo_key=$5,
			    data=$6::jsonb - ARRAY['id','revision','campaignId','imageKey','logoKey']
			WHERE tenant_id=$1 AND id=$2
		`, tenantID, creative.ID, creative.Revision, creative.ImageKey, creative.LogoKey, encoded)
	}

	return &creative, err
}

func saveSponsorExclusion(ctx context.Context, c *cmd.SaveSponsorExclusion) error {
	if (c.Exclusion.PageType != "post" && c.Exclusion.PageType != "page") || c.Exclusion.ID <= 0 || len(c.Exclusion.Reason) > 500 {
		return validate.Failed("Choose a post or Page and a reason up to 500 characters.")
	}

	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		value := struct {
			Exclusion entity.SponsorExclusion
			Excluded  bool
		}{c.Exclusion, c.Excluded}
		receipt, err := sponsorReceipt(tenant.ID, user.ID, "sponsor-exclusion", c.SubmissionID, value)
		if err != nil {
			return err
		}

		replayed, err := receipt.read(trx, nil)
		if err != nil {
			return err
		}

		readCurrent := func() error {
			c.Result = []*entity.SponsorExclusion{}
			return trx.Select(&c.Result, `
				SELECT page_type, content_id, reason FROM sponsor_exclusions
				WHERE tenant_id=$1 ORDER BY page_type, content_id
			`, tenant.ID)
		}
		if replayed {
			return readCurrent()
		}

		if err := lockSponsorConfiguration(trx, tenant.ID, true); err != nil {
			return err
		}

		var exists bool
		if err := trx.Scalar(&exists, `
			SELECT EXISTS (
				SELECT 1 FROM posts WHERE tenant_id=$1 AND id=$2 AND $3='post'
				UNION ALL SELECT 1 FROM pages WHERE tenant_id=$1 AND id=$2 AND $3='page'
			)
		`, tenant.ID, c.Exclusion.ID, c.Exclusion.PageType); err != nil {
			return err
		}

		if !exists {
			return app.ErrNotFound
		}

		if !c.Excluded {
			_, err = trx.Execute(`DELETE FROM sponsor_exclusions WHERE tenant_id=$1 AND page_type=$2 AND content_id=$3`, tenant.ID, c.Exclusion.PageType, c.Exclusion.ID)
		} else {
			_, err = trx.Execute(`
				INSERT INTO sponsor_exclusions (tenant_id, page_type, content_id, reason) VALUES ($1,$2,$3,$4)
				ON CONFLICT (tenant_id, page_type, content_id) DO UPDATE SET reason=EXCLUDED.reason
			`, tenant.ID, c.Exclusion.PageType, c.Exclusion.ID, c.Exclusion.Reason)
		}

		if err != nil {
			return err
		}

		if err := readCurrent(); err != nil {
			return err
		}

		return receipt.save(trx, nil)
	})
}
