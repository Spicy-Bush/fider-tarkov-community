package postgres

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
	"github.com/lib/pq"
)

func schedulePostNotification(ctx context.Context, c *cmd.SchedulePostNotification) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		encoded, err := json.Marshal(c.Post)
		if err != nil {
			return err
		}
		_, err = trx.Execute(`INSERT INTO post_notification_deliveries
			(post_id, tenant_id, user_id, post, base_url, locale) VALUES ($1, $2, $3, $4, $5, $6)`,
			c.Post.ID, tenant.ID, user.ID, string(encoded), c.BaseURL, tenant.Locale)
		return err
	})
}

func processPostNotification(ctx context.Context, c *cmd.ProcessPostNotification) error {
	var deliveryErr error
	err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		var delivery struct {
			PostID           int    `db:"post_id"`
			TenantID         int    `db:"tenant_id"`
			UserID           int    `db:"user_id"`
			Post             string `db:"post"`
			BaseURL          string `db:"base_url"`
			Locale           string `db:"locale"`
			Channel          string `db:"channel"`
			RecipientID      int    `db:"recipient_id"`
			SendIndividually bool   `db:"send_individually"`
		}
		err := trx.Get(&delivery, `SELECT post_id, tenant_id, user_id, post::text, base_url, locale,
			'' AS channel, 0 AS recipient_id, FALSE AS send_individually FROM post_notification_deliveries
			WHERE NOT prepared AND available_at <= NOW()
			ORDER BY available_at, post_id LIMIT 1 FOR UPDATE SKIP LOCKED`)
		if errors.Cause(err) == app.ErrNotFound {
			err = trx.Get(&delivery, `SELECT p.post_id, p.tenant_id, p.user_id, p.post::text, p.base_url, p.locale,
				r.channel, r.recipient_id, r.send_individually FROM post_notification_recipients r
				JOIN post_notification_deliveries p ON p.post_id = r.post_id
				WHERE r.available_at <= NOW() ORDER BY r.available_at, r.post_id, r.channel, r.recipient_id
				LIMIT 1 FOR UPDATE OF r SKIP LOCKED`)
		}
		if errors.Cause(err) == app.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		c.Found = true
		ids := []int64{int64(delivery.RecipientID)}
		recipients := []cmd.PostNotificationRecipient{{Channel: delivery.Channel, ID: delivery.RecipientID}}
		if delivery.Channel == "email" && !delivery.SendIndividually && c.EmailBatchSize > 1 {
			rows, err := trx.Query(`SELECT recipient_id FROM post_notification_recipients
				WHERE post_id = $1 AND channel = 'email' AND recipient_id <> $2
				AND NOT send_individually AND available_at <= NOW()
				ORDER BY recipient_id LIMIT $3 FOR UPDATE SKIP LOCKED`,
				delivery.PostID, delivery.RecipientID, c.EmailBatchSize - 1)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					return err
				}
				ids = append(ids, id)
				recipients = append(recipients, cmd.PostNotificationRecipient{Channel: "email", ID: int(id)})
			}
			if err := rows.Err(); err != nil {
				return err
			}
		}
		if _, err := trx.Execute("SAVEPOINT notification_delivery"); err != nil {
			return err
		}
		deliveryErr = func() (err error) {
			defer func() {
				if cause := recover(); cause != nil {
					err = errors.Panicked(cause)
				}
			}()
			var status enum.PostStatus
			if err := trx.Scalar(&status, "SELECT status FROM posts WHERE id = $1", delivery.PostID); err != nil {
				return err
			}
			if status == enum.PostDeleted {
				return completePostNotification(trx, delivery.PostID, delivery.Channel, ids)
			}
			baseURL, err := url.Parse(delivery.BaseURL)
			if err != nil {
				return err
			}
			ctx = context.WithValue(ctx, app.RequestCtxKey, web.Request{URL: baseURL})
			var domain string
			if err := trx.Scalar(&domain, "SELECT subdomain FROM tenants WHERE id = $1", delivery.TenantID); err != nil {
				return err
			}
			tenant := &query.GetTenantByDomain{Domain: domain}
			if err := getTenantByDomain(ctx, tenant); err != nil {
				return err
			}
			ctx = context.WithValue(ctx, app.TenantCtxKey, tenant.Result)
			var author dbUser
			if err := trx.Get(&author, `SELECT id, name, email, role, status FROM users
				WHERE id = $1 AND tenant_id = $2`, delivery.UserID, delivery.TenantID); err != nil {
				return err
			}
			ctx = context.WithValue(ctx, app.UserCtxKey, author.toModel(ctx))
			ctx = context.WithValue(ctx, app.LocaleCtxKey, delivery.Locale)
			var post entity.Post
			if err := json.Unmarshal([]byte(delivery.Post), &post); err != nil {
				return err
			}
			if delivery.Channel == "" {
				recipients, err := c.Prepare(ctx, &post)
				if err != nil {
					return err
				}
				channels := make([]string, len(recipients))
				ids := make([]int64, len(recipients))
				for i, recipient := range recipients {
					channels[i], ids[i] = recipient.Channel, int64(recipient.ID)
				}
				if _, err := trx.Execute(`INSERT INTO post_notification_recipients (post_id, tenant_id, channel, recipient_id)
					SELECT $1, $4, * FROM UNNEST($2::text[], $3::bigint[]) ON CONFLICT DO NOTHING`,
					delivery.PostID, pq.Array(channels), pq.Array(ids), delivery.TenantID); err != nil {
					return err
				}
				if _, err := trx.Execute("UPDATE post_notification_deliveries SET prepared = TRUE, last_error = NULL WHERE post_id = $1", delivery.PostID); err != nil {
					return err
				}
			} else {
				if err := c.Send(ctx, &post, recipients); err != nil {
					return err
				}
			}
			return completePostNotification(trx, delivery.PostID, delivery.Channel, ids)
		}()
		if deliveryErr == nil {
			return nil
		}
		if _, err := trx.Execute("ROLLBACK TO SAVEPOINT notification_delivery"); err != nil {
			return err
		}
		if delivery.Channel == "" {
			_, err = trx.Execute(`UPDATE post_notification_deliveries SET attempts = attempts + 1, last_error = $2,
				available_at = NOW() + LEAST(3600, POWER(2, LEAST(attempts + 1, 12))) * INTERVAL '1 second'
				WHERE post_id = $1`, delivery.PostID, deliveryErr.Error())
		} else {
			_, err = trx.Execute(`UPDATE post_notification_recipients SET attempts = attempts + 1, last_error = $4,
				send_individually = send_individually OR $5,
				available_at = NOW() + LEAST(3600, POWER(2, LEAST(attempts + 1, 12))) * INTERVAL '1 second'
				WHERE post_id = $1 AND channel = $2 AND recipient_id = ANY($3)`,
				delivery.PostID, delivery.Channel, pq.Array(ids), deliveryErr.Error(), email.IsRecipientRejected(deliveryErr))
		}
		return err
	})
	if err != nil {
		return err
	}
	return deliveryErr
}

func completePostNotification(trx *dbx.Trx, postID int, channel string, recipientIDs []int64) error {
	if _, err := trx.Execute("SELECT post_id FROM post_notification_deliveries WHERE post_id = $1 FOR UPDATE", postID); err != nil {
		return err
	}
	if channel != "" {
		if _, err := trx.Execute(`DELETE FROM post_notification_recipients
			WHERE post_id = $1 AND channel = $2 AND recipient_id = ANY($3)`, postID, channel, pq.Array(recipientIDs)); err != nil {
			return err
		}
	}
	_, err := trx.Execute(`DELETE FROM post_notification_deliveries p WHERE post_id = $1
		AND NOT EXISTS (SELECT 1 FROM post_notification_recipients r WHERE r.post_id = p.post_id)`, postID)
	return err
}
