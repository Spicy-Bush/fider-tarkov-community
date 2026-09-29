package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

const visibleCommentOwners = `
    WITH visible_pages AS (
        SELECT id FROM pages p
        WHERE p.tenant_id = $1 AND page_is_visible(p.status, p.visibility, p.allowed_roles, $2, $4)
    ), visible_comment_owners AS (
        SELECT c.id FROM comments c
        LEFT JOIN visible_posts_for($1, $5::boolean, $3) post ON post.id = c.post_id
        WHERE c.tenant_id = $1 AND (
            post.id IS NOT NULL
            OR c.page_id IN (SELECT id FROM visible_pages)
        )
    )
`

const visibleComments = visibleCommentOwners + `,
    visible_comments AS (
        SELECT c.* FROM comments c
        LEFT JOIN users author ON author.id = c.user_id AND author.tenant_id = c.tenant_id
        WHERE c.id IN (SELECT id FROM visible_comment_owners) AND c.deleted_at IS NULL
          AND (
            NOT c.moderation_pending OR c.user_id = $3
            OR COALESCE(author.role, 0) = ANY($6::integer[])
          )
    )
`

func commentOwnerParams(tenant *entity.Tenant, user *entity.User) []any {
	return []any{
		tenant.ID, viewerRole(user), viewerID(user),
		entity.Can(user, tenant, entity.ManagePages),
		entity.Can(user, tenant, entity.ModeratePosts),
	}
}

func commentVisibilityParams(tenant *entity.Tenant, user *entity.User) []any {
	return append(commentOwnerParams(tenant, user), pq.Array(entity.ModeratedContentRoles(user, tenant)))
}

func viewerRole(user *entity.User) string {
	if user == nil {
		return ""
	}

	return user.Role.String()
}

func viewerID(user *entity.User) int {
	if user == nil {
		return 0
	}

	return user.ID
}

func getDiscussion(ctx context.Context, q *query.GetDiscussion) error {
	if err := loadDiscussion(ctx, q); err != nil {
		return err
	}
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if !q.Result.CanView(user, tenant) {
		q.Result = nil
		return app.ErrNotFound
	}
	return nil
}

func loadDiscussion(ctx context.Context, q *query.GetDiscussion) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = nil
		postID := 0
		pageID := q.PageID

		if q.CommentID != 0 {
			var owner struct {
				PostID int `db:"post_id"`
				PageID int `db:"page_id"`
			}

			if err := trx.Get(&owner, `
                SELECT COALESCE(post_id, 0) AS post_id, COALESCE(page_id, 0) AS page_id
                FROM comments WHERE tenant_id = $1 AND id = $2
            `, tenant.ID, q.CommentID); err != nil {
				return err
			}

			postID = owner.PostID
			pageID = owner.PageID
		}

		var discussion *entity.Discussion
		if pageID != 0 {
			var page struct {
				ID             int                   `db:"id"`
				Title          string                `db:"title"`
				Slug           string                `db:"slug"`
				Status         entity.PageStatus     `db:"status"`
				Visibility     entity.PageVisibility `db:"visibility"`
				AllowedRoles   dbx.NullString        `db:"allowed_roles"`
				AllowComments  bool                  `db:"allow_comments"`
				AllowImages    bool                  `db:"allow_comment_images"`
				AllowReactions bool                  `db:"allow_reactions"`
			}
			selection := `SELECT id, title, slug, status, visibility, allowed_roles,
                allow_comments, allow_comment_images, allow_reactions
                FROM pages WHERE tenant_id = $1 AND id = $2`
			if q.LockOwner {
				selection += " FOR SHARE"
			}
			if err := trx.Get(&page, selection, tenant.ID, pageID); err != nil {
				return err
			}
			model := &entity.Page{
				ID:                 page.ID,
				Title:              page.Title,
				Slug:               page.Slug,
				Status:             page.Status,
				Visibility:         page.Visibility,
				AllowComments:      page.AllowComments,
				AllowCommentImages: page.AllowImages,
				AllowReactions:     page.AllowReactions,
			}
			if page.AllowedRoles.Valid {
				if err := json.Unmarshal([]byte(page.AllowedRoles.String), &model.AllowedRoles); err != nil {
					return err
				}
			}
			discussion = entity.PageDiscussion(model)
		} else {
			var post struct {
				ID       int             `db:"id"`
				Number   int             `db:"number"`
				Title    string          `db:"title"`
				Slug     string          `db:"slug"`
				Status   enum.PostStatus `db:"status"`
				Locked   bool            `db:"locked"`
				AuthorID int             `db:"user_id"`
				Hidden   bool            `db:"moderation_pending"`
			}
			selection := `SELECT post.id, post.number, post.title, post.slug, post.status,
                post.user_id, post.moderation_pending,
                COALESCE((locked_settings->>'locked')::boolean, FALSE) AS locked
                FROM posts post WHERE post.tenant_id = $1 AND `
			id := q.PostNumber
			if postID != 0 {
				selection += "post.id = $2"
				id = postID
			} else {
				selection += "post.number = $2"
			}
			if q.LockOwner {
				selection += " FOR NO KEY UPDATE"
			}
			if err := trx.Get(&post, selection, tenant.ID, id); err != nil {
				return err
			}
			discussion = entity.PostDiscussion(&entity.Post{
				ID:                post.ID,
				Number:            post.Number,
				Title:             post.Title,
				Slug:              post.Slug,
				Status:            post.Status,
				User:              &entity.User{ID: post.AuthorID},
				ModerationPending: post.Hidden,
				LockedSettings:    &entity.PostLockedSettings{Locked: post.Locked},
			})
		}

		q.Result = discussion
		return nil
	})
}

const commentDetails = `
    SELECT c.id, COALESCE(c.post_id, 0) AS post_id, COALESCE(c.page_id, 0) AS page_id,
           -selected.position AS sort_score,
           c.parent_id, c.content, c.created_at, c.edited_at,
           c.deleted_at IS NOT NULL AS deleted, c.moderation_pending, c.moderation_data,
           EXISTS (SELECT 1 FROM comments child WHERE child.parent_id = c.id) AS has_replies,
           u.id AS user_id, u.name AS user_name, u.email AS user_email,
           u.role AS user_role, u.visual_role AS user_visual_role, u.status AS user_status,
           u.avatar_type AS user_avatar_type, u.avatar_bkey AS user_avatar_bkey,
           e.id AS edited_by_id, e.name AS edited_by_name, e.email AS edited_by_email,
           e.role AS edited_by_role, e.visual_role AS edited_by_visual_role,
           e.status AS edited_by_status, e.avatar_type AS edited_by_avatar_type,
           e.avatar_bkey AS edited_by_avatar_bkey,
           ARRAY(SELECT attachment_bkey FROM attachments a
                 WHERE a.comment_id = c.id AND a.tenant_id = c.tenant_id ORDER BY a.id) AS attachment_bkeys,
           (SELECT json_agg(reaction ORDER BY reaction.emoji) FROM (
                SELECT emoji, COUNT(*) AS count, BOOL_OR(user_id = $2) AS "includesMe"
                FROM reactions WHERE comment_id = c.id GROUP BY emoji
           ) reaction)::text AS reaction_counts
    FROM selected
    JOIN comments c ON c.id = selected.id AND c.tenant_id = $1
    JOIN users u ON u.id = c.user_id AND u.tenant_id = c.tenant_id
    LEFT JOIN users e ON e.id = c.edited_by_id AND e.tenant_id = c.tenant_id
    ORDER BY selected.position, c.created_at DESC, c.id DESC
`

func readComments(ctx context.Context, trx *dbx.Trx, selection string, args ...any) ([]*entity.Comment, error) {
	var records []*dbComment
	if err := trx.Select(&records, "WITH RECURSIVE selected AS ("+selection+") "+commentDetails, args...); err != nil {
		return nil, err
	}

	comments := make([]*entity.Comment, 0, len(records))
	for _, record := range records {
		comments = append(comments, record.toModel(ctx))
	}

	return comments, nil
}

func getDiscussionComments(ctx context.Context, q *query.GetDiscussionComments) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if !q.Discussion.CanView(user, tenant) {
			return app.ErrNotFound
		}

		viewerID := 0
		if user != nil {
			viewerID = user.ID
		}

		ownerColumn := "post_id"
		if q.Discussion.Owner.Kind == "page" {
			ownerColumn = "page_id"
		}

		if len(q.IDs) > 0 {
			var err error
			q.Result, err = readComments(ctx, trx, `
                SELECT id, 0 AS position FROM comments
                WHERE tenant_id = $1 AND `+pq.QuoteIdentifier(ownerColumn)+` = $3
                  AND id = ANY($4::integer[])
            `, tenant.ID, viewerID, q.Discussion.Owner.ID, pq.Array(q.IDs))
			return err
		}

		if q.ParentID != nil {
			var exists bool
			if err := trx.Scalar(&exists, `SELECT EXISTS (
                SELECT 1 FROM comments WHERE tenant_id = $1 AND id = $2
                AND `+pq.QuoteIdentifier(ownerColumn)+` = $3
            )`, tenant.ID, *q.ParentID, q.Discussion.Owner.ID); err != nil {
				return err
			}
			if !exists {
				return app.ErrNotFound
			}
		}

		candidates := `
            SELECT c.id, c.created_at, c.deleted_at FROM comments c
            WHERE c.tenant_id = $1 AND c.` + pq.QuoteIdentifier(ownerColumn) + ` = $3
              AND (c.parent_id = $4 OR ($4::integer IS NULL AND c.parent_id IS NULL))
              AND (c.deleted_at IS NULL OR EXISTS (SELECT 1 FROM comments child WHERE child.parent_id = c.id))
        `
		if q.Sort == "replies" {
			var err error
			q.Result, err = readCommentsByReplyCount(ctx, trx, q, candidates, ownerColumn, tenant.ID, viewerID)
			return err
		}

		selection := "SELECT id, 0 AS position FROM (" + candidates + `) candidate
            WHERE ($6::integer = 0 OR (created_at, id) < ($5, $6))
            ORDER BY created_at DESC, id DESC LIMIT 26
        `
		parameters := []any{tenant.ID, viewerID, q.Discussion.Owner.ID, q.ParentID, q.After, q.AfterID}

		if q.Sort == "liked" || q.Sort == "" || q.Sort == "disliked" {
			selection = "WITH candidates AS MATERIALIZED (" + candidates + `), reaction_counts AS (
            SELECT comment_id, COUNT(*) AS score
            FROM reactions
            WHERE comment_id = ANY(ARRAY(SELECT id FROM candidates)) AND emoji = $8
            GROUP BY comment_id
        ), ranked AS (
            SELECT candidate.id, candidate.created_at, COALESCE(reaction_counts.score, 0) AS score
            FROM candidates candidate
            LEFT JOIN reaction_counts ON reaction_counts.comment_id = candidate.id
        )
            SELECT id, -score AS position FROM ranked
            WHERE ($6::integer = 0 OR (score, created_at, id) < ($7, $5, $6))
            ORDER BY score DESC, created_at DESC, id DESC LIMIT 26
        `
			emoji := "👍"
			if q.Sort == "disliked" {
				emoji = "👎"
			}

			parameters = append(parameters, q.AfterScore, emoji)
		}

		var err error
		q.Result, err = readComments(ctx, trx, selection, parameters...)
		return err
	})
}

func readCommentsByReplyCount(ctx context.Context, trx *dbx.Trx, q *query.GetDiscussionComments, candidates, ownerColumn string, tenantID, viewerID int) ([]*entity.Comment, error) {
	if q.AfterID == 0 {
		first, err := readComments(ctx, trx, "SELECT id, 0 AS position FROM ("+candidates+") candidates LIMIT 2", tenantID, viewerID, q.Discussion.Owner.ID, q.ParentID)
		if err != nil {
			return nil, err
		}
		// A single sibling needs neither a ranking comparison nor a continuation cursor.
		if len(first) < 2 {
			return first, nil
		}
	}

	selection, err := prepareDiscussionReplies(ctx, trx, q, ownerColumn, tenantID)
	if err != nil {
		return nil, err
	}
	if !selection.Uncached {
		return readComments(ctx, trx, `
            SELECT id, -score AS position FROM UNNEST($3::integer[], $4::bigint[]) ranked(id, score)
        `, tenantID, viewerID, pq.Array(selection.IDs), pq.Array(selection.Scores))
	}

	rows, err := trx.Query(`
        SELECT id, COALESCE(parent_id, 0), deleted_at IS NOT NULL,
               CASE WHEN parent_id IS NOT DISTINCT FROM $3::integer THEN created_at END
        FROM comments WHERE tenant_id = $1 AND `+pq.QuoteIdentifier(ownerColumn)+` = $2
          AND id > COALESCE($3::integer, 0)
        ORDER BY id DESC
    `, tenantID, q.Discussion.Owner.ID, q.ParentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rankedComment struct {
		ID        int
		Score     int64
		CreatedAt time.Time
	}
	var ranked []rankedComment
	descendants := make(map[int]int64)
	var rawID, rawParentID int64
	var deleted bool
	var createdAt sql.NullTime
	for rows.Next() {
		if err := rows.Scan(&rawID, &rawParentID, &deleted, &createdAt); err != nil {
			return nil, err
		}
		id, parentID := int(rawID), int(rawParentID)

		// Parents precede replies by ID, so each subtree is complete before its parent.
		count, hasReplies := descendants[id]
		delete(descendants, id)
		if createdAt.Valid && (!deleted || hasReplies) {
			ranked = append(ranked, rankedComment{ID: id, Score: count, CreatedAt: createdAt.Time})
		}
		if parentID != 0 {
			if !deleted {
				count++
			}
			descendants[parentID] += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].Score != ranked[right].Score {
			return ranked[left].Score > ranked[right].Score
		}
		if !ranked[left].CreatedAt.Equal(ranked[right].CreatedAt) {
			return ranked[left].CreatedAt.After(ranked[right].CreatedAt)
		}
		return ranked[left].ID > ranked[right].ID
	})

	ids := make([]int, 0, 26)
	scores := make([]int64, 0, 26)
	for _, comment := range ranked {
		if q.AfterID != 0 {
			if comment.Score > int64(q.AfterScore) || (comment.Score == int64(q.AfterScore) &&
				(comment.CreatedAt.After(q.After) || (comment.CreatedAt.Equal(q.After) && comment.ID >= q.AfterID))) {
				continue
			}
		}
		ids = append(ids, comment.ID)
		scores = append(scores, comment.Score)
		if len(ids) == 26 {
			break
		}
	}
	return readComments(ctx, trx, `
        SELECT id, -score AS position FROM UNNEST($3::integer[], $4::bigint[]) ranked(id, score)
    `, tenantID, viewerID, pq.Array(ids), pq.Array(scores))
}

func getCommentAncestors(ctx context.Context, q *query.GetCommentAncestors) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if !q.Discussion.CanView(user, tenant) {
			return app.ErrNotFound
		}

		viewerID := 0
		if user != nil {
			viewerID = user.ID
		}
		ownerColumn := "post_id"
		if q.Discussion.Owner.Kind == "page" {
			ownerColumn = "page_id"
		}

		var err error
		q.Result, err = readComments(ctx, trx, `
            SELECT id, parent_id, 0 AS position FROM comments WHERE tenant_id = $1 AND id = $3
              AND `+pq.QuoteIdentifier(ownerColumn)+` = $4
            UNION ALL
            SELECT parent.id, parent.parent_id, child.position - 1
            FROM comments parent JOIN selected child ON parent.id = child.parent_id
            WHERE parent.tenant_id = $1 AND child.position > -2
        `, tenant.ID, viewerID, q.CommentID, q.Discussion.Owner.ID)
		return err
	})
}

func getDiscussionChainReplies(ctx context.Context, q *query.GetDiscussionChainReplies) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		viewerID := 0
		if user != nil {
			viewerID = user.ID
		}

		// Unbranched replies travel together to avoid a network round trip for each level.
		var err error
		q.Result, err = readComments(ctx, trx, `
            WITH RECURSIVE chain AS (
                SELECT unnest($3::integer[]) AS id, 0 AS depth
                UNION ALL
                SELECT child.id, parent.depth + 1
                FROM chain parent
                JOIN LATERAL (
                    SELECT MIN(id) AS id FROM (
                        SELECT id FROM comments c
                        WHERE c.tenant_id = $1 AND c.parent_id = parent.id
                          AND (c.deleted_at IS NULL OR EXISTS (
                              SELECT 1 FROM comments reply WHERE reply.parent_id = c.id
                          ))
                        LIMIT 2
                    ) children
                    HAVING COUNT(*) = 1
                ) child ON TRUE
                WHERE parent.depth < $4
            )
            SELECT id, depth AS position FROM chain
            WHERE depth > 0 ORDER BY depth, id LIMIT 100
        `, tenant.ID, viewerID, pq.Array(q.ParentIDs), q.Depth)
		return err
	})
}
