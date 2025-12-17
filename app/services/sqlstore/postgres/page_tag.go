package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pages"
)

func getPageTags(ctx context.Context, q *query.GetPageTags) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		tags := []*entity.PageTag{}
		err := trx.Select(&tags, `
			SELECT id, name, slug
			FROM page_tags
			WHERE tenant_id = $1
			ORDER BY name ASC
		`, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get page tags")
		}

		q.Result = tags
		return nil
	})
}

func getPageTagByID(ctx context.Context, q *query.GetPageTagByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		tag := &entity.PageTag{}
		err := trx.Get(tag, `
			SELECT id, name, slug
			FROM page_tags
			WHERE id = $1 AND tenant_id = $2
		`, q.ID, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get page tag by id")
		}

		q.Result = tag
		return nil
	})
}

func createPageTag(ctx context.Context, c *cmd.CreatePageTag) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		slug, err := pages.GenerateSlug(ctx, c.Name, c.Slug, 0)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		var id int
		err = trx.Get(&id, `
			INSERT INTO page_tags (tenant_id, name, slug)
			VALUES ($1, $2, $3)
			RETURNING id
		`, tenant.ID, c.Name, slug)

		if err != nil {
			return errors.Wrap(err, "failed to create page tag")
		}

		c.Result = &entity.PageTag{
			ID:   id,
			Name: c.Name,
			Slug: slug,
		}

		return nil
	})
}

func updatePageTag(ctx context.Context, c *cmd.UpdatePageTag) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		slug, err := pages.GenerateSlug(ctx, c.Name, c.Slug, c.ID)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		_, err = trx.Execute(`
			UPDATE page_tags
			SET name = $1, slug = $2
			WHERE id = $3 AND tenant_id = $4
		`, c.Name, slug, c.ID, tenant.ID)

		return errors.Wrap(err, "failed to update page tag")
	})
}

func deletePageTag(ctx context.Context, c *cmd.DeletePageTag) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("DELETE FROM page_tags WHERE id = $1 AND tenant_id = $2", c.ID, tenant.ID)
		return errors.Wrap(err, "failed to delete page tag")
	})
}
