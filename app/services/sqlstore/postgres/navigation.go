package postgres

import (
	"context"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func getNavigationLinks(ctx context.Context, q *query.GetNavigationLinks) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var links []*entity.NavigationLink
		err := trx.Select(&links, `
			SELECT id, title, url, display_order, location, created_at, updated_at
			FROM navigation_links
			WHERE tenant_id = $1
			ORDER BY location, display_order
		`, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get navigation links")
		}

		q.Result = links
		return nil
	})
}

func saveNavigationLinks(ctx context.Context, c *cmd.SaveNavigationLinks) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("DELETE FROM navigation_links WHERE tenant_id = $1", tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete existing navigation links")
		}

		for _, link := range c.Links {
			_, err := trx.Execute(`
				INSERT INTO navigation_links (tenant_id, title, url, display_order, location, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, tenant.ID, link.Title, link.URL, link.DisplayOrder, link.Location, time.Now(), time.Now())

			if err != nil {
				return errors.Wrap(err, "failed to insert navigation link")
			}
		}

		return nil
	})
}

