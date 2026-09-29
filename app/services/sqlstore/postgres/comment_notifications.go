package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/lib/pq"
)

func getCommentNotificationUsers(ctx context.Context, q *query.GetCommentNotificationUsers) error {
	q.Result = make([]*entity.User, 0)
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		event := q.Notification
		var postID, pageID *int
		owner := &query.GetDiscussion{CommentID: event.CommentID}
		if err := loadDiscussion(ctx, owner); err != nil {
			if errors.Cause(err) == app.ErrNotFound {
				return nil
			}
			return err
		}
		discussion := owner.Result
		if event.Owner.Kind == "post" {
			postID = &event.Owner.ID
		} else {
			pageID = &event.Owner.ID
		}

		comment, err := readCommentVisibility(trx, tenant.ID, event.CommentID)
		if err != nil {
			return err
		}

		var candidates []*dbUser
		err = trx.Select(&candidates, `
            SELECT u.id, u.name, u.email, u.role, u.status
            FROM users u
            LEFT JOIN post_subscribers post_subscription
              ON post_subscription.tenant_id = u.tenant_id AND post_subscription.user_id = u.id
             AND post_subscription.post_id = $3
            LEFT JOIN page_subscriptions page_subscription
              ON page_subscription.page_id = $4 AND page_subscription.user_id = u.id
            LEFT JOIN user_settings comment_setting
              ON comment_setting.tenant_id = u.tenant_id AND comment_setting.user_id = u.id
             AND comment_setting.key = $12
            LEFT JOIN user_settings mention_setting
              ON mention_setting.tenant_id = u.tenant_id AND mention_setting.user_id = u.id
             AND mention_setting.key = $13
            WHERE u.tenant_id = $1 AND u.status = $2 AND u.id <> $5
              AND ($6::integer <> $14 OR u.email_supressed_at IS NULL)
              AND (COALESCE(cardinality($7::integer[]), 0) = 0 OR u.id = ANY($7))
              AND (
                (u.id = ANY($8::integer[]) AND COALESCE(mention_setting.value::integer,
                    CASE WHEN u.role = ANY($18::integer[]) THEN $15::integer ELSE 0 END) & $6 > 0)
                OR (NOT $9 AND COALESCE(comment_setting.value::integer,
                    CASE WHEN u.role = ANY($11::integer[]) THEN $16::integer ELSE 0 END) & $6 > 0 AND (
                    u.id = $10
                    OR ($3::integer IS NOT NULL AND (
                        post_subscription.status = $17
                        OR (post_subscription.status IS NULL AND NOT u.role = ANY($19::integer[]))
                    ))
                    OR ($4::integer IS NOT NULL AND (
                        page_subscription.user_id IS NOT NULL OR u.role = ANY($11::integer[])
                    ))
                ))
              )
            ORDER BY u.id
        `, tenant.ID, enum.UserActive, postID, pageID, user.ID, q.Channel, pq.Array(q.UserIDs),
			pq.Array(event.MentionIDs), event.Edited, event.ParentAuthorID,
			pq.Array(enum.NotificationEventNewComment.DefaultEnabledUserRoles),
			enum.NotificationEventNewComment.UserSettingsKeyName, enum.NotificationEventMention.UserSettingsKeyName,
			enum.NotificationChannelEmail, enum.NotificationEventMention.DefaultSettingValue,
			enum.NotificationEventNewComment.DefaultSettingValue, enum.SubscriberActive,
			pq.Array(enum.NotificationEventMention.DefaultEnabledUserRoles),
			pq.Array(enum.NotificationEventNewComment.RequiresSubscriptionUserRoles))
		if err != nil {
			return err
		}

		for _, candidate := range candidates {
			recipient := candidate.toModel(ctx)
			if comment.ContentState(recipient, discussion, tenant) == "visible" {
				q.Result = append(q.Result, recipient)
			}
		}

		return nil
	})
}

func readCommentVisibility(trx *dbx.Trx, tenantID, commentID int) (*entity.Comment, error) {
	var facts struct {
		Deleted    bool      `db:"deleted"`
		Pending    bool      `db:"moderation_pending"`
		AuthorID   int       `db:"author_id"`
		AuthorRole enum.Role `db:"author_role"`
	}
	if err := trx.Get(&facts, `
        SELECT comment.deleted_at IS NOT NULL AS deleted, comment.moderation_pending,
               author.id AS author_id, author.role AS author_role
        FROM comments comment
        JOIN users author ON author.id = comment.user_id AND author.tenant_id = comment.tenant_id
        WHERE comment.tenant_id = $1 AND comment.id = $2
    `, tenantID, commentID); err != nil {
		return nil, err
	}

	return &entity.Comment{
		Deleted:           facts.Deleted,
		ModerationPending: facts.Pending,
		User: &entity.User{
			ID:   facts.AuthorID,
			Role: facts.AuthorRole,
		},
	}, nil
}
