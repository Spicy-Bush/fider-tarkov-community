package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/gosimple/slug"
	"github.com/lib/pq"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

type dbPost struct {
	ID                 int            `db:"id"`
	Number             int            `db:"number"`
	Title              string         `db:"title"`
	Slug               string         `db:"slug"`
	Description        string         `db:"description"`
	CreatedAt          time.Time      `db:"created_at"`
	LastActivityAt     time.Time      `db:"last_activity_at"`
	User               *dbUser        `db:"user"`
	VoteType           sql.NullInt32  `db:"vote_type"`
	VoteRevision       int64          `db:"vote_revision"`
	VotesCount         int            `db:"votes_count"`
	CommentsCount      int            `db:"comments_count"`
	RecentVotes        int            `db:"recent_votes_count"`
	RecentComments     int            `db:"recent_comments_count"`
	Upvotes            int            `db:"upvotes"`
	Downvotes          int            `db:"downvotes"`
	Status             int            `db:"status"`
	Response           sql.NullString `db:"response"`
	RespondedAt        dbx.NullTime   `db:"response_date"`
	ResponseUser       *dbUser        `db:"response_user"`
	OriginalNumber     sql.NullInt64  `db:"original_number"`
	OriginalTitle      sql.NullString `db:"original_title"`
	OriginalSlug       sql.NullString `db:"original_slug"`
	OriginalStatus     sql.NullInt64  `db:"original_status"`
	Tags               []string       `db:"tags"`
	LockedSettings     sql.NullString `db:"locked_settings"`
	LockedBy           *dbUser        `db:"locked_by"`
	FirstTaggedAt      sql.NullTime   `db:"first_tagged_at"`
	ArchivedAt         dbx.NullTime   `db:"archived_at"`
	ArchivedFromStatus sql.NullInt64  `db:"archived_from_status"`
	ModerationPending  bool           `db:"moderation_pending"`
	ModerationData     sql.NullString `db:"moderation_data"`
}

func (i *dbPost) toModel(ctx context.Context) *entity.Post {
	voteType := 0
	if i.VoteType.Valid {
		voteType = int(i.VoteType.Int32)
	}

	post := &entity.Post{
		ID:             i.ID,
		Number:         i.Number,
		Title:          i.Title,
		Slug:           i.Slug,
		Description:    i.Description,
		CreatedAt:      i.CreatedAt,
		LastActivityAt: i.LastActivityAt,
		VoteType:       voteType,
		VoteRevision:   i.VoteRevision,
		VotesCount:     i.VotesCount,
		CommentsCount:  i.CommentsCount,
		Status:         enum.PostStatus(i.Status),
		User:           i.User.toModel(ctx),
		Tags:           i.Tags,
		LockedSettings: nil,
		Upvotes:        i.Upvotes,
		Downvotes:      i.Downvotes,
	}

	if i.FirstTaggedAt.Valid {
		post.FirstTaggedAt = &i.FirstTaggedAt.Time
	}

	if i.Response.Valid {
		post.Response = &entity.PostResponse{
			Text:        i.Response.String,
			RespondedAt: i.RespondedAt.Time,
			User:        i.ResponseUser.toModel(ctx),
		}
		if post.Status == enum.PostDuplicate && i.OriginalNumber.Valid {
			post.Response.Original = &entity.OriginalPost{
				Number: int(i.OriginalNumber.Int64),
				Slug:   i.OriginalSlug.String,
				Title:  i.OriginalTitle.String,
				Status: enum.PostStatus(i.OriginalStatus.Int64),
			}
		}
	}

	if i.LockedSettings.Valid {
		var lockedSettings entity.PostLockedSettings
		err := json.Unmarshal([]byte(i.LockedSettings.String), &lockedSettings)
		if err == nil && lockedSettings.Locked {
			if lockedBy := i.LockedBy.toModel(ctx); lockedBy != nil {
				lockedSettings.LockedBy = lockedBy
			}
			post.LockedSettings = &lockedSettings
		}
	}

	if post.Status == enum.PostArchived && i.ArchivedAt.Valid {
		post.ArchivedSettings = &entity.PostArchivedSettings{
			ArchivedAt:     i.ArchivedAt.Time,
			PreviousStatus: enum.PostStatus(i.ArchivedFromStatus.Int64),
		}
	}

	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	post.ModerationPending = i.ModerationPending
	post.DiscussionPermissions = entity.PostDiscussion(post).Permissions(user, tenant)
	post.Permissions = post.AllowedActions(user, tenant, time.Now())

	isStaff := user != nil && (user.IsCollaborator() || user.IsModerator() || user.IsAdministrator())
	if isStaff {
		if i.ModerationData.Valid {
			post.ModerationData = i.ModerationData.String
		}
	} else {
		post.ModerationPending = false
	}

	return post
}

type cteResult struct {
	SQL    string
	Params []interface{}
}

// getSortExpression will return the ORDER BY expression for a given view
func getSortExpression(view string) (sort string, sortDir string) {
	sortDir = "DESC"
	switch view {
	case "newest":
		sort = "p.created_at"
	case "oldest":
		sort = "p.created_at"
		sortDir = "ASC"
	case "most-wanted":
		sort = "(p.upvotes - p.downvotes)"
	case "least-wanted":
		sort = "(p.upvotes - p.downvotes)"
		sortDir = "ASC"
	case "most-discussed":
		sort = "p.comments_count"
	case "planned", "started", "completed", "declined":
		sort = "p.response_date"
	case "all":
		sort = "p.created_at"
	case "controversial":
		sort = "CASE " +
			"WHEN p.upvotes > 0 OR p.downvotes > 0 THEN " +
			"(p.upvotes + p.downvotes) * (1 - ABS(p.upvotes - p.downvotes)::float / GREATEST(p.upvotes + p.downvotes, 1)) / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - p.created_at)/86400) + 1, 0.5) " +
			"ELSE 0 " +
			"END"
	case "trending":
		fallthrough
	default:
		sort = "(" +
			"COALESCE(p.recent_comments, 0)*3 + " +
			"CASE " +
			"  WHEN COALESCE(p.recent_votes, 0) >= 0 THEN COALESCE(p.recent_votes, 0)*5 " +
			"  WHEN COALESCE(p.recent_votes, 0) > -10 THEN 0 " +
			"  ELSE COALESCE(p.recent_votes, 0)*5 " +
			"END + " +
			"CASE WHEN (p.upvotes > 20) THEN p.upvotes/2 ELSE 0 END" +
			") / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - p.last_activity_at)/86400) + 2, 0.8)"
	}
	return sort, sortDir
}

// getStatusFilters will return the status filters for a given view
func getStatusFilters(view string, providedStatuses []enum.PostStatus) []enum.PostStatus {
	if len(providedStatuses) > 0 {
		return providedStatuses
	}

	switch view {
	case "planned":
		return []enum.PostStatus{enum.PostPlanned}
	case "started":
		return []enum.PostStatus{enum.PostStarted}
	case "completed":
		return []enum.PostStatus{enum.PostCompleted}
	case "declined":
		return []enum.PostStatus{enum.PostDeclined}
	case "all":
		return []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
			enum.PostCompleted,
			enum.PostDeclined,
		}
	default:
		return []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
		}
	}
}

// getDateInterval will convert date string to SQL interval
func getDateInterval(date string) string {
	switch date {
	case "1d":
		return "1 day"
	case "7d":
		return "7 days"
	case "30d":
		return "30 days"
	case "6m":
		return "6 months"
	case "1y":
		return "1 year"
	default:
		return ""
	}
}

func buildCTE(q query.SearchPosts, tenantID int, user *entity.User) cteResult {
	statuses := getStatusFilters(q.View, q.Statuses)
	sort, sortDir := getSortExpression(q.View)
	userID := viewerID(user)

	if q.Query != "" && len(q.Statuses) == 0 {
		statuses = getStatusFilters("all", nil)
		if q.View == "make-post" {
			statuses = append(statuses, enum.PostDuplicate)
		}
	}

	conditions := []string{"p.status = ANY($4)"}
	params := []interface{}{tenantID, viewerRole(user), userID, pq.Array(statuses)}
	paramIdx := 5
	if q.View == "recently-updated" && q.Query == "" {
		sort = "CASE WHEN p.status = $5 THEN -999999999 ELSE extract(epoch from COALESCE(p.response_date, p.created_at)) END"
		params = append(params, enum.PostOpen)
		paramIdx++
	}

	if len(q.IDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("p.id = ANY($%d)", paramIdx))
		params = append(params, pq.Array(q.IDs))
		paramIdx++
	}

	if interval := getDateInterval(q.Date); interval != "" {
		conditions = append(conditions, fmt.Sprintf("p.created_at >= NOW() - $%d::interval", paramIdx))
		params = append(params, interval)
		paramIdx++
	}

	if q.Untagged {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id)")
	}

	if q.MyPostsOnly && userID > 0 {
		conditions = append(conditions, "p.user_id = $3")
	}

	if q.MyVotesOnly && userID > 0 {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM post_votes v WHERE v.post_id = p.id AND v.user_id = $3)")
	}

	if q.NotMyVotes && userID > 0 {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM post_votes v WHERE v.post_id = p.id AND v.user_id = $3)")
	}

	tagJoin := ""
	if len(q.Tags) > 0 {
		params = append(params, pq.Array(q.Tags))
		tagFilter := fmt.Sprintf(`
			SELECT pt.post_id
			FROM post_tags pt
			INNER JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
			WHERE pt.tenant_id = $1 AND t.slug = ANY($%d)
			GROUP BY pt.post_id
		`, paramIdx)

		if q.TagLogic == "AND" {
			tagFilter += fmt.Sprintf(`
				HAVING COUNT(DISTINCT t.slug) = cardinality($%d::text[])
			`, paramIdx)
		}

		tagJoin = fmt.Sprintf("INNER JOIN (%s) matching ON matching.post_id = p.id", tagFilter)
	}

	if q.Query != "" {
		queryParameter := len(params) + 1
		textParameter := len(params) + 2
		params = append(params, ToTSQuery(q.Query), SanitizeString(q.Query))
		vector := `setweight(to_tsvector('english', COALESCE(p.title, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE(p.description, '')), 'B')`
		conditions = append(conditions, fmt.Sprintf("(%s) @@ to_tsquery('english', $%d)", vector, queryParameter))
		sort = fmt.Sprintf(`ts_rank(%s, to_tsquery('english', $%d)) +
			similarity(p.title, $%d) + similarity(p.description, $%d)`, vector, queryParameter, textParameter, textParameter)
		sortDir = "DESC"
	}

	cteSQL := fmt.Sprintf(`
		SELECT p.id, (%s) AS ranking_score
		FROM visible_posts_for($1, $2, $3) p
		%s
		WHERE %s
		ORDER BY ranking_score %s, p.id DESC
	`, sort, tagJoin, strings.Join(conditions, " AND "), sortDir)

	return cteResult{SQL: cteSQL, Params: params}
}

const postDetails = `
		SELECT 
			p.id,
			p.number,
			p.title,
			p.slug,
			p.description,
			p.created_at,
			p.last_activity_at,
			(p.upvotes - p.downvotes) AS votes_count,
			p.comments_count,
			p.recent_votes AS recent_votes_count,
			p.recent_comments AS recent_comments_count,
			p.upvotes,
			p.downvotes,
			p.status,
			u.id AS user_id,
			u.name AS user_name,
			u.email AS user_email,
			u.role AS user_role,
			u.visual_role AS user_visual_role,
			u.status AS user_status,
			u.avatar_type AS user_avatar_type,
			u.avatar_bkey AS user_avatar_bkey,
			p.response,
			p.response_date,
			r.id AS response_user_id,
			r.name AS response_user_name,
			r.email AS response_user_email,
			r.role AS response_user_role,
			r.visual_role AS response_user_visual_role,
			r.status AS response_user_status,
			r.avatar_type AS response_user_avatar_type,
			r.avatar_bkey AS response_user_avatar_bkey,
			d.number AS original_number,
			d.title AS original_title,
			d.slug AS original_slug,
			d.status AS original_status,
			COALESCE(agg_t.tags, ARRAY[]::text[]) AS tags,
			p.locked_settings,
			l.id AS locked_by_id,
			l.name AS locked_by_name,
			l.role AS locked_by_role,
			l.visual_role AS locked_by_visual_role,
			l.status AS locked_by_status,
			l.avatar_type AS locked_by_avatar_type,
			l.avatar_bkey AS locked_by_avatar_bkey,
			p.archived_at,
			p.archived_from_status,
			agg_t.first_tagged_at,
			CASE WHEN $3 = 0 THEN NULL ELSE
				(SELECT vote_type FROM post_votes WHERE post_id = p.id AND user_id = $3 LIMIT 1)
			END AS vote_type,
			CASE WHEN $3 = 0 THEN 0 ELSE
				COALESCE((SELECT revision FROM post_vote_revisions WHERE post_id = p.id AND user_id = $3), 0)
			END AS vote_revision,
			p.moderation_pending,
			p.moderation_data
		FROM visible_posts_for($1, $2, $3) p
		INNER JOIN users u ON u.id = p.user_id AND u.tenant_id = $1
		LEFT JOIN users r ON r.id = p.response_user_id AND r.tenant_id = $1
		LEFT JOIN users l ON l.id = (p.locked_settings->'lockedBy'->>'id')::integer
			AND l.tenant_id = $1 AND l.status <> 2
		LEFT JOIN visible_posts_for($1, $2, $3) d ON d.id = p.original_id
		LEFT JOIN LATERAL (
			SELECT 
				ARRAY_REMOVE(ARRAY_AGG(t.slug), NULL) AS tags,
				MIN(pt.created_at) AS first_tagged_at
			FROM post_tags pt
			INNER JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
			WHERE pt.post_id = p.id AND pt.tenant_id = $1
			  AND (t.is_public OR $2 IN ('administrator', 'collaborator', 'moderator'))
			GROUP BY pt.post_id
		) agg_t ON true
	`

func buildSearchQuery(q query.SearchPosts, tenant *entity.Tenant, user *entity.User) (string, []interface{}) {
	_, sortDir := getSortExpression(q.View)
	if q.Query != "" {
		sortDir = "DESC"
	}

	cte := buildCTE(q, tenant.ID, user)
	cteWithLimit := cte.SQL
	if q.Limit != "" && q.Limit != "all" {
		cteWithLimit += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(cte.Params)+1, len(cte.Params)+2)
		cte.Params = append(cte.Params, q.Limit, q.Offset)
	}

	fullSQL := "WITH top_posts AS (" + cteWithLimit + ")" + postDetails +
		" JOIN top_posts tp ON tp.id = p.id" +
		" ORDER BY tp.ranking_score " + sortDir + ", tp.id DESC"
	return fullSQL, cte.Params
}

func buildPostsByIDsQuery(tenant *entity.Tenant, user *entity.User, statuses []enum.PostStatus, postIDs []int) cteResult {
	return cteResult{
		SQL:    postDetails + " WHERE p.status = ANY($4) AND p.id = ANY($5)",
		Params: []interface{}{tenant.ID, viewerRole(user), viewerID(user), pq.Array(statuses), pq.Array(postIDs)},
	}
}

func postIsReferenced(ctx context.Context, q *query.PostIsReferenced) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = false

		exists, err := trx.Exists(`
			SELECT 1 FROM posts p 
			INNER JOIN posts o
			ON o.tenant_id = p.tenant_id
			AND o.id = p.original_id
			WHERE p.tenant_id = $1
			AND o.id = $2`, tenant.ID, q.PostID)
		if err != nil {
			return errors.Wrap(err, "failed to check if post is referenced")
		}

		q.Result = exists
		return nil
	})
}

func setPostResponse(ctx context.Context, c *cmd.SetPostResponse) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if c.Status == enum.PostDuplicate {
			return errors.New("Use MarkAsDuplicate to change an post status to Duplicate")
		}

		respondedAt := time.Now()
		if c.Post.Status == c.Status && c.Post.Response != nil {
			respondedAt = c.Post.Response.RespondedAt
		}

		_, err := trx.Execute(`
		UPDATE posts 
		SET response = $3, original_id = NULL, response_date = $4, response_user_id = $5, status = $6 
		WHERE id = $1 and tenant_id = $2
		`, c.Post.ID, tenant.ID, c.Text, respondedAt, user.ID, c.Status)
		if err != nil {
			return errors.Wrap(err, "failed to update post's response")
		}

		c.Post.Status = c.Status
		c.Post.Response = &entity.PostResponse{
			Text:        c.Text,
			RespondedAt: respondedAt,
			User:        user,
		}
		return nil
	})
}

func markPostAsDuplicate(ctx context.Context, c *cmd.MarkPostAsDuplicate) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := trx.Execute(`SELECT id FROM posts WHERE tenant_id = $1 AND id IN ($2, $3)
			ORDER BY id FOR NO KEY UPDATE`, tenant.ID, c.Post.ID, c.Original.ID); err != nil {
			return err
		}
		respondedAt := time.Now()
		if c.Post.Status == enum.PostDuplicate && c.Post.Response != nil {
			respondedAt = c.Post.Response.RespondedAt
		}

		_, err := trx.Execute(`
		UPDATE posts 
		SET response = $7, original_id = $3, response_date = $4, response_user_id = $5, status = $6 
		WHERE id = $1 and tenant_id = $2
		`, c.Post.ID, tenant.ID, c.Original.ID, respondedAt, user.ID, enum.PostDuplicate, c.Text)
		if err != nil {
			return errors.Wrap(err, "failed to update post's response")
		}

		c.Post.Status = enum.PostDuplicate
		c.Post.Response = &entity.PostResponse{
			Text:        c.Text,
			RespondedAt: respondedAt,
			User:        user,
			Original: &entity.OriginalPost{
				Number: c.Original.Number,
				Title:  c.Original.Title,
				Slug:   c.Original.Slug,
				Status: c.Original.Status,
			},
		}

		_, err = trx.Execute(`INSERT INTO post_votes (user_id, post_id, tenant_id, vote_type, created_at)
			SELECT user_id, $1, tenant_id, vote_type, NOW()
			FROM post_votes WHERE post_id = $2 AND tenant_id = $3
			ON CONFLICT (user_id, post_id) DO UPDATE
			SET vote_type = EXCLUDED.vote_type, created_at = EXCLUDED.created_at
			WHERE post_votes.vote_type IS DISTINCT FROM EXCLUDED.vote_type`, c.Original.ID, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to transfer votes to original post")
		}

		return nil
	})
}

func countPostPerStatus(ctx context.Context, q *query.CountPostPerStatus) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {

		type dbStatusCount struct {
			Status enum.PostStatus `db:"status"`
			Count  int             `db:"count"`
		}

		q.Result = make(map[enum.PostStatus]int)
		stats := []*dbStatusCount{}
		err := trx.Select(&stats, "SELECT status, COUNT(*) AS count FROM posts WHERE tenant_id = $1 GROUP BY status", tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to count posts per status")
		}

		for _, v := range stats {
			q.Result[v.Status] = v.Count
		}
		return nil
	})
}

func addNewPost(ctx context.Context, c *cmd.AddNewPost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		attachments, err := uploadAttachments(ctx, c.Attachments)
		if err != nil {
			return err
		}

		inserted := dbPost{Tags: []string{}}
		err = trx.Get(&inserted, `
			WITH inserted AS (
				INSERT INTO posts (title, slug, description, tenant_id, user_id, created_at, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				RETURNING *
			)
			SELECT p.id, p.number, p.title, p.slug, p.description,
				p.created_at, p.last_activity_at, p.status,
				p.upvotes, p.downvotes, (p.upvotes - p.downvotes) AS votes_count,
				p.comments_count, p.recent_votes AS recent_votes_count,
				p.recent_comments AS recent_comments_count,
				p.moderation_pending, p.moderation_data,
				u.id AS user_id, u.name AS user_name, u.email AS user_email,
				u.role AS user_role, u.visual_role AS user_visual_role,
				u.status AS user_status, u.avatar_type AS user_avatar_type,
				u.avatar_bkey AS user_avatar_bkey
			FROM inserted p
			JOIN users u ON u.id = p.user_id AND u.tenant_id = p.tenant_id
		`, c.Title, slug.Make(c.Title), c.Description, tenant.ID, user.ID, time.Now(), enum.PostOpen)
		if err != nil {
			return errors.Wrap(err, "failed add new post")
		}

		if err := attachments.apply(ctx, inserted.ID, 0); err != nil {
			return err
		}

		c.Result = inserted.toModel(ctx)

		if err := internalAddSubscriber(trx, c.Result, tenant, user, false); err != nil {
			return err
		}

		return nil
	})
}

func updatePost(ctx context.Context, c *cmd.UpdatePost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		attachments, err := uploadAttachments(ctx, c.Attachments)
		if err != nil {
			return err
		}

		_, err = trx.Execute(`
			UPDATE posts SET title = $1, slug = $2, description = $3
			WHERE id = $4 AND tenant_id = $5
		`, c.Title, slug.Make(c.Title), c.Description, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed update post")
		}

		if err := attachments.apply(ctx, c.Post.ID, 0); err != nil {
			return err
		}

		q := &query.GetPostByID{PostID: c.Post.ID}
		if err := getPostByID(ctx, q); err != nil {
			return err
		}
		c.Result = q.Result
		return nil
	})
}

func getPostByID(ctx context.Context, q *query.GetPostByID) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var post dbPost
		err := trx.Get(&post, postDetails + ` WHERE p.id = $4 LIMIT 1`,
			tenant.ID, viewerRole(user), viewerID(user), q.PostID)
		if err != nil {
			return errors.Wrap(err, "failed to get post with id '%d'", q.PostID)
		}
		q.Result = post.toModel(ctx)
		return nil
	})
}

func getPostBySlug(ctx context.Context, q *query.GetPostBySlug) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var post dbPost
		err := trx.Get(&post, postDetails + ` WHERE p.slug = $4 LIMIT 1`,
			tenant.ID, viewerRole(user), viewerID(user), q.Slug)
		if err != nil {
			return errors.Wrap(err, "failed to get post with slug '%s'", q.Slug)
		}
		q.Result = post.toModel(ctx)
		return nil
	})
}

func getPostByNumber(ctx context.Context, q *query.GetPostByNumber) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var post dbPost
		err := trx.Get(&post, postDetails + ` WHERE p.number = $4 LIMIT 1`,
			tenant.ID, viewerRole(user), viewerID(user), q.Number)
		if err != nil {
			return errors.Wrap(err, "failed to get post with number '%d'", q.Number)
		}
		q.Result = post.toModel(ctx)
		return nil
	})
}

func getUserPostCount(ctx context.Context, q *query.GetUserPostCount) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, currentUser *entity.User) error {
		var count int

		sqlQuery := `
            SELECT COUNT(*) 
            FROM posts
            WHERE tenant_id = $1
              AND user_id = $2
              AND status NOT IN (6, 7)
              AND created_at >= $3
        `

		if err := trx.Scalar(&count, sqlQuery, tenant.ID, q.UserID, q.Since); err != nil {
			return errors.Wrap(err, "failed to get user post count")
		}

		q.Result = count
		return nil
	})
}

func getUserCommentCount(ctx context.Context, q *query.GetUserCommentCount) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, currentUser *entity.User) error {
		var count int

		sqlQuery := `
            SELECT COUNT(*)
            FROM comments
            WHERE tenant_id = $1
              AND user_id = $2
              AND deleted_at IS NULL
              AND created_at >= $3
        `
		if err := trx.Scalar(&count, sqlQuery, tenant.ID, q.UserID, q.Since); err != nil {
			return errors.Wrap(err, "failed to get user comment count")
		}

		q.Result = count
		return nil
	})
}

func countUntaggedPosts(ctx context.Context, q *query.CountUntaggedPosts) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if q.Date == "" {
			q.Date = entity.DefaultQueueDate(user)
		}

		if len(q.Statuses) == 0 {
			q.Statuses = []enum.PostStatus{
				enum.PostOpen,
				enum.PostStarted,
				enum.PostPlanned,
				enum.PostCompleted,
			}
		}

		var count int
		sqlQuery := `
			SELECT COUNT(*)
			FROM posts p
			WHERE p.tenant_id = $1
			  AND p.status = ANY($2)
			  AND NOT EXISTS (SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id)
		`

		args := []any{tenant.ID, pq.Array(q.Statuses)}

		if q.Date != "" {
			var days int
			switch q.Date {
			case "1d":
				days = 1
			case "7d":
				days = 7
			case "30d":
				days = 30
			case "1y":
				days = 365
			}
			if days > 0 {
				sqlQuery += " AND p.created_at >= NOW() - $3 * INTERVAL '1 day'"
				args = append(args, days)
			}
		}

		if err := trx.Scalar(&count, sqlQuery, args...); err != nil {
			return errors.Wrap(err, "failed to count untagged posts")
		}

		q.Result = count
		return nil
	})
}

func searchPosts(ctx context.Context, q *query.SearchPosts) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if q.Untagged && q.Date == "" {
			q.Date = entity.DefaultQueueDate(user)
		}

		// Normalize inputs
		if q.Tags == nil {
			q.Tags = []string{}
		}
		if q.Statuses == nil {
			q.Statuses = []enum.PostStatus{}
		}
		if q.Limit != "all" {
			if _, err := strconv.Atoi(q.Limit); err != nil {
				q.Limit = "30"
			}
		}
		if q.Offset == "" {
			q.Offset = "0"
		}

		// build and execute query
		sqlQuery, params := buildSearchQuery(*q, tenant, user)

		var posts []*dbPost
		err := trx.Select(&posts, sqlQuery, params...)
		if err != nil {
			return errors.Wrap(err, "failed to search posts")
		}

		q.Result = make([]*entity.Post, len(posts))
		for i, post := range posts {
			q.Result[i] = post.toModel(ctx)
		}
		return nil
	})
}

func getAllPosts(ctx context.Context, q *query.GetAllPosts) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		searchQuery := &query.SearchPosts{View: "all", Limit: "all"}
		if err := searchPosts(ctx, searchQuery); err != nil {
			return errors.Wrap(err, "failed to get all posts")
		}
		q.Result = searchQuery.Result
		return nil
	})
}

func getPostsByIDs(ctx context.Context, q *query.GetPostsByIDs) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if len(q.PostIDs) == 0 {
			q.Result = []*entity.Post{}
			return nil
		}

		statuses := []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
			enum.PostCompleted,
			enum.PostDeclined,
		}

		sqlQuery := buildPostsByIDsQuery(tenant, user, statuses, q.PostIDs)

		var posts []*dbPost
		err := trx.Select(&posts, sqlQuery.SQL, sqlQuery.Params...)
		if err != nil {
			return errors.Wrap(err, "failed to get posts by IDs")
		}

		q.Result = make([]*entity.Post, len(posts))
		for i, post := range posts {
			q.Result[i] = post.toModel(ctx)
		}
		return nil
	})
}

func lockPost(ctx context.Context, c *cmd.LockPost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		lockedSettings := map[string]interface{}{
			"locked":   true,
			"lockedAt": time.Now(),
			"lockedBy": map[string]interface{}{
				"id": user.ID,
			},
			"lockMessage": c.LockMessage,
		}

		lockedSettingsJSON, err := json.Marshal(lockedSettings)
		if err != nil {
			return errors.Wrap(err, "failed to marshal locked settings")
		}

		_, err = trx.Execute(`
			UPDATE posts 
			SET locked_settings = $3
			WHERE id = $1 and tenant_id = $2
		`, c.Post.ID, tenant.ID, lockedSettingsJSON)
		if err != nil {
			return errors.Wrap(err, "failed to lock post")
		}

		c.Post.LockedSettings = &entity.PostLockedSettings{
			Locked:      true,
			LockedAt:    time.Now(),
			LockedBy:    user,
			LockMessage: c.LockMessage,
		}
		return nil
	})
}

func unlockPost(ctx context.Context, c *cmd.UnlockPost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE posts 
			SET locked_settings = NULL
			WHERE id = $1 and tenant_id = $2
		`, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to unlock post")
		}

		c.Post.LockedSettings = nil
		return nil
	})
}

func refreshPostStats(ctx context.Context, c *cmd.RefreshPostStats) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, _ *entity.Tenant, _ *entity.User) error {
		baseQuery := `
			UPDATE posts p SET
				recent_votes = COALESCE((
					SELECT SUM(v.vote_type) 
					FROM post_votes v 
					WHERE v.post_id = p.id AND v.tenant_id = p.tenant_id 
					AND v.created_at > CURRENT_DATE - INTERVAL '30 days'
				), 0),
				recent_comments = COALESCE((
					SELECT COUNT(*) 
					FROM comments c 
					WHERE c.post_id = p.id AND c.tenant_id = p.tenant_id 
					AND c.deleted_at IS NULL 
					AND c.created_at > CURRENT_DATE - INTERVAL '30 days'
				), 0)
			WHERE p.status NOT IN ($1, $2)`

		var rowsUpdated int64
		var err error
		if c.Since != nil {
			rowsUpdated, err = trx.Execute(baseQuery+" AND p.last_activity_at >= $3", int(enum.PostDeleted), int(enum.PostArchived), *c.Since)
		} else {
			rowsUpdated, err = trx.Execute(baseQuery, int(enum.PostDeleted), int(enum.PostArchived))
		}
		if err != nil {
			return errors.Wrap(err, "failed to refresh post stats")
		}

		c.RowsUpdated = rowsUpdated
		return nil
	})
}

func archivePost(ctx context.Context, c *cmd.ArchivePost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE posts 
			SET status = $3, archived_at = NOW(), archived_from_status = status
			WHERE id = $1 AND tenant_id = $2
		`, c.Post.ID, tenant.ID, int(enum.PostArchived))
		if err != nil {
			return errors.Wrap(err, "failed to archive post")
		}

		c.Post.ArchivedSettings = &entity.PostArchivedSettings{
			ArchivedAt:     time.Now(),
			ArchivedBy:     user,
			PreviousStatus: c.Post.Status,
		}
		c.Post.Status = enum.PostArchived
		return nil
	})
}

func unarchivePost(ctx context.Context, c *cmd.UnarchivePost) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var previousStatus int
		err := trx.Get(&previousStatus, `
			SELECT COALESCE(archived_from_status, 0) FROM posts WHERE id = $1 AND tenant_id = $2
		`, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get previous status")
		}

		_, err = trx.Execute(`
			UPDATE posts 
			SET status = $3, archived_at = NULL, archived_from_status = NULL
			WHERE id = $1 AND tenant_id = $2
		`, c.Post.ID, tenant.ID, previousStatus)
		if err != nil {
			return errors.Wrap(err, "failed to unarchive post")
		}

		c.Post.Status = enum.PostStatus(previousStatus)
		c.Post.ArchivedSettings = nil
		return nil
	})
}

func bulkArchivePosts(ctx context.Context, c *cmd.BulkArchivePosts) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if len(c.PostIDs) == 0 {
			return nil
		}

		_, err := trx.Execute(`
			UPDATE posts 
			SET status = $2, archived_at = NOW(), archived_from_status = status
			WHERE tenant_id = $1 AND id = ANY($3) AND status NOT IN ($4, $5)
		`, tenant.ID, int(enum.PostArchived), pq.Array(c.PostIDs), int(enum.PostDeleted), int(enum.PostArchived))
		if err != nil {
			return errors.Wrap(err, "failed to bulk archive posts")
		}

		return nil
	})
}

func getArchivablePosts(ctx context.Context, q *query.GetArchivablePosts) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		conditions := []string{"p.status <> $4"}
		args := []interface{}{tenant.ID, viewerRole(user), viewerID(user), int(enum.PostArchived)}
		argNum := 5

		if q.CreatedBefore != nil {
			conditions = append(conditions, fmt.Sprintf("p.created_at < $%d", argNum))
			args = append(args, *q.CreatedBefore)
			argNum++
		}

		if q.InactiveSince != nil {
			conditions = append(conditions, fmt.Sprintf("p.last_activity_at < $%d", argNum))
			args = append(args, *q.InactiveSince)
			argNum++
		}

		if q.MaxVotes != nil {
			conditions = append(conditions, fmt.Sprintf("(p.upvotes - p.downvotes) < $%d", argNum))
			args = append(args, *q.MaxVotes)
			argNum++
		}

		if q.MaxComments != nil {
			conditions = append(conditions, fmt.Sprintf("p.comments_count < $%d", argNum))
			args = append(args, *q.MaxComments)
			argNum++
		}

		if len(q.Statuses) > 0 {
			statusInts := make([]int, len(q.Statuses))
			for i, s := range q.Statuses {
				statusInts[i] = int(s)
			}
			conditions = append(conditions, fmt.Sprintf("p.status = ANY($%d)", argNum))
			args = append(args, pq.Array(statusInts))
			argNum++
		}

		if len(q.Tags) > 0 {
			conditions = append(conditions, fmt.Sprintf(`
				EXISTS (
					SELECT 1 FROM post_tags pt 
					INNER JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
					WHERE pt.post_id = p.id AND t.slug = ANY($%d)
				)
			`, argNum))
			args = append(args, pq.Array(q.Tags))
			argNum++
		}

		whereClause := strings.Join(conditions, " AND ")

		countQuery := `SELECT COUNT(*) FROM visible_posts_for($1, $2, $3) p WHERE ` + whereClause
		err := trx.Get(&q.Total, countQuery, args...)
		if err != nil {
			return errors.Wrap(err, "failed to count archivable posts")
		}

		if q.PerPage <= 0 {
			q.PerPage = 50
		}
		if q.Page <= 0 {
			q.Page = 1
		}

		const maxInt32 = 2147483647
		if q.PerPage > maxInt32 {
			q.PerPage = maxInt32
		}
		if q.Page > maxInt32 {
			q.Page = maxInt32
		}

		offset := (q.Page - 1) * q.PerPage
		if offset > maxInt32 {
			offset = maxInt32
		}

		cte := fmt.Sprintf(`
			SELECT p.id, p.last_activity_at AS ranking_score
			FROM visible_posts_for($1, $2, $3) p
			WHERE %s
			ORDER BY p.last_activity_at ASC
			LIMIT $%d OFFSET $%d
		`, whereClause, argNum, argNum+1)
		args = append(args, q.PerPage, offset)

		selectQuery := "WITH top_posts AS (" + cte + ")" + postDetails +
			" JOIN top_posts tp ON tp.id = p.id" +
			" ORDER BY tp.ranking_score ASC, tp.id DESC"

		var posts []*dbPost
		err = trx.Select(&posts, selectQuery, args...)
		if err != nil {
			return errors.Wrap(err, "failed to get archivable posts")
		}

		q.Result = make([]*entity.Post, len(posts))
		for i, p := range posts {
			q.Result[i] = p.toModel(ctx)
		}

		return nil
	})
}

func countVotesSinceArchive(ctx context.Context, q *query.CountVotesSinceArchive) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		err := trx.Scalar(&q.Result, `
			SELECT COALESCE(SUM(vote_type), 0) 
			FROM post_votes 
			WHERE post_id = $1 AND tenant_id = $2 AND created_at > $3
		`, q.PostID, tenant.ID, q.ArchivedAt)
		if err != nil {
			return errors.Wrap(err, "failed to count votes since archive")
		}
		return nil
	})
}
