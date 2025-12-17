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

func getPageTopics(ctx context.Context, q *query.GetPageTopics) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		topics := []*entity.PageTopic{}
		err := trx.Select(&topics, `
			SELECT id, name, slug, description, color
			FROM page_topics
			WHERE tenant_id = $1
			ORDER BY name ASC
		`, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get page topics")
		}

		q.Result = topics
		return nil
	})
}

func getPageTopicByID(ctx context.Context, q *query.GetPageTopicByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		topic := &entity.PageTopic{}
		err := trx.Get(topic, `
			SELECT id, name, slug, description, color
			FROM page_topics
			WHERE id = $1 AND tenant_id = $2
		`, q.ID, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get page topic by id")
		}

		q.Result = topic
		return nil
	})
}

func createPageTopic(ctx context.Context, c *cmd.CreatePageTopic) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		slug, err := pages.GenerateSlug(ctx, c.Name, c.Slug, 0)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		var id int
		err = trx.Get(&id, `
			INSERT INTO page_topics (tenant_id, name, slug, description, color)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, tenant.ID, c.Name, slug, c.Description, c.Color)

		if err != nil {
			return errors.Wrap(err, "failed to create page topic")
		}

		c.Result = &entity.PageTopic{
			ID:          id,
			Name:        c.Name,
			Slug:        slug,
			Description: c.Description,
			Color:       c.Color,
		}

		return nil
	})
}

func updatePageTopic(ctx context.Context, c *cmd.UpdatePageTopic) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		slug, err := pages.GenerateSlug(ctx, c.Name, c.Slug, c.ID)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		_, err = trx.Execute(`
			UPDATE page_topics
			SET name = $1, slug = $2, description = $3, color = $4
			WHERE id = $5 AND tenant_id = $6
		`, c.Name, slug, c.Description, c.Color, c.ID, tenant.ID)

		return errors.Wrap(err, "failed to update page topic")
	})
}

func deletePageTopic(ctx context.Context, c *cmd.DeletePageTopic) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("DELETE FROM page_topics WHERE id = $1 AND tenant_id = $2", c.ID, tenant.ID)
		return errors.Wrap(err, "failed to delete page topic")
	})
}

