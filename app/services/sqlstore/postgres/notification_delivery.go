package postgres

import (
	"context"
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

type notificationIntent struct {
	MentionIDs []int64
	Edited     bool
}

func scheduleNotification(ctx context.Context, c *cmd.ScheduleNotification) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
            INSERT INTO notification_deliveries (post_id, comment_id, tenant_id, user_id, mention_ids, edited, base_url, locale)
            VALUES (NULLIF($1, 0), NULLIF($2, 0), $3, $4, COALESCE($5::integer[], '{}'), $6, $7, $8)
        `, c.PostID, c.CommentID, tenant.ID, user.ID, pq.Array(c.MentionIDs), c.Edited, c.BaseURL, tenant.Locale)
		return err
	})
}

func processNotification(ctx context.Context, c *cmd.ProcessNotification) error {
	c.Found = false
	var deliveryErr error

	err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		var delivery struct {
			ID               int64   `db:"id"`
			PostID           int     `db:"post_id"`
			CommentID        int     `db:"comment_id"`
			TenantID         int     `db:"tenant_id"`
			UserID           int     `db:"user_id"`
			MentionIDs       []int64 `db:"mention_ids"`
			Edited           bool    `db:"edited"`
			BaseURL          string  `db:"base_url"`
			Locale           string  `db:"locale"`
			Channel          string  `db:"channel"`
			RecipientID      int     `db:"recipient_id"`
			SendIndividually bool    `db:"send_individually"`
		}

		err := trx.Get(&delivery, `
            SELECT id, COALESCE(post_id, 0) AS post_id, COALESCE(comment_id, 0) AS comment_id,
                   tenant_id, user_id, mention_ids, edited, base_url, locale,
                   '' AS channel, 0 AS recipient_id, FALSE AS send_individually
            FROM notification_deliveries
            WHERE NOT prepared AND available_at <= NOW()
            ORDER BY available_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
        `)
		if errors.Cause(err) == app.ErrNotFound {
			err = trx.Get(&delivery, `
                SELECT delivery.id, COALESCE(delivery.post_id, 0) AS post_id,
                       COALESCE(delivery.comment_id, 0) AS comment_id,
                       delivery.tenant_id, delivery.user_id, delivery.mention_ids, delivery.edited,
                       delivery.base_url, delivery.locale, recipient.channel,
                       recipient.recipient_id, recipient.send_individually
                FROM notification_recipients recipient
                JOIN notification_deliveries delivery ON delivery.id = recipient.delivery_id
                WHERE recipient.available_at <= NOW() AND delivery.available_at <= NOW()
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
            `, delivery.ID, delivery.RecipientID, c.EmailBatchSize-1)
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

			intent := notificationIntent{MentionIDs: delivery.MentionIDs, Edited: delivery.Edited}
			event, state, err := currentNotification(trx, delivery.TenantID, delivery.PostID, delivery.CommentID, intent)
			if err != nil {
				return err
			}

			switch state {
			case notificationRemoved:
				return completeNotification(trx, delivery.ID, delivery.Channel, ids)
			case notificationHidden:
				_, err := trx.Execute(`
                    UPDATE notification_deliveries SET available_at = NOW() + INTERVAL '1 minute'
                    WHERE id = $1
                `, delivery.ID)
				return err
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

type notificationState int

const (
	notificationReady notificationState = iota
	notificationHidden
	notificationRemoved
)

func currentNotification(trx *dbx.Trx, tenantID, postID, commentID int, intent notificationIntent) (entity.NotificationDelivery, notificationState, error) {
	event := entity.NotificationDelivery{}
	if postID != 0 {
		var post dbPost
		if err := trx.Get(&post, `
            SELECT id, number, title, slug, description, created_at, status, moderation_pending
            FROM posts WHERE tenant_id = $1 AND id = $2
        `, tenantID, postID); err != nil {
			return event, notificationRemoved, err
		}

		if enum.PostStatus(post.Status) == enum.PostDeleted {
			return event, notificationRemoved, nil
		}
		if post.ModerationPending {
			return event, notificationHidden, nil
		}

		event.Post = &entity.Post{
			ID:          post.ID,
			Number:      post.Number,
			Title:       post.Title,
			Slug:        post.Slug,
			Description: post.Description.String,
			CreatedAt:   post.CreatedAt,
			Status:      enum.PostStatus(post.Status),
		}
		return event, notificationReady, nil
	}

	var comment struct {
		Content        string `db:"content"`
		PostID         int    `db:"post_id"`
		PageID         int    `db:"page_id"`
		Number         int    `db:"number"`
		Title          string `db:"title"`
		Slug           string `db:"slug"`
		ParentAuthorID int    `db:"parent_author_id"`
		Hidden         bool   `db:"hidden"`
		Removed        bool   `db:"removed"`
	}
	if err := trx.Get(&comment, `
        SELECT comment.content, COALESCE(comment.post_id, 0) AS post_id,
               COALESCE(comment.page_id, 0) AS page_id, COALESCE(post.number, 0) AS number,
               COALESCE(post.title, page.title) AS title, COALESCE(post.slug, page.slug) AS slug,
               COALESCE(parent.user_id, 0) AS parent_author_id,
               comment.moderation_pending OR COALESCE(post.moderation_pending, FALSE) AS hidden,
               comment.deleted_at IS NOT NULL OR COALESCE(post.status = $3, FALSE) AS removed
        FROM comments comment
        LEFT JOIN comments parent ON parent.id = comment.parent_id AND parent.tenant_id = comment.tenant_id
        LEFT JOIN posts post ON post.id = comment.post_id AND post.tenant_id = comment.tenant_id
        LEFT JOIN pages page ON page.id = comment.page_id AND page.tenant_id = comment.tenant_id
        WHERE comment.tenant_id = $1 AND comment.id = $2
    `, tenantID, commentID, enum.PostDeleted); err != nil {
		return event, notificationRemoved, err
	}

	if comment.Removed {
		return event, notificationRemoved, nil
	}
	if comment.Hidden {
		return event, notificationHidden, nil
	}

	owner := entity.PageDiscussion(&entity.Page{
		ID:    comment.PageID,
		Title: comment.Title,
		Slug:  comment.Slug,
	}).Owner
	if comment.PostID != 0 {
		owner = entity.PostDiscussion(&entity.Post{
			ID:     comment.PostID,
			Number: comment.Number,
			Title:  comment.Title,
			Slug:   comment.Slug,
		}).Owner
	}

	currentMentions := make(map[int]bool)
	for _, mention := range entity.CommentString(comment.Content).ParseMentions() {
		currentMentions[mention.ID] = true
	}

	mentions := make([]int, 0, len(intent.MentionIDs))
	for _, id := range intent.MentionIDs {
		if currentMentions[int(id)] {
			mentions = append(mentions, int(id))
		}
	}
	event.Comment = &entity.CommentNotification{
		CommentID:      commentID,
		Owner:          owner,
		Content:        comment.Content,
		MentionIDs:     mentions,
		ParentAuthorID: comment.ParentAuthorID,
		Edited:         intent.Edited,
	}
	return event, notificationReady, nil
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
