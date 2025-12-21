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

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

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
	TagDates           sql.NullString `db:"tag_dates"`
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
		VotesCount:     i.VotesCount,
		CommentsCount:  i.CommentsCount,
		Status:         enum.PostStatus(i.Status),
		User:           i.User.toModel(ctx),
		Tags:           i.Tags,
		LockedSettings: nil,
		Upvotes:        i.Upvotes,
		Downvotes:      i.Downvotes,
	}

	if i.TagDates.Valid {
		post.TagDates = i.TagDates.String
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
			if lockedSettings.LockedBy != nil {
				getUser := &query.GetUserByID{UserID: lockedSettings.LockedBy.ID}
				if err := bus.Dispatch(ctx, getUser); err == nil {
					lockedSettings.LockedBy = getUser.Result
				}
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
	isStaff := user != nil && (user.IsCollaborator() || user.IsModerator() || user.IsAdministrator())
	if isStaff {
		post.ModerationPending = i.ModerationPending
		if i.ModerationData.Valid {
			post.ModerationData = i.ModerationData.String
		}
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
	case "recently-updated":
		sort = fmt.Sprintf("CASE WHEN p.status = %d THEN -999999999 ELSE extract(epoch from COALESCE(p.response_date, p.created_at)) END", int(enum.PostOpen))
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
		if len(providedStatuses) > 0 {
			return providedStatuses
		}
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

// buildCTE will constructs the CTE that finds post IDs based on filters
// it uses different strategies depending on which filters are active
func buildCTE(q query.SearchPosts, tenantID int, userID int) cteResult {
	statuses := getStatusFilters(q.View, q.Statuses)
	sort, sortDir := getSortExpression(q.View)

	// base conditions that always apply
	conditions := []string{
		"p.tenant_id = $1",
		"p.status = ANY($2)",
		fmt.Sprintf("p.status != %d", int(enum.PostDeleted)),
	}
	params := []interface{}{tenantID, pq.Array(statuses)}
	paramIdx := 3

	// date filter
	if interval := getDateInterval(q.Date); interval != "" {
		conditions = append(conditions, fmt.Sprintf("p.created_at >= NOW() - INTERVAL '%s'", interval))
	}

	// untagged filter
	if q.Untagged {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id)")
	}

	// my posts filter
	if q.MyPostsOnly && userID > 0 {
		conditions = append(conditions, fmt.Sprintf("p.user_id = $%d", paramIdx))
		params = append(params, userID)
		paramIdx++
	}

	// determine the driving strategy based on the filters the user has applied
	var cteSQL string

	if len(q.Tags) > 0 {
		// tags: start from post_tags to leverage index on the tags table
		tagConditions := make([]string, 0)
		if q.TagLogic == "AND" {
			// for AND logic, we need posts that have ALL the specified tags
			for _, tag := range q.Tags {
				tagConditions = append(tagConditions, fmt.Sprintf("$%d", paramIdx))
				params = append(params, tag)
				paramIdx++
			}
			tagFilter := fmt.Sprintf(`
				SELECT pt.post_id
				FROM post_tags pt
				INNER JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
				WHERE pt.tenant_id = $1 AND t.slug IN (%s)
				GROUP BY pt.post_id
				HAVING COUNT(DISTINCT t.slug) = %d
			`, strings.Join(tagConditions, ","), len(q.Tags))

			cteSQL = fmt.Sprintf(`
				SELECT p.id, (%s) AS ranking_score
				FROM posts p
				INNER JOIN (%s) matching ON matching.post_id = p.id
				WHERE %s
				ORDER BY ranking_score %s
			`, sort, tagFilter, strings.Join(conditions, " AND "), sortDir)
		} else {
			// OR logic . any of the tags
			params = append(params, pq.Array(q.Tags))
			tagFilter := fmt.Sprintf(`
				SELECT DISTINCT pt.post_id
				FROM post_tags pt
				INNER JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
				WHERE pt.tenant_id = $1 AND t.slug = ANY($%d)
			`, paramIdx)
			paramIdx++

			cteSQL = fmt.Sprintf(`
				SELECT p.id, (%s) AS ranking_score
				FROM posts p
				INNER JOIN (%s) matching ON matching.post_id = p.id
				WHERE %s
				ORDER BY ranking_score %s
			`, sort, tagFilter, strings.Join(conditions, " AND "), sortDir)
		}
	} else if q.MyVotesOnly && userID > 0 {
		// when a user is searching for their votes: start from post_votes
		params = append(params, userID)
		voteFilter := fmt.Sprintf(`
			SELECT DISTINCT post_id FROM post_votes WHERE tenant_id = $1 AND user_id = $%d
		`, paramIdx)
		paramIdx++

		cteSQL = fmt.Sprintf(`
			SELECT p.id, (%s) AS ranking_score
			FROM posts p
			INNER JOIN (%s) my_votes ON my_votes.post_id = p.id
			WHERE %s
			ORDER BY ranking_score %s
		`, sort, voteFilter, strings.Join(conditions, " AND "), sortDir)
	} else if q.NotMyVotes && userID > 0 {
		// anti joining to use NOT EXISTS for "not my votes"
		params = append(params, userID)
		conditions = append(conditions, fmt.Sprintf("NOT EXISTS (SELECT 1 FROM post_votes v WHERE v.post_id = p.id AND v.user_id = $%d)", paramIdx))
		paramIdx++

		cteSQL = fmt.Sprintf(`
			SELECT p.id, (%s) AS ranking_score
			FROM posts p
			WHERE %s
			ORDER BY ranking_score %s
		`, sort, strings.Join(conditions, " AND "), sortDir)
	} else {
		// if all other strategies fail, we'll use a dumb posts scan with status filter
		cteSQL = fmt.Sprintf(`
			SELECT p.id, (%s) AS ranking_score
			FROM posts p
			WHERE %s
			ORDER BY ranking_score %s
		`, sort, strings.Join(conditions, " AND "), sortDir)
	}

	return cteResult{SQL: cteSQL, Params: params}
}

// buildTextSearchCTE constructs a CTE for text search queries
func buildTextSearchCTE(searchQuery string, tenantID int, statuses []enum.PostStatus) cteResult {
	params := []interface{}{tenantID, pq.Array(statuses), ToTSQuery(searchQuery), SanitizeString(searchQuery)}

	cteSQL := fmt.Sprintf(`
		SELECT p.id,
			ts_rank(
				setweight(to_tsvector('english', COALESCE(p.title, '')), 'A') || 
				setweight(to_tsvector('english', COALESCE(p.description, '')), 'B'), 
				to_tsquery('english', $3)
			) + similarity(p.title, $4) + similarity(p.description, $4) AS ranking_score
		FROM posts p
		WHERE p.tenant_id = $1 
		  AND p.status = ANY($2)
		  AND p.status != %d
		  AND (
			setweight(to_tsvector('english', COALESCE(p.title, '')), 'A') || 
			setweight(to_tsvector('english', COALESCE(p.description, '')), 'B')
		  ) @@ to_tsquery('english', $3)
		ORDER BY ranking_score DESC
	`, int(enum.PostDeleted))

	return cteResult{SQL: cteSQL, Params: params}
}

// buildHydration constructs the SELECT that fetches all post details
// it takes a CTE name and produces the full hydration query
func buildHydration(tenantID int, user *entity.User, cteName string, limit string, offset string, sortDir string) string {
	tagCondition := "AND t.is_public = true"
	if user != nil && (user.IsCollaborator() || user.IsModerator()) {
		tagCondition = ""
	}

	// determine if tag_dates should be shown because helpers
	// are restricted to tagging posts only before a certain period of time
	tagDatesField := "NULL::jsonb"
	if user != nil && (user.IsCollaborator() || user.IsModerator() || user.IsAdministrator() || user.IsHelper()) {
		tagDatesField = "agg_t.tag_dates"
	}

	voteTypeField := "NULL::int"
	if user != nil {
		voteTypeField = fmt.Sprintf("(SELECT vote_type FROM post_votes WHERE post_id = p.id AND user_id = %d LIMIT 1)", user.ID)
	}

	moderationFilter := ""
	if user == nil {
		moderationFilter = "AND p.moderation_pending = FALSE"
	} else if !user.IsCollaborator() && !user.IsModerator() && !user.IsAdministrator() {
		moderationFilter = fmt.Sprintf("AND (p.moderation_pending = FALSE OR p.user_id = %d)", user.ID)
	}

	orderClause := ""
	if sortDir != "" {
		orderClause = fmt.Sprintf("ORDER BY tp.ranking_score %s", sortDir)
	}

	limitClause := ""
	if limit != "" && limit != "all" {
		limitClause = fmt.Sprintf("LIMIT %s OFFSET %s", limit, offset)
	}

	return fmt.Sprintf(`
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
			p.archived_at,
			p.archived_from_status,
			%s AS tag_dates,
			%s AS vote_type,
			p.moderation_pending,
			p.moderation_data
		FROM %s tp
		JOIN posts p ON p.id = tp.id %s
		INNER JOIN users u ON u.id = p.user_id AND u.tenant_id = %d
		LEFT JOIN users r ON r.id = p.response_user_id AND r.tenant_id = %d
		LEFT JOIN posts d ON d.id = p.original_id AND d.tenant_id = %d
		LEFT JOIN LATERAL (
			SELECT 
				ARRAY_REMOVE(ARRAY_AGG(t.slug), NULL) AS tags,
				jsonb_agg(
					jsonb_build_object('slug', t.slug, 'created_at', pt.created_at)
				) AS tag_dates
			FROM post_tags pt
			INNER JOIN tags t ON t.id = pt.tag_id AND t.tenant_id = pt.tenant_id
			WHERE pt.post_id = p.id AND pt.tenant_id = %d %s
			GROUP BY pt.post_id
		) agg_t ON true
		%s
		%s
	`, tagDatesField, voteTypeField, cteName, moderationFilter, tenantID, tenantID, tenantID, tenantID, tagCondition, orderClause, limitClause)
}

// this will combine the buildCTE and buildHydration into a complete query to search for posts
func buildSearchQuery(q query.SearchPosts, tenant *entity.Tenant, user *entity.User) (string, []interface{}) {
	userID := 0
	if user != nil {
		userID = user.ID
	}

	// get sort direction for the view
	_, sortDir := getSortExpression(q.View)

	// handle text search separately
	if q.Query != "" {
		statuses := []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
			enum.PostCompleted,
			enum.PostDeclined,
		}
		if q.View == "make-post" {
			statuses = append(statuses, enum.PostDuplicate)
		}
		cte := buildTextSearchCTE(q.Query, tenant.ID, statuses)
		// text search always uses DESC, naybe we change later
		hydration := buildHydration(tenant.ID, user, "top_posts", q.Limit, q.Offset, "DESC")

		fullSQL := fmt.Sprintf("WITH top_posts AS (%s LIMIT %s OFFSET %s) %s", cte.SQL, q.Limit, q.Offset, hydration)
		return fullSQL, cte.Params
	}

	// build the CTE specifically for non text search queries
	cte := buildCTE(q, tenant.ID, userID)

	// add LIMIT/OFFSET to the CTE
	cteWithLimit := cte.SQL
	if q.Limit != "" && q.Limit != "all" {
		cteWithLimit = fmt.Sprintf("%s LIMIT %s OFFSET %s", cte.SQL, q.Limit, q.Offset)
	}

	hydration := buildHydration(tenant.ID, user, "top_posts", "", "", sortDir)
	fullSQL := fmt.Sprintf("WITH top_posts AS (%s) %s", cteWithLimit, hydration)

	return fullSQL, cte.Params
}

func buildSinglePostQuery(tenant *entity.Tenant, user *entity.User, condition string) string {
	cte := fmt.Sprintf(`
		SELECT p.id, 0 AS ranking_score
		FROM posts p
		WHERE p.tenant_id = $1 AND %s
	`, condition)

	hydration := buildHydration(tenant.ID, user, "top_posts", "1", "0", "")
	return fmt.Sprintf("WITH top_posts AS (%s) %s", cte, hydration)
}

func buildPostsByIDsQuery(tenant *entity.Tenant, user *entity.User) string {
	cte := `
		SELECT p.id, 0 AS ranking_score
		FROM posts p
		WHERE p.tenant_id = $1 AND p.status = ANY($2) AND p.id = ANY($3)
	`

	hydration := buildHydration(tenant.ID, user, "top_posts", "", "", "")
	return fmt.Sprintf("WITH top_posts AS (%s) %s", cte, hydration)
}

func postIsReferenced(ctx context.Context, q *query.PostIsReferenced) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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

		var votes []*struct {
			UserID   int           `db:"user_id"`
			VoteType enum.VoteType `db:"vote_type"`
		}
		err = trx.Select(&votes, "SELECT user_id, vote_type FROM post_votes WHERE post_id = $1 AND tenant_id = $2", c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get votes of post with id '%d'", c.Post.ID)
		}

		for _, vote := range votes {
			_, err = trx.Execute(`
				DELETE FROM post_votes 
				WHERE post_id = $1 AND user_id = $2 AND tenant_id = $3
			`, c.Original.ID, vote.UserID, tenant.ID)

			if err != nil {
				return errors.Wrap(err, "failed to remove existing vote on original post")
			}
		}

		for _, vote := range votes {
			_, err = trx.Execute(`
				INSERT INTO post_votes (user_id, post_id, tenant_id, vote_type, created_at) 
				VALUES ($1, $2, $3, $4, NOW())
			`, vote.UserID, c.Original.ID, tenant.ID, vote.VoteType)

			if err != nil {
				return errors.Wrap(err, "failed to transfer vote to original post")
			}
		}

		return nil
	})
}

func countPostPerStatus(ctx context.Context, q *query.CountPostPerStatus) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {

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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var id int
		err := trx.Get(&id,
			`INSERT INTO posts (title, slug, number, description, tenant_id, user_id, created_at, status) 
			 VALUES ($1, $2, (SELECT COALESCE(MAX(number), 0) + 1 FROM posts p WHERE p.tenant_id = $4), $3, $4, $5, $6, 0) 
			 RETURNING id`, c.Title, slug.Make(c.Title), c.Description, tenant.ID, user.ID, time.Now())
		if err != nil {
			return errors.Wrap(err, "failed add new post")
		}

		q := &query.GetPostByID{PostID: id}
		if err := getPostByID(ctx, q); err != nil {
			return err
		}
		c.Result = q.Result

		if err := internalAddSubscriber(trx, q.Result, tenant, user, false); err != nil {
			return err
		}

		return nil
	})
}

func updatePost(ctx context.Context, c *cmd.UpdatePost) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`UPDATE posts SET title = $1, slug = $2, description = $3 
													 WHERE id = $4 AND tenant_id = $5`, c.Title, slug.Make(c.Title), c.Description, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed update post")
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		sqlQuery := buildSinglePostQuery(tenant, user, "p.id = $2")
		post, err := querySinglePost(ctx, trx, sqlQuery, tenant.ID, q.PostID)
		if err != nil {
			return errors.Wrap(err, "failed to get post with id '%d'", q.PostID)
		}
		q.Result = post
		return nil
	})
}

func getPostBySlug(ctx context.Context, q *query.GetPostBySlug) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		sqlQuery := buildSinglePostQuery(tenant, user, "p.slug = $2")
		post, err := querySinglePost(ctx, trx, sqlQuery, tenant.ID, q.Slug)
		if err != nil {
			return errors.Wrap(err, "failed to get post with slug '%s'", q.Slug)
		}
		q.Result = post
		return nil
	})
}

func getPostByNumber(ctx context.Context, q *query.GetPostByNumber) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		sqlQuery := buildSinglePostQuery(tenant, user, "p.number = $2")
		post, err := querySinglePost(ctx, trx, sqlQuery, tenant.ID, q.Number)
		if err != nil {
			return errors.Wrap(err, "failed to get post with number '%d'", q.Number)
		}
		q.Result = post
		return nil
	})
}

func getUserPostCount(ctx context.Context, q *query.GetUserPostCount) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, currentUser *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, currentUser *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user != nil && user.IsHelper() && !user.IsCollaborator() && !user.IsModerator() && !user.IsAdministrator() {
			if q.Date == "" {
				q.Date = "7d"
			}
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
				sqlQuery += fmt.Sprintf(" AND p.created_at >= NOW() - INTERVAL '%d days'", days)
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		// Apply helper restrictions for untagged queries
		if user != nil && user.IsHelper() && !user.IsCollaborator() && !user.IsModerator() && !user.IsAdministrator() {
			if q.Untagged && q.Date == "" {
				q.Date = "7d"
			}
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		searchQuery := &query.SearchPosts{View: "all", Limit: "all"}
		if err := searchPosts(ctx, searchQuery); err != nil {
			return errors.Wrap(err, "failed to get all posts")
		}
		q.Result = searchQuery.Result
		return nil
	})
}

func getPostsByIDs(ctx context.Context, q *query.GetPostsByIDs) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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

		sqlQuery := buildPostsByIDsQuery(tenant, user)

		var posts []*dbPost
		err := trx.Select(&posts, sqlQuery, tenant.ID, pq.Array(statuses), pq.Array(q.PostIDs))
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

func querySinglePost(ctx context.Context, trx *dbx.Trx, query string, args ...any) (*entity.Post, error) {
	post := dbPost{}

	if err := trx.Get(&post, query, args...); err != nil {
		return nil, err
	}

	return post.toModel(ctx), nil
}

func lockPost(ctx context.Context, c *cmd.LockPost) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, _ *entity.Tenant, _ *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		conditions := []string{"p.tenant_id = $1", "p.status NOT IN ($2, $3)"}
		args := []interface{}{tenant.ID, int(enum.PostDeleted), int(enum.PostArchived)}
		argNum := 4

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

		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM posts p WHERE %s`, whereClause)
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
			FROM posts p
			WHERE %s
			ORDER BY p.last_activity_at ASC
			LIMIT $%d OFFSET $%d
		`, whereClause, argNum, argNum+1)
		args = append(args, q.PerPage, offset)

		hydration := buildHydration(tenant.ID, user, "top_posts", "", "", "ASC")

		selectQuery := fmt.Sprintf("WITH top_posts AS (%s) %s", cte, hydration)

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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		err := trx.Get(&q.Result, `
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
