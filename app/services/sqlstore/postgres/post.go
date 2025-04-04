package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

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
	ID             int            `db:"id"`
	Number         int            `db:"number"`
	Title          string         `db:"title"`
	Slug           string         `db:"slug"`
	Description    string         `db:"description"`
	CreatedAt      time.Time      `db:"created_at"`
	User           *dbUser        `db:"user"`
	VoteType       sql.NullInt32  `db:"vote_type"`
	VotesCount     int            `db:"votes_count"`
	CommentsCount  int            `db:"comments_count"`
	RecentVotes    int            `db:"recent_votes_count"`
	RecentComments int            `db:"recent_comments_count"`
	Upvotes        int            `db:"upvotes"`
	Downvotes      int            `db:"downvotes"`
	Status         int            `db:"status"`
	Response       sql.NullString `db:"response"`
	RespondedAt    dbx.NullTime   `db:"response_date"`
	ResponseUser   *dbUser        `db:"response_user"`
	OriginalNumber sql.NullInt64  `db:"original_number"`
	OriginalTitle  sql.NullString `db:"original_title"`
	OriginalSlug   sql.NullString `db:"original_slug"`
	OriginalStatus sql.NullInt64  `db:"original_status"`
	Tags           []string       `db:"tags"`
	LockedSettings sql.NullString `db:"locked_settings"`
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
		VoteType:       voteType,
		VotesCount:     i.VotesCount,
		CommentsCount:  i.CommentsCount,
		Status:         enum.PostStatus(i.Status),
		User:           i.User.toModel(ctx),
		Tags:           i.Tags,
		LockedSettings: nil,
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

	return post
}

var (
	sqlSelectPostsWhere = `	WITH 
													agg_tags AS ( 
														SELECT 
																post_id, 
																ARRAY_REMOVE(ARRAY_AGG(tags.slug), NULL) as tags
														FROM post_tags
														INNER JOIN tags
														ON tags.ID = post_tags.TAG_ID
														AND tags.tenant_id = post_tags.tenant_id
														WHERE post_tags.tenant_id = $1
														%s
														GROUP BY post_id 
													), 
													agg_comments AS (
															SELECT 
																	post_id, 
																	COUNT(CASE WHEN comments.created_at > CURRENT_DATE - INTERVAL '30 days' THEN 1 END) as recent,
																	COUNT(*) as all
															FROM comments 
															INNER JOIN posts
															ON posts.id = comments.post_id
															AND posts.tenant_id = comments.tenant_id
															WHERE posts.tenant_id = $1
															AND comments.deleted_at IS NULL
															GROUP BY post_id
													),
													agg_votes AS (
															SELECT 
																	post_id, 
																	SUM(CASE WHEN post_votes.created_at > CURRENT_DATE - INTERVAL '30 days' THEN vote_type ELSE 0 END) as recent,
																	SUM(vote_type) as all,
																	SUM(CASE WHEN vote_type > 0 THEN 1 ELSE 0 END) as upvotes,
																	SUM(CASE WHEN vote_type < 0 THEN 1 ELSE 0 END) as downvotes
															FROM post_votes 
															INNER JOIN posts
															ON posts.id = post_votes.post_id
															AND posts.tenant_id = post_votes.tenant_id
															WHERE posts.tenant_id = $1
															GROUP BY post_id
													)
													SELECT p.id, 
																p.number, 
																p.title, 
																p.slug, 
																p.description, 
																p.created_at,
																COALESCE(agg_s.all, 0) as votes_count,
																COALESCE(agg_c.all, 0) as comments_count,
																COALESCE(agg_s.recent, 0) AS recent_votes_count,
																COALESCE(agg_c.recent, 0) AS recent_comments_count,
																COALESCE(agg_s.upvotes, 0) AS upvotes,
																COALESCE(agg_s.downvotes, 0) AS downvotes,																
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
																%s AS vote_type
													FROM posts p
													INNER JOIN users u
													ON u.id = p.user_id
													AND u.tenant_id = $1
													LEFT JOIN users r
													ON r.id = p.response_user_id
													AND r.tenant_id = $1
													LEFT JOIN posts d
													ON d.id = p.original_id
													AND d.tenant_id = $1
													LEFT JOIN agg_comments agg_c
													ON agg_c.post_id = p.id
													LEFT JOIN agg_votes agg_s
													ON agg_s.post_id = p.id
													LEFT JOIN agg_tags agg_t 
													ON agg_t.post_id = p.id
													WHERE p.status != ` + strconv.Itoa(int(enum.PostDeleted)) + ` AND %s`
)

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
		SET response = '', original_id = $3, response_date = $4, response_user_id = $5, status = $6 
		WHERE id = $1 and tenant_id = $2
		`, c.Post.ID, tenant.ID, c.Original.ID, respondedAt, user.ID, enum.PostDuplicate)
		if err != nil {
			return errors.Wrap(err, "failed to update post's response")
		}

		c.Post.Status = enum.PostDuplicate
		c.Post.Response = &entity.PostResponse{
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
		post, err := querySinglePost(ctx, trx, buildPostQuery(user, "p.tenant_id = $1 AND p.id = $2"), tenant.ID, q.PostID)
		if err != nil {
			return errors.Wrap(err, "failed to get post with id '%d'", q.PostID)
		}
		q.Result = post
		return nil
	})
}

func getPostBySlug(ctx context.Context, q *query.GetPostBySlug) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		post, err := querySinglePost(ctx, trx, buildPostQuery(user, "p.tenant_id = $1 AND p.slug = $2"), tenant.ID, q.Slug)
		if err != nil {
			return errors.Wrap(err, "failed to get post with slug '%s'", q.Slug)
		}
		q.Result = post
		return nil
	})
}

func getPostByNumber(ctx context.Context, q *query.GetPostByNumber) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		post, err := querySinglePost(ctx, trx, buildPostQuery(user, "p.tenant_id = $1 AND p.number = $2"), tenant.ID, q.Number)
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

		// Example, ignoring deleted posts, i.e. status != PostDeleted (which is 5 in your enum)
		sqlQuery := `
            SELECT COUNT(*) 
            FROM posts
            WHERE tenant_id = $1
              AND user_id = $2
              AND status != 6
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

func searchPosts(ctx context.Context, q *query.SearchPosts) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		innerQuery := buildPostQuery(user, "p.tenant_id = $1 AND p.status = ANY($2)")
		if q.Untagged {
			innerQuery = fmt.Sprintf("%s AND NOT EXISTS (SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id)", innerQuery)
		}
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

		var (
			posts []*dbPost
			err   error
		)
		if q.Query != "" {
			scoreField := "ts_rank(setweight(to_tsvector(title), 'A') || setweight(to_tsvector(description), 'B'), to_tsquery('english', $3)) + similarity(title, $4) + similarity(description, $4)"
			sql := fmt.Sprintf(`
				SELECT * FROM (%s) AS q 
				WHERE %s > 0.1
				ORDER BY %s DESC
				LIMIT %s OFFSET %s
			`, innerQuery, scoreField, scoreField, q.Limit, q.Offset)
			statuses := []enum.PostStatus{
				enum.PostOpen,
				enum.PostStarted,
				enum.PostPlanned,
				enum.PostCompleted,
				enum.PostDeclined,
			}
			if q.View == "make-post" {
				statuses = append(statuses, enum.PostDuplicate, enum.PostDeclined)
			}
			err = trx.Select(&posts, sql, tenant.ID, pq.Array(statuses), ToTSQuery(q.Query), SanitizeString(q.Query))
		} else {
			userID := 0
			if user != nil {
				userID = user.ID
			}

			condition, statuses, sort, extraParams := getViewData(*q, userID)
			sql := fmt.Sprintf(`
				SELECT * FROM (%s) AS q 
				WHERE 1 = 1 %s
				ORDER BY %s DESC
				LIMIT %s OFFSET %s
			`, innerQuery, condition, sort, q.Limit, q.Offset)
			params := []interface{}{tenant.ID, pq.Array(statuses)}
			params = append(params, extraParams...)
			err = trx.Select(&posts, sql, params...)
		}

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

func querySinglePost(ctx context.Context, trx *dbx.Trx, query string, args ...any) (*entity.Post, error) {
	post := dbPost{}

	if err := trx.Get(&post, query, args...); err != nil {
		return nil, err
	}

	return post.toModel(ctx), nil
}

func buildPostQuery(user *entity.User, filter string) string {
	tagCondition := `AND tags.is_public = true`
	if user != nil && (user.IsCollaborator() || user.IsModerator()) {
		tagCondition = ``
	}
	voteTypeSubQuery := "NULL"
	if user != nil {
		voteTypeSubQuery = fmt.Sprintf("(SELECT vote_type FROM post_votes WHERE post_id = p.id AND user_id = %d LIMIT 1)", user.ID)
	}
	return fmt.Sprintf(sqlSelectPostsWhere, tagCondition, voteTypeSubQuery, filter)
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
