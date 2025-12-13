package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/lib/pq"
)

func savePushSubscription(ctx context.Context, c *cmd.SavePushSubscription) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil || tenant == nil {
			return errors.New("user and tenant are required")
		}

		_, err := trx.Execute(`
			INSERT INTO push_subscriptions (tenant_id, user_id, endpoint, key_p256dh, key_auth)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id, user_id, endpoint) 
			DO UPDATE SET key_p256dh = $4, key_auth = $5
		`, tenant.ID, user.ID, c.Endpoint, c.KeyP256dh, c.KeyAuth)

		if err != nil {
			return errors.Wrap(err, "failed to save push subscription")
		}
		return nil
	})
}

func deletePushSubscription(ctx context.Context, c *cmd.DeletePushSubscription) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil || tenant == nil {
			return nil
		}

		_, err := trx.Execute(`
			DELETE FROM push_subscriptions 
			WHERE tenant_id = $1 AND user_id = $2 AND endpoint = $3
		`, tenant.ID, user.ID, c.Endpoint)

		if err != nil {
			return errors.Wrap(err, "failed to delete push subscription")
		}
		return nil
	})
}

func deleteAllPushSubscriptions(ctx context.Context, c *cmd.DeleteAllPushSubscriptions) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil || tenant == nil {
			return nil
		}

		_, err := trx.Execute(`
			DELETE FROM push_subscriptions 
			WHERE tenant_id = $1 AND user_id = $2
		`, tenant.ID, user.ID)

		if err != nil {
			return errors.Wrap(err, "failed to delete all push subscriptions")
		}
		return nil
	})
}

func deletePushSubscriptionByEndpoint(ctx context.Context, c *cmd.DeletePushSubscriptionByEndpoint) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if tenant == nil {
			return nil
		}

		_, err := trx.Execute(`
			DELETE FROM push_subscriptions 
			WHERE tenant_id = $1 AND endpoint = $2
		`, tenant.ID, c.Endpoint)

		if err != nil {
			return errors.Wrap(err, "failed to delete push subscription by endpoint")
		}
		return nil
	})
}

func getPushSubscriptionsByUser(ctx context.Context, q *query.GetPushSubscriptionsByUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]*entity.PushSubscription, 0)

		if tenant == nil {
			return nil
		}

		err := trx.Select(&q.Result, `
			SELECT id, tenant_id, user_id, endpoint, key_p256dh, key_auth, created_at
			FROM push_subscriptions
			WHERE tenant_id = $1 AND user_id = $2
		`, tenant.ID, q.UserID)

		if err != nil {
			return errors.Wrap(err, "failed to get push subscriptions for user %d", q.UserID)
		}
		return nil
	})
}

func getPushSubscriptionsByUsers(ctx context.Context, q *query.GetPushSubscriptionsByUsers) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]*entity.PushSubscription, 0)

		if tenant == nil || len(q.UserIDs) == 0 {
			return nil
		}

		err := trx.Select(&q.Result, `
			SELECT id, tenant_id, user_id, endpoint, key_p256dh, key_auth, created_at
			FROM push_subscriptions
			WHERE tenant_id = $1 AND user_id = ANY($2)
		`, tenant.ID, pq.Array(q.UserIDs))

		if err != nil {
			return errors.Wrap(err, "failed to get push subscriptions for users")
		}
		return nil
	})
}

func getAllPushSubscriptions(ctx context.Context, q *query.GetAllPushSubscriptions) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]*entity.PushSubscription, 0)

		if tenant == nil {
			return nil
		}

		err := trx.Select(&q.Result, `
			SELECT id, tenant_id, user_id, endpoint, key_p256dh, key_auth, created_at
			FROM push_subscriptions
			WHERE tenant_id = $1
		`, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get all push subscriptions")
		}
		return nil
	})
}

func hasPushSubscription(ctx context.Context, q *query.HasPushSubscription) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = false

		if tenant == nil {
			return nil
		}

		var exists bool
		err := trx.Scalar(&exists, `
			SELECT EXISTS(
				SELECT 1 FROM push_subscriptions
				WHERE tenant_id = $1 AND user_id = $2
			)
		`, tenant.ID, q.UserID)

		if err != nil {
			return errors.Wrap(err, "failed to check push subscription for user %d", q.UserID)
		}

		q.Result = exists
		return nil
	})
}

