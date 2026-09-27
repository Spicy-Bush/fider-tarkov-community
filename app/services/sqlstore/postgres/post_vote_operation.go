package postgres

import (
	"context"
	"database/sql"

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
		if user == nil || user.Status == enum.UserBlocked {
			return validate.Unauthorized()
		}

		isStaff := user.IsCollaborator() || user.IsModerator()
		var eligibility struct {
			ID         int             `db:"id"`
			Status     enum.PostStatus `db:"status"`
			Locked     bool            `db:"locked"`
			ArchivedAt sql.NullTime    `db:"archived_at"`
		}
		if err := trx.Get(&eligibility, `SELECT id, status, archived_at,
			COALESCE((locked_settings->>'locked')::boolean, false) AS locked
			FROM visible_posts WHERE number = $1 AND tenant_id = $2
			AND (moderation_pending = FALSE OR user_id = $3 OR $4)
			FOR NO KEY UPDATE`, c.Number, tenant.ID, user.ID, isStaff); err != nil {
			return err
		}

		post := entity.Post{ID: eligibility.ID, Number: c.Number, Status: eligibility.Status}
		if eligibility.Locked && !user.IsCollaborator() {
			return validate.Unauthorized()
		}

		if !post.CanBeVoted() || user.IsMuted() {
			return validate.Unauthorized()
		}

		read := func() error {
			return trx.Get(&c.State, `SELECT COALESCE(v.vote_type, 0) AS direction,
				COALESCE(r.revision, 0) AS revision, p.upvotes, p.downvotes
				FROM visible_posts p
				LEFT JOIN post_votes v ON v.post_id = p.id AND v.user_id = $2
				LEFT JOIN post_vote_revisions r ON r.post_id = p.id AND r.user_id = $2
				WHERE p.number = $1 AND p.tenant_id = $3
				AND (p.moderation_pending = FALSE OR p.user_id = $2 OR $4)`, c.Number, user.ID, tenant.ID, isStaff)
		}

		if err := read(); err != nil {
			return err
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
