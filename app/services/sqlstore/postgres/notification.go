package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/lib/pq"
)

const visibleNotifications = visibleCommentOwners + `,
    visible_notifications AS (
        SELECT n.* FROM notifications n
        LEFT JOIN comments comment ON comment.id = n.comment_id AND comment.tenant_id = n.tenant_id
        LEFT JOIN users author ON author.id = comment.user_id AND author.tenant_id = n.tenant_id
        LEFT JOIN visible_posts_for($1, $2, $3) post ON post.id = n.post_id
        WHERE n.tenant_id = $1 AND n.user_id = $3
          AND (n.page_id IS NULL OR n.page_id IN (SELECT id FROM visible_pages))
          AND (n.post_id IS NULL OR post.id IS NOT NULL)
          AND (n.comment_id IS NULL OR (
            comment.id IN (SELECT id FROM visible_comment_owners) AND comment.deleted_at IS NULL
            AND (
                NOT comment.moderation_pending OR comment.user_id = $3
                OR $2 IN ('administrator', 'collaborator')
                OR ($2 = 'moderator' AND author.role IN (1, 5))
            )
          ))
    )
`

func purgeExpiredNotifications(ctx context.Context, c *cmd.PurgeExpiredNotifications) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, _ *entity.Tenant, _ *entity.User) error {
		count, err := trx.Execute("DELETE FROM notifications WHERE CREATED_AT <= NOW() - INTERVAL '365 days'")
		if err != nil {
			return errors.Wrap(err, "failed to delete expired notifications")
		}
		c.NumOfDeletedNotifications = int(count)
		return nil
	})
}

func markAllNotificationsAsRead(ctx context.Context, c *cmd.MarkAllNotificationsAsRead) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			return nil
		}
		_, err := trx.Execute(`
			UPDATE notifications SET read = true, updated_at = $1
			WHERE tenant_id = $2 AND user_id = $3 AND read = false
		`, time.Now(), tenant.ID, user.ID)
		if err != nil {
			return errors.Wrap(err, "failed to mark all notifications as read")
		}
		return nil
	})
}

func countUnreadNotifications(ctx context.Context, q *query.CountUnreadNotifications) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = 0

		if user != nil {
			err := trx.Scalar(&q.Result, visibleNotifications + "SELECT COUNT(*) FROM visible_notifications WHERE read = false",
				tenant.ID, user.Role.String(), user.ID)
			if err != nil {
				return errors.Wrap(err, "failed count total unread notifications")
			}
		}
		return nil
	})
}

func markNotificationAsRead(ctx context.Context, c *cmd.MarkNotificationAsRead) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			return nil
		}

		_, err := trx.Execute(`
			UPDATE notifications SET read = true, updated_at = $1
			WHERE id = $2 AND tenant_id = $3 AND user_id = $4 AND read = false
		`, time.Now(), c.ID, tenant.ID, user.ID)
		if err != nil {
			return errors.Wrap(err, "failed to mark notification as read")
		}
		return nil
	})
}

func getNotificationByID(ctx context.Context, q *query.GetNotificationByID) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = nil
		notification := &entity.Notification{}

		err := trx.Get(notification, visibleNotifications + `
			SELECT id, title, link, read, created_at 
			FROM visible_notifications WHERE id = $4
		`, tenant.ID, user.Role.String(), user.ID, q.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get notifications with id '%d'", q.ID)
		}

		q.Result = notification
		return nil
	})
}

func getActiveNotifications(ctx context.Context, q *query.GetActiveNotifications) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if q.Page < 1 {
			q.Page = 1
		}
		if q.PerPage < 1 {
			q.PerPage = 10
		}

		if q.PerPage > 100 {
			q.PerPage = 100
		}
		offset := int64(q.Page-1) * int64(q.PerPage)
		condition := "TRUE"
		if q.Type == "unread" {
			condition = "NOT n.read"
		} else if q.Type == "read" {
			condition = "n.read"
		}

		var snapshot struct {
			Unread int    `db:"unread"`
			Read   int    `db:"read"`
			Items  []byte `db:"items"`
		}
		args := []any{tenant.ID, user.Role.String(), user.ID, q.PerPage, offset}
		err := trx.Get(&snapshot, visibleNotifications+fmt.Sprintf(`
			, active_notifications AS (
				SELECT * FROM visible_notifications
				WHERE NOT read OR updated_at > CURRENT_DATE - INTERVAL '30 days'
			), totals AS (
				SELECT COUNT(*) FILTER (WHERE NOT read) AS unread,
					COUNT(*) FILTER (WHERE read) AS read
				FROM active_notifications
			), selected AS (
				SELECT n.id, n.title, n.link, n.read, n.created_at AS "createdAt",
					n.author_id, u.avatar_type, u.avatar_bkey, u.name AS "authorName"
				FROM active_notifications n
				LEFT JOIN users u ON u.id = n.author_id
				WHERE %s
				ORDER BY n.updated_at DESC, n.id DESC
				LIMIT $4 OFFSET $5
			)
			SELECT totals.unread, totals.read,
				COALESCE((SELECT json_agg(selected) FROM selected), '[]') AS items
			FROM totals
		`, condition), args...)
		if err != nil {
			return errors.Wrap(err, "failed to get active notifications")
		}

		var rows []struct {
			entity.Notification
			AuthorID      int    `json:"author_id"`
			AvatarBlobKey string `json:"avatar_bkey"`
			AvatarType    int    `json:"avatar_type"`
		}
		if err := json.Unmarshal(snapshot.Items, &rows); err != nil {
			return errors.Wrap(err, "failed to decode active notifications")
		}
		q.Result = make([]*entity.Notification, 0, len(rows))
		for i := range rows {
			row := &rows[i]
			row.Notification.AuthorID = row.AuthorID
			row.Notification.AvatarType = enum.AvatarType(row.AvatarType)
			row.Notification.AvatarBlobKey = row.AvatarBlobKey
			row.AvatarURL = buildAvatarURL(ctx, row.Notification.AvatarType, row.AuthorID, row.AuthorName, row.AvatarBlobKey)
			q.Result = append(q.Result, &row.Notification)
		}
		q.UnreadCount = snapshot.Unread
		q.ReadCount = snapshot.Read
		q.TotalCount = snapshot.Unread + snapshot.Read
		if q.Type == "unread" {
			q.TotalCount = snapshot.Unread
		} else if q.Type == "read" {
			q.TotalCount = snapshot.Read
		}

		return nil
	})
}

func purgeReadNotifications(ctx context.Context, c *cmd.PurgeReadNotifications) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			return nil
		}

		query := `
			DELETE FROM notifications 
			WHERE tenant_id = $1 
			AND user_id = $2 
			AND read = true
		`

		count, err := trx.Execute(query, tenant.ID, user.ID)
		if err != nil {
			return errors.Wrap(err, "failed to purge read notifications")
		}

		c.NumOfPurgedNotifications = int(count)
		return nil
	})
}

func addNewNotification(ctx context.Context, c *cmd.AddNewNotification) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		c.Result = nil
		if user.ID == c.User.ID {
			return nil
		}

		now := time.Now()
		notification := &entity.Notification{
			Title:     c.Title,
			Link:      c.Link,
			CreatedAt: now,
			Read:      false,
		}

		var postID interface{} = c.PostID
		if c.PostID == 0 {
			postID = nil
		}

		err := trx.Get(&notification.ID, `
			INSERT INTO notifications (tenant_id, user_id, title, link, read, post_id, author_id, created_at, updated_at, comment_id, page_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, NULLIF($9, 0), NULLIF($10, 0))
			RETURNING id
		`, tenant.ID, c.User.ID, c.Title, c.Link, false, postID, user.ID, now, c.CommentID, c.PageID)
		if err != nil {
			return errors.Wrap(err, "failed to insert notification")
		}

		c.Result = notification
		return nil
	})
}

func addSubscriber(ctx context.Context, c *cmd.AddSubscriber) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		return internalAddSubscriber(trx, c.Post, tenant, c.User, true)
	})
}

func removeSubscriber(ctx context.Context, c *cmd.RemoveSubscriber) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			INSERT INTO post_subscribers (tenant_id, user_id, post_id, created_at, updated_at, status)
			VALUES ($1, $2, $3, $4, $4, $5) ON CONFLICT (user_id, post_id)
			DO UPDATE SET status = 0, updated_at = $4`,
			tenant.ID, c.User.ID, c.Post.ID, time.Now(), enum.SubscriberInactive,
		)
		if err != nil {
			return errors.Wrap(err, "failed remove post subscriber")
		}
		return nil
	})
}

func getActiveSubscribers(ctx context.Context, q *query.GetActiveSubscribers) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]*entity.User, 0)

		var (
			users []*dbUser
			err   error
		)

		// When searching for email subscrivers, skip users with email supressed
		supressionCondition := ""
		if q.Channel == enum.NotificationChannelEmail {
			supressionCondition = "AND u.email_supressed_at IS NULL"
		}

		// If the event doesn't require a subscription, notify everyone
		if len(q.Event.RequiresSubscriptionUserRoles) == 0 {
			err = trx.Select(&users, fmt.Sprintf(`
				SELECT DISTINCT u.id, u.name, u.email, u.tenant_id, u.role, u.status
				FROM users u
				LEFT JOIN user_settings set
				ON set.user_id = u.id
				AND set.tenant_id = u.tenant_id
				AND set.key = $1
				WHERE u.tenant_id = $2
				AND u.status = $5
				AND (COALESCE(cardinality($6::int[]), 0) = 0 OR u.id = ANY($6))
				%s
				AND (
					(set.value IS NULL AND u.role = ANY($3))
					OR CAST(set.value AS integer) & $4 > 0
				)
				ORDER by u.id`, supressionCondition),
				q.Event.UserSettingsKeyName,
				tenant.ID,
				pq.Array(q.Event.DefaultEnabledUserRoles),
				q.Channel,
				enum.UserActive,
				pq.Array(q.UserIDs),
			)
		} else {
			// If the event requires a subscription, notify only those who subscribed
			err = trx.Select(&users, fmt.Sprintf(`
				SELECT DISTINCT u.id, u.name, u.email, u.tenant_id, u.role, u.status
				FROM users u
				LEFT JOIN post_subscribers sub
				ON sub.user_id = u.id
				AND sub.post_id = (SELECT id FROM posts p WHERE p.tenant_id = $4 and p.number = $1 LIMIT 1)
				AND sub.tenant_id = u.tenant_id
				LEFT JOIN user_settings set
				ON set.user_id = u.id
				AND set.key = $3
				AND set.tenant_id = u.tenant_id
		WHERE u.tenant_id = $4
		AND u.status = $8
		AND (COALESCE(cardinality($9::int[]), 0) = 0 OR u.id = ANY($9))
		%s
		AND ( sub.status = $2 OR (sub.status IS NULL AND NOT u.role = ANY($7)) )
				AND (
					(set.value IS NULL AND u.role = ANY($5))
					OR CAST(set.value AS integer) & $6 > 0
				)
				ORDER by u.id`, supressionCondition),
				q.Number,
				enum.SubscriberActive,
				q.Event.UserSettingsKeyName,
				tenant.ID,
				pq.Array(q.Event.DefaultEnabledUserRoles),
				q.Channel,
				pq.Array(q.Event.RequiresSubscriptionUserRoles),
				enum.UserActive,
				pq.Array(q.UserIDs),
			)
		}

		if err != nil {
			return errors.Wrap(err, "failed to get post number '%d' subscribers", q.Number)
		}

		q.Result = make([]*entity.User, len(users))
		for i, user := range users {
			q.Result[i] = user.toModel(ctx)
		}
		return nil
	})
}

func internalAddSubscriber(trx *dbx.Trx, post *entity.Post, tenant *entity.Tenant, user *entity.User, force bool) error {
	conflict := " DO NOTHING"
	if force {
		conflict = "(user_id, post_id) DO UPDATE SET status = $5, updated_at = $4"
	}

	_, err := trx.Execute(fmt.Sprintf(`
	INSERT INTO post_subscribers (tenant_id, user_id, post_id, created_at, updated_at, status)
	VALUES ($1, $2, $3, $4, $4, $5)  ON CONFLICT %s`, conflict),
		tenant.ID, user.ID, post.ID, time.Now(), enum.SubscriberActive,
	)
	if err != nil {
		return errors.Wrap(err, "failed insert post subscriber")
	}
	return nil
}

func supressEmail(ctx context.Context, c *cmd.SupressEmail) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		cmd := "UPDATE users SET email_supressed_at = $1 WHERE email = ANY($2) AND email_supressed_at IS NULL"
		rowsCount, err := trx.Execute(cmd, time.Now(), pq.Array(c.EmailAddresses))
		if err != nil {
			return errors.Wrap(err, "failed to update supress email: %s", strings.Join(c.EmailAddresses, ","))
		}
		c.NumOfSupressedEmailAddresses = int(rowsCount)
		return nil
	})
}

// GetUsersToNotify retrieves users who should receive notifications
func getUsersToNotify(ctx context.Context, q *query.GetUsersToNotify) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var users []*dbUser
		err := trx.Select(&users, `
			SELECT DISTINCT u.id, u.name, u.email, u.tenant_id, u.role, u.status
			FROM users u
			LEFT JOIN user_settings set
			ON set.user_id = u.id
			AND set.tenant_id = u.tenant_id
			AND set.key = $1
			WHERE u.tenant_id = $2
			AND u.status = $5
			AND (
				(set.value IS NULL AND u.role = ANY($3))
				OR CAST(set.value AS integer) & $4 > 0
			)
			ORDER by u.id`,
			q.Event.UserSettingsKeyName,
			tenant.ID,
			pq.Array(q.Event.DefaultEnabledUserRoles),
			q.Channel,
			enum.UserActive,
		)
		if err != nil {
			return errors.Wrap(err, "failed to get users to notify")
		}

		q.Result = make([]*entity.User, len(users))
		for i, user := range users {
			q.Result[i] = user.toModel(ctx)
		}
		return nil
	})
}
