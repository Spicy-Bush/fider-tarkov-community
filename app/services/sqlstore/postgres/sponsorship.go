package postgres

import (
	"context"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/lib/pq"
)

func listSponsorshipPackages(ctx context.Context, q *query.ListSponsorshipPackages) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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

func createSponsorshipPackage(ctx context.Context, c *cmd.CreateSponsorshipPackage) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		value := entity.SponsorshipPackage{
			Slug: c.Slug, Name: c.Name, Description: c.Description,
			Slots: c.Slots, DurationDays: c.DurationDays, Sort: c.Sort,
		}
		receipt, err := sponsorReceipt(tenant.ID, user.ID, "sponsor-package-create", c.SubmissionID, value)
		if err != nil {
			return err
		}

		var id int
		replayed, err := receipt.read(trx, &id)
		if err != nil {
			return err
		}

		if replayed {
			c.Result = new(entity.SponsorshipPackage)
			return readSponsorshipPackage(trx, tenant.ID, id, c.Result)
		}

		now := time.Now().UTC()
		err = trx.Get(&id, `
			INSERT INTO sponsorship_packages (tenant_id, slug, name, description, slots, duration_days, sort, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			tenant.ID, c.Slug, c.Name, c.Description, c.Slots, c.DurationDays, c.Sort, now)
		if err != nil {
			if cause, ok := errors.Cause(err).(*pq.Error); ok && cause.Constraint == "uq_sponsorship_packages_tenant_slug" {
				return validate.Failed("Choose a slug that is not already used by another package.")
			}

			return errors.Wrap(err, "failed to create sponsorship package")
		}
		c.Result = &entity.SponsorshipPackage{
			ID: id, Slug: c.Slug, Name: c.Name, Description: c.Description,
			Slots: c.Slots, DurationDays: c.DurationDays, Sort: c.Sort, CreatedAt: now,
		}
		return receipt.save(trx, id)
	})
}

func updateSponsorshipPackage(ctx context.Context, c *cmd.UpdateSponsorshipPackage) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		value := entity.SponsorshipPackage{
			ID: c.ID, Slug: c.Slug, Name: c.Name, Description: c.Description,
			Slots: c.Slots, DurationDays: c.DurationDays, Sort: c.Sort,
		}
		receipt, err := sponsorReceipt(tenant.ID, user.ID, "sponsor-package-update", c.SubmissionID, value)
		if err != nil {
			return err
		}

		replayed, err := receipt.read(trx, nil)
		if err != nil {
			return err
		}

		if replayed {
			c.Result = new(entity.SponsorshipPackage)
			return readSponsorshipPackage(trx, tenant.ID, c.ID, c.Result)
		}

		_, err = trx.Execute(`
			UPDATE sponsorship_packages
			SET slug=$1, name=$2, description=$3, slots=$4, duration_days=$5, sort=$6
			WHERE id=$7 AND tenant_id=$8`,
			c.Slug, c.Name, c.Description, c.Slots, c.DurationDays, c.Sort, c.ID, tenant.ID)
		if err != nil {
			if cause, ok := errors.Cause(err).(*pq.Error); ok && cause.Constraint == "uq_sponsorship_packages_tenant_slug" {
				return validate.Failed("Choose a slug that is not already used by another package.")
			}

			return errors.Wrap(err, "failed to update sponsorship package")
		}
		c.Result = new(entity.SponsorshipPackage)
		if err := readSponsorshipPackage(trx, tenant.ID, c.ID, c.Result); err != nil {
			return err
		}

		return receipt.save(trx, nil)
	})
}

func readSponsorshipPackage(trx *dbx.Trx, tenantID, id int, result *entity.SponsorshipPackage) error {
	return trx.Get(result, `
		SELECT id, slug, name, description, slots, duration_days, sort, created_at
		FROM sponsorship_packages WHERE tenant_id=$1 AND id=$2
	`, tenantID, id)
}

func deleteSponsorshipPackage(ctx context.Context, c *cmd.DeleteSponsorshipPackage) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := permissionContext(ctx, trx, tenant, user, entity.ManageSponsorship); err != nil {
			return err
		}

		_, err := trx.Execute(`DELETE FROM sponsorship_packages WHERE id=$1 AND tenant_id=$2`, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete sponsorship package")
		}
		return nil
	})
}
