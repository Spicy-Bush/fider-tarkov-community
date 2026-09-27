package postgres

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
	"github.com/lib/pq"
)

func scheduleNotification(ctx context.Context, c *cmd.ScheduleNotification) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		event := entity.NotificationDelivery{Post: c.Post, Comment: c.Comment}
		encoded, err := json.Marshal(event)
		if err != nil {
			return err
		}

		var postID, commentID *int
		if c.Post != nil {
			postID = &c.Post.ID
		}

		if c.Comment != nil {
			commentID = &c.Comment.CommentID
		}

		_, err = trx.Execute(`
            INSERT INTO notification_deliveries (post_id, comment_id, tenant_id, user_id, payload, base_url, locale)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
        `, postID, commentID, tenant.ID, user.ID, string(encoded), c.BaseURL, tenant.Locale)
		return err
	})
}

func processNotification(ctx context.Context, c *cmd.ProcessNotification) error {
	c.Found = false
	var deliveryErr error

	err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		var delivery struct {
			ID               int64  `db:"id"`
			TenantID         int    `db:"tenant_id"`
			UserID           int    `db:"user_id"`
			Payload          string `db:"payload"`
			BaseURL          string `db:"base_url"`
			Locale           string `db:"locale"`
			Channel          string `db:"channel"`
			RecipientID      int    `db:"recipient_id"`
			SendIndividually bool   `db:"send_individually"`
		}

		err := trx.Get(&delivery, `
            SELECT id, tenant_id, user_id, payload::text, base_url, locale,
                   '' AS channel, 0 AS recipient_id, FALSE AS send_individually
            FROM notification_deliveries
            WHERE NOT prepared AND available_at <= NOW()
            ORDER BY available_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
        `)
		if errors.Cause(err) == app.ErrNotFound {
			err = trx.Get(&delivery, `
                SELECT delivery.id, delivery.tenant_id, delivery.user_id, delivery.payload::text,
                       delivery.base_url, delivery.locale, recipient.channel,
                       recipient.recipient_id, recipient.send_individually
                FROM notification_recipients recipient
                JOIN notification_deliveries delivery ON delivery.id = recipient.delivery_id
                WHERE recipient.available_at <= NOW()
                ORDER BY recipient.available_at, recipient.delivery_id, recipient.channel, recipient.recipient_id
                LIMIT 1 FOR UPDATE OF recipient SKIP LOCKED
            `)
		}

		if errors.Cause(err) == app.ErrNotFound {
			return nil
		}

		if err != nil {
			return err
		}

		c.Found = true
		ids := []int64{int64(delivery.RecipientID)}
		recipients := []cmd.NotificationRecipient{{Channel: delivery.Channel, ID: delivery.RecipientID}}

		if delivery.Channel == "email" && !delivery.SendIndividually && c.EmailBatchSize > 1 {
			rows, err := trx.Query(`
                SELECT recipient_id FROM notification_recipients
                WHERE delivery_id = $1 AND channel = 'email' AND recipient_id <> $2
                  AND NOT send_individually AND available_at <= NOW()
                ORDER BY recipient_id LIMIT $3 FOR UPDATE SKIP LOCKED
            `, delivery.ID, delivery.RecipientID, c.EmailBatchSize - 1)
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
				recipients = append(recipients, cmd.NotificationRecipient{Channel: "email", ID: int(id)})
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

			var event entity.NotificationDelivery
			if err := json.Unmarshal([]byte(delivery.Payload), &event); err != nil {
				return err
			}

			var removed bool
			if event.Post != nil {
				if err := trx.Scalar(&removed, "SELECT status = 6 FROM posts WHERE id = $1", event.Post.ID); err != nil {
					return err
				}
			} else {
				if err := trx.Scalar(&removed, "SELECT deleted_at IS NOT NULL FROM comments WHERE id = $1", event.Comment.CommentID); err != nil {
					return err
				}
			}

			if removed {
				return completeNotification(trx, delivery.ID, delivery.Channel, ids)
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
			if err := trx.Get(&author, `
                SELECT id, name, email, role, status FROM users WHERE id = $1 AND tenant_id = $2
            `, delivery.UserID, delivery.TenantID); err != nil {
				return err
			}
			ctx = context.WithValue(ctx, app.UserCtxKey, author.toModel(ctx))
			ctx = context.WithValue(ctx, app.LocaleCtxKey, delivery.Locale)

			if delivery.Channel == "" {
				recipients, err := c.Prepare(ctx, &event)
				if err != nil {
					return err
				}

				channels := make([]string, len(recipients))
				ids := make([]int64, len(recipients))
				for index, recipient := range recipients {
					channels[index] = recipient.Channel
					ids[index] = int64(recipient.ID)
				}

				if _, err := trx.Execute(`
                    INSERT INTO notification_recipients (delivery_id, tenant_id, channel, recipient_id)
                    SELECT $1, $4, * FROM UNNEST($2::text[], $3::bigint[]) ON CONFLICT DO NOTHING
                `, delivery.ID, pq.Array(channels), pq.Array(ids), delivery.TenantID); err != nil {
					return err
				}

				if _, err := trx.Execute("UPDATE notification_deliveries SET prepared = TRUE, last_error = NULL WHERE id = $1", delivery.ID); err != nil {
					return err
				}
			} else if err := c.Send(ctx, &event, recipients); err != nil {
				return err
			}

			return completeNotification(trx, delivery.ID, delivery.Channel, ids)
		}()

		if deliveryErr == nil {
			return nil
		}

		if _, err := trx.Execute("ROLLBACK TO SAVEPOINT notification_delivery"); err != nil {
			return err
		}

		if delivery.Channel == "" {
			_, err = trx.Execute(`
                UPDATE notification_deliveries SET attempts = attempts + 1, last_error = $2,
                    available_at = NOW() + LEAST(3600, POWER(2, LEAST(attempts + 1, 12))) * INTERVAL '1 second'
                WHERE id = $1
            `, delivery.ID, deliveryErr.Error())
		} else {
			_, err = trx.Execute(`
                UPDATE notification_recipients SET attempts = attempts + 1, last_error = $4,
                    send_individually = send_individually OR $5,
                    available_at = NOW() + LEAST(3600, POWER(2, LEAST(attempts + 1, 12))) * INTERVAL '1 second'
                WHERE delivery_id = $1 AND channel = $2 AND recipient_id = ANY($3)
            `, delivery.ID, delivery.Channel, pq.Array(ids), deliveryErr.Error(), email.IsRecipientRejected(deliveryErr))
		}

		return err
	})

	if err != nil {
		return err
	}

	return deliveryErr
}

func completeNotification(trx *dbx.Trx, deliveryID int64, channel string, recipientIDs []int64) error {
	if _, err := trx.Execute("SELECT id FROM notification_deliveries WHERE id = $1 FOR UPDATE", deliveryID); err != nil {
		return err
	}

	if channel != "" {
		if _, err := trx.Execute(`
            DELETE FROM notification_recipients
            WHERE delivery_id = $1 AND channel = $2 AND recipient_id = ANY($3)
        `, deliveryID, channel, pq.Array(recipientIDs)); err != nil {
			return err
		}
	}

	_, err := trx.Execute(`
        DELETE FROM notification_deliveries delivery WHERE id = $1
        AND NOT EXISTS (SELECT 1 FROM notification_recipients recipient WHERE recipient.delivery_id = delivery.id)
    `, deliveryID)
	return err
}
