package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func applyPostVote(ctx context.Context, c *cmd.ApplyPostVote) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			return validate.Unauthorized()
		}

		ctx, err := lockedContentContext(ctx, trx, tenant, user)
		if err != nil {
			return err
		}
		tenant = ctx.Value(app.TenantCtxKey).(*entity.Tenant)
		user = ctx.Value(app.UserCtxKey).(*entity.User)

		canViewHidden := entity.Can(user, tenant, entity.ModeratePosts)
		var eligibility struct {
			ID         int             `db:"id"`
			Status     enum.PostStatus `db:"status"`
			Locked     bool            `db:"locked"`
			ArchivedAt sql.NullTime    `db:"archived_at"`
		}
		if err := trx.Get(&eligibility, `
			SELECT post.id, post.status, post.archived_at,
				COALESCE((post.locked_settings->>'locked')::boolean, false) AS locked
			FROM posts post
			WHERE post.number = $1 AND post.tenant_id = $2
			FOR NO KEY UPDATE OF post
		`, c.Number, tenant.ID); err != nil {
			return err
		}

		post := entity.Post{ID: eligibility.ID, Number: c.Number, Status: eligibility.Status}
		if eligibility.Locked {
			post.LockedSettings = &entity.PostLockedSettings{Locked: true}
		}

		read := func() error {
			return trx.Get(&c.State, `SELECT COALESCE(v.vote_type, 0) AS direction,
				COALESCE(r.revision, 0) AS revision, p.upvotes, p.downvotes, p.last_activity_at
				FROM visible_posts_for($3, $4::boolean, $2) p
				LEFT JOIN post_votes v ON v.post_id = p.id AND v.user_id = $2
				LEFT JOIN post_vote_revisions r ON r.post_id = p.id AND r.user_id = $2
				WHERE p.number = $1`, c.Number, user.ID, tenant.ID, canViewHidden)
		}

		if err := read(); err != nil {
			return err
		}

		if !post.AllowedActions(user, tenant, time.Now()).Vote {
			return validate.Unauthorized()
		}

		if c.State.Revision != c.Revision {
			return nil
		}

		if c.State.Direction == c.Direction {
			// A no-op must still invalidate older requests.
			_, err := trx.Execute(`INSERT INTO post_vote_revisions (post_id, user_id, tenant_id, revision)
				VALUES ($1, $2, $3, 1) ON CONFLICT (post_id, user_id)
				DO UPDATE SET revision = post_vote_revisions.revision + 1`,
				post.ID, user.ID, tenant.ID)
			if err != nil {
				return err
			}
		} else {
			var change bus.Msg = &cmd.RemoveVote{Post: &post, User: user}
			if c.Direction != 0 {
				change = &cmd.AddVote{Post: &post, User: user, VoteType: enum.VoteType(c.Direction)}
			}
			if err := bus.Dispatch(ctx, change); err != nil {
				return err
			}
		}

		if c.Direction == 1 && post.Status == enum.PostArchived && eligibility.ArchivedAt.Valid {
			votes := &query.CountVotesSinceArchive{PostID: post.ID, ArchivedAt: eligibility.ArchivedAt.Time}
			if err := bus.Dispatch(ctx, votes); err != nil {
				return err
			}
			if votes.Result > 10 {
				if err := bus.Dispatch(ctx, &cmd.UnarchivePost{Post: &post, Reason: "Vote threshold exceeded"}); err != nil {
					return err
				}
				c.Unarchived = true
			}
		}

		if err := read(); err != nil {
			return err
		}

		c.State.Applied = true
		return nil
	})
}
