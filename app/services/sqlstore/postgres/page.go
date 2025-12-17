package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pages"
	"github.com/lib/pq"
)

type dbPage struct {
	ID                 int            `db:"id"`
	Title              string         `db:"title"`
	Slug               string         `db:"slug"`
	Content            string         `db:"content"`
	Excerpt            dbx.NullString `db:"excerpt"`
	BannerImageBKey    dbx.NullString `db:"banner_image_bkey"`
	Status             string         `db:"status"`
	Visibility         string         `db:"visibility"`
	AllowedRoles       dbx.NullString `db:"allowed_roles"`
	ParentPageID       dbx.NullInt    `db:"parent_page_id"`
	AllowComments      bool           `db:"allow_comments"`
	AllowReactions     bool           `db:"allow_reactions"`
	ShowTOC            bool           `db:"show_toc"`
	ScheduledFor       dbx.NullTime   `db:"scheduled_for"`
	PublishedAt        dbx.NullTime   `db:"published_at"`
	CreatedAt          time.Time      `db:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"`
	CreatedBy          *dbUser        `db:"created_by"`
	UpdatedBy          *dbUser        `db:"updated_by"`
	MetaDescription    dbx.NullString `db:"meta_description"`
	CanonicalURL       dbx.NullString `db:"canonical_url"`
	CommentsCount      int            `db:"comments_count"`
	CachedEmbeddedData dbx.NullString `db:"cached_embedded_data"`
	CachedAt           dbx.NullTime   `db:"cached_at"`
}

func (p *dbPage) toModel(ctx context.Context) *entity.Page {
	page := &entity.Page{
		ID:             p.ID,
		Title:          p.Title,
		Slug:           p.Slug,
		Content:        p.Content,
		Status:         entity.PageStatus(p.Status),
		Visibility:     entity.PageVisibility(p.Visibility),
		AllowComments:  p.AllowComments,
		AllowReactions: p.AllowReactions,
		ShowTOC:        p.ShowTOC,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		CreatedBy:      p.CreatedBy.toModel(ctx),
		UpdatedBy:      p.UpdatedBy.toModel(ctx),
		CommentsCount:  p.CommentsCount,
		EmbeddedPosts:  []*entity.Post{},
	}

	if p.Excerpt.Valid {
		page.Excerpt = p.Excerpt.String
	}
	if p.BannerImageBKey.Valid {
		page.BannerImageBKey = p.BannerImageBKey.String
	}
	if p.AllowedRoles.Valid {
		_ = json.Unmarshal([]byte(p.AllowedRoles.String), &page.AllowedRoles)
	}
	if p.ParentPageID.Valid {
		pid := int(p.ParentPageID.Int64)
		page.ParentPageID = &pid
	}
	if p.ScheduledFor.Valid {
		page.ScheduledFor = &p.ScheduledFor.Time
	}
	if p.PublishedAt.Valid {
		page.PublishedAt = &p.PublishedAt.Time
	}
	if p.MetaDescription.Valid {
		page.MetaDescription = p.MetaDescription.String
	}
	if p.CanonicalURL.Valid {
		page.CanonicalURL = p.CanonicalURL.String
	}
	if p.CachedEmbeddedData.Valid && p.CachedEmbeddedData.String != "" {
		cachedData, err := pages.UnmarshalCachedData(p.CachedEmbeddedData.String)
		if err == nil && cachedData != nil {
			page.EmbeddedPosts = cachedData.Posts
		}
	}
	if p.CachedAt.Valid {
		page.CachedAt = &p.CachedAt.Time
	}

	return page
}

func getPageBySlug(ctx context.Context, q *query.GetPageBySlug) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		page := &dbPage{}
		err := trx.Get(page, `
			SELECT p.id, p.title, p.slug, p.content, p.excerpt, p.banner_image_bkey,
				p.status, p.visibility, p.allowed_roles, p.parent_page_id,
				p.allow_comments, p.allow_reactions, p.show_toc,
				p.cached_embedded_data, p.cached_at,
				p.scheduled_for, p.published_at, p.created_at, p.updated_at,
				p.meta_description, p.canonical_url,
				cb.id AS created_by_id, cb.name AS created_by_name, cb.email AS created_by_email,
				cb.role AS created_by_role, cb.status AS created_by_status,
				cb.avatar_type AS created_by_avatar_type, cb.avatar_bkey AS created_by_avatar_bkey,
				ub.id AS updated_by_id, ub.name AS updated_by_name, ub.email AS updated_by_email,
				ub.role AS updated_by_role, ub.status AS updated_by_status,
				ub.avatar_type AS updated_by_avatar_type, ub.avatar_bkey AS updated_by_avatar_bkey,
				(SELECT COUNT(*) FROM comments WHERE page_id = p.id AND deleted_at IS NULL) as comments_count
			FROM pages p
			INNER JOIN users cb ON cb.id = p.created_by_id
			INNER JOIN users ub ON ub.id = p.updated_by_id
			WHERE p.tenant_id = $1 AND p.slug = $2
		`, tenant.ID, q.Slug)

		if err != nil {
			return errors.Wrap(err, "failed to get page by slug")
		}

		q.Result = page.toModel(ctx)
		return loadPageRelations(ctx, trx, user, q.Result)
	})
}

func getPageByID(ctx context.Context, q *query.GetPageByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		page := &dbPage{}
		err := trx.Get(page, `
			SELECT p.id, p.title, p.slug, p.content, p.excerpt, p.banner_image_bkey,
				p.status, p.visibility, p.allowed_roles, p.parent_page_id,
				p.allow_comments, p.allow_reactions, p.show_toc,
				p.cached_embedded_data, p.cached_at,
				p.scheduled_for, p.published_at, p.created_at, p.updated_at,
				p.meta_description, p.canonical_url,
				cb.id AS created_by_id, cb.name AS created_by_name, cb.email AS created_by_email,
				cb.role AS created_by_role, cb.status AS created_by_status,
				cb.avatar_type AS created_by_avatar_type, cb.avatar_bkey AS created_by_avatar_bkey,
				ub.id AS updated_by_id, ub.name AS updated_by_name, ub.email AS updated_by_email,
				ub.role AS updated_by_role, ub.status AS updated_by_status,
				ub.avatar_type AS updated_by_avatar_type, ub.avatar_bkey AS updated_by_avatar_bkey,
				(SELECT COUNT(*) FROM comments WHERE page_id = p.id AND deleted_at IS NULL) as comments_count
			FROM pages p
			INNER JOIN users cb ON cb.id = p.created_by_id
			INNER JOIN users ub ON ub.id = p.updated_by_id
			WHERE p.tenant_id = $1 AND p.id = $2
		`, tenant.ID, q.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get page by id")
		}

		q.Result = page.toModel(ctx)
		return loadPageRelations(ctx, trx, user, q.Result)
	})
}

func loadPageRelations(ctx context.Context, trx *dbx.Trx, user *entity.User, page *entity.Page) error {
	var dbAuthors []*dbUser
	err := trx.Select(&dbAuthors, `
		SELECT u.id, u.name, u.email, u.role, u.status, u.avatar_type, u.avatar_bkey
		FROM users u
		INNER JOIN page_authors pa ON pa.user_id = u.id
		WHERE pa.page_id = $1
		ORDER BY pa.display_order
	`, page.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get page authors")
	}
	authors := make([]*entity.User, len(dbAuthors))
	for i, a := range dbAuthors {
		authors[i] = a.toModel(ctx)
	}
	page.Authors = authors

	topics := []*entity.PageTopic{}
	err = trx.Select(&topics, `
		SELECT pt.id, pt.name, pt.slug, pt.description, pt.color
		FROM page_topics pt
		INNER JOIN page_topics_map ptm ON ptm.topic_id = pt.id
		WHERE ptm.page_id = $1
	`, page.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get page topics")
	}
	page.Topics = topics

	tags := []*entity.PageTag{}
	err = trx.Select(&tags, `
		SELECT t.id, t.name, t.slug
		FROM page_tags t
		INNER JOIN page_tags_map tm ON tm.tag_id = t.id
		WHERE tm.page_id = $1
	`, page.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get page tags")
	}
	page.Tags = tags

	if page.AllowReactions {
		type reactionCount struct {
			Emoji      string `db:"emoji"`
			Count      int    `db:"count"`
			IncludesMe bool   `db:"includes_me"`
		}
		var reactions []*reactionCount

		userIDParam := 0
		if user != nil {
			userIDParam = user.ID
		}

		err = trx.Select(&reactions, `
			SELECT 
				emoji,
				COUNT(*) as count,
				BOOL_OR(user_id = $2) as includes_me
			FROM page_reactions
			WHERE page_id = $1
			GROUP BY emoji
			ORDER BY count DESC
		`, page.ID, userIDParam)

		if err == nil {
			page.ReactionCounts = make([]entity.ReactionCounts, len(reactions))
			for i, r := range reactions {
				page.ReactionCounts[i] = entity.ReactionCounts{
					Emoji:      r.Emoji,
					Count:      r.Count,
					IncludesMe: r.IncludesMe,
				}
			}
		}
	}

	return nil
}

func createPage(ctx context.Context, c *cmd.CreatePage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		slug, err := pages.GenerateSlug(ctx, c.Title, c.Slug, 0)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		if c.BannerImage != nil && c.BannerImage.Upload != nil {
			if err := bus.Dispatch(ctx, &cmd.UploadImage{
				Image:  c.BannerImage,
				Folder: "pages",
			}); err != nil {
				return err
			}
		}

		var bannerBKey string
		if c.BannerImage != nil {
			bannerBKey = c.BannerImage.BlobKey
		}

		var allowedRolesJSON interface{}
		if len(c.AllowedRoles) > 0 {
			b, _ := json.Marshal(c.AllowedRoles)
			allowedRolesJSON = string(b)
		}

		cachedData, _ := pages.RefreshEmbeddedData(ctx, c.Content)
		var cachedJSON interface{}
		if cachedData != nil {
			b, _ := pages.MarshalCachedData(cachedData)
			if b != "" {
				cachedJSON = b
			}
		}

		canonicalURL := "/pages/" + slug

		var id int
		err = trx.Get(&id, `
		INSERT INTO pages (
			tenant_id, title, slug, content, excerpt, banner_image_bkey,
			status, visibility, allowed_roles, parent_page_id,
			allow_comments, allow_reactions, show_toc, scheduled_for, published_at,
			created_at, updated_at, created_by_id, updated_by_id,
			meta_description, canonical_url, cached_embedded_data, cached_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
		) RETURNING id
	`, tenant.ID, c.Title, slug, c.Content, c.Excerpt,
			bannerBKey, c.Status, c.Visibility, allowedRolesJSON,
			c.ParentPageID, c.AllowComments, c.AllowReactions, c.ShowTOC,
			c.ScheduledFor, getPublishedAt(c.Status),
			time.Now(), time.Now(), user.ID, user.ID,
			c.MetaDescription, canonicalURL, cachedJSON, time.Now())

		if err != nil {
			return errors.Wrap(err, "failed to create page")
		}

		authors := c.Authors
		if len(authors) == 0 {
			authors = []int{user.ID}
		}
		if err := setPageAuthors(ctx, trx, id, authors); err != nil {
			return err
		}
		if err := setPageTopics(ctx, trx, id, c.Topics); err != nil {
			return err
		}
		if err := setPageTags(ctx, trx, id, c.Tags); err != nil {
			return err
		}

		q := &query.GetPageByID{ID: id}
		if err := getPageByID(ctx, q); err != nil {
			return err
		}
		c.Result = q.Result

		return nil
	})
}

func updatePage(ctx context.Context, c *cmd.UpdatePage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		slug, err := pages.GenerateSlug(ctx, c.Title, c.Slug, c.PageID)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		if c.BannerImage != nil && c.BannerImage.Upload != nil {
			if err := bus.Dispatch(ctx, &cmd.UploadImage{
				Image:  c.BannerImage,
				Folder: "pages",
			}); err != nil {
				return err
			}
		}

		var bannerBKey string
		if c.BannerImage != nil {
			bannerBKey = c.BannerImage.BlobKey
		}

		var allowedRolesJSON interface{}
		if len(c.AllowedRoles) > 0 {
			b, _ := json.Marshal(c.AllowedRoles)
			allowedRolesJSON = string(b)
		}

		cachedData, _ := pages.RefreshEmbeddedData(ctx, c.Content)
		var cachedJSON interface{}
		if cachedData != nil {
			b, _ := pages.MarshalCachedData(cachedData)
			if b != "" {
				cachedJSON = b
			}
		}

		canonicalURL := "/pages/" + slug

		_, err = trx.Execute(`
		UPDATE pages SET
			title = $1, slug = $2, content = $3, excerpt = $4, banner_image_bkey = $5,
			status = $6, visibility = $7, allowed_roles = $8, parent_page_id = $9,
			allow_comments = $10, allow_reactions = $11, show_toc = $12,
			scheduled_for = $13, published_at = $14, updated_at = $15, updated_by_id = $16,
			meta_description = $17, canonical_url = $18, cached_embedded_data = $19, cached_at = $20
		WHERE id = $21 AND tenant_id = $22
	`, c.Title, slug, c.Content, c.Excerpt, bannerBKey,
			c.Status, c.Visibility, allowedRolesJSON, c.ParentPageID,
			c.AllowComments, c.AllowReactions, c.ShowTOC,
			c.ScheduledFor, getPublishedAt(c.Status),
			time.Now(), user.ID, c.MetaDescription, canonicalURL,
			cachedJSON, time.Now(), c.PageID, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to update page")
		}

		authors := c.Authors
		if len(authors) == 0 {
			authors = []int{user.ID}
		}
		if err := setPageAuthors(ctx, trx, c.PageID, authors); err != nil {
			return err
		}
		if err := setPageTopics(ctx, trx, c.PageID, c.Topics); err != nil {
			return err
		}
		if err := setPageTags(ctx, trx, c.PageID, c.Tags); err != nil {
			return err
		}

		return nil
	})
}

func deletePage(ctx context.Context, c *cmd.DeletePage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("DELETE FROM pages WHERE id = $1 AND tenant_id = $2", c.PageID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete page")
		}
		return nil
	})
}

func setPageAuthors(_ context.Context, trx *dbx.Trx, pageID int, authorIDs []int) error {
	_, err := trx.Execute("DELETE FROM page_authors WHERE page_id = $1", pageID)
	if err != nil {
		return errors.Wrap(err, "failed to delete page authors")
	}

	for i, authorID := range authorIDs {
		_, err = trx.Execute(`
			INSERT INTO page_authors (page_id, user_id, display_order)
			VALUES ($1, $2, $3)
		`, pageID, authorID, i)
		if err != nil {
			return errors.Wrap(err, "failed to insert page author")
		}
	}

	return nil
}

func setPageTopics(_ context.Context, trx *dbx.Trx, pageID int, topicIDs []int) error {
	_, err := trx.Execute("DELETE FROM page_topics_map WHERE page_id = $1", pageID)
	if err != nil {
		return errors.Wrap(err, "failed to delete page topics")
	}

	for _, topicID := range topicIDs {
		_, err = trx.Execute(`
			INSERT INTO page_topics_map (page_id, topic_id)
			VALUES ($1, $2)
		`, pageID, topicID)
		if err != nil {
			return errors.Wrap(err, "failed to insert page topic")
		}
	}

	return nil
}

func setPageTags(_ context.Context, trx *dbx.Trx, pageID int, tagIDs []int) error {
	_, err := trx.Execute("DELETE FROM page_tags_map WHERE page_id = $1", pageID)
	if err != nil {
		return errors.Wrap(err, "failed to delete page tags")
	}

	for _, tagID := range tagIDs {
		_, err = trx.Execute(`
			INSERT INTO page_tags_map (page_id, tag_id)
			VALUES ($1, $2)
		`, pageID, tagID)
		if err != nil {
			return errors.Wrap(err, "failed to insert page tag")
		}
	}

	return nil
}

func getPublishedAt(status entity.PageStatus) *time.Time {
	if status == entity.PageStatusPublished {
		now := time.Now()
		return &now
	}
	return nil
}

func listPages(ctx context.Context, q *query.ListPages) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		conditions := []string{"p.tenant_id = $1"}
		args := []interface{}{tenant.ID}
		argCount := 1

		if q.Query != "" {
			argCount++
			conditions = append(conditions, fmt.Sprintf(`(
				p.title ILIKE $%d OR p.content ILIKE $%d OR 
				EXISTS (
					SELECT 1 FROM page_topics_map ptm
					INNER JOIN page_topics pt ON pt.id = ptm.topic_id
					WHERE ptm.page_id = p.id AND pt.name ILIKE $%d
				)
			)`, argCount, argCount, argCount))
			args = append(args, "%"+q.Query+"%")
		}

		if len(q.Status) > 0 {
			argCount++
			statuses := make([]string, len(q.Status))
			for i, s := range q.Status {
				statuses[i] = string(s)
			}
			conditions = append(conditions, fmt.Sprintf("p.status = ANY($%d)", argCount))
			args = append(args, pq.Array(statuses))
		} else {
			conditions = append(conditions, "p.status = 'published'")
			conditions = append(conditions, "p.visibility = 'public'")
		}

		if len(q.Topics) > 0 {
			argCount++
			conditions = append(conditions, fmt.Sprintf(`
				EXISTS (
					SELECT 1 FROM page_topics_map ptm
					INNER JOIN page_topics pt ON pt.id = ptm.topic_id
					WHERE ptm.page_id = p.id AND pt.slug = ANY($%d)
				)
			`, argCount))
			args = append(args, pq.Array(q.Topics))
		}

		if len(q.Tags) > 0 {
			argCount++
			conditions = append(conditions, fmt.Sprintf(`
				EXISTS (
					SELECT 1 FROM page_tags_map tm
					INNER JOIN page_tags t ON t.id = tm.tag_id
					WHERE tm.page_id = p.id AND t.slug = ANY($%d)
				)
			`, argCount))
			args = append(args, pq.Array(q.Tags))
		}

		whereClause := strings.Join(conditions, " AND ")

		err := trx.Get(&q.TotalCount, "SELECT COUNT(*) FROM pages p WHERE "+whereClause, args...)
		if err != nil {
			return errors.Wrap(err, "failed to count pages")
		}

		orderBy := "p.created_at DESC"
		switch q.View {
		case "oldest":
			orderBy = "p.created_at ASC"
		case "updated":
			orderBy = "p.updated_at DESC"
		case "alphabetical":
			orderBy = "p.title ASC"
		}

		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		args = append(args, q.Limit, q.Offset)

		pages := []*dbPage{}
		err = trx.Select(&pages, fmt.Sprintf(`
			SELECT p.id, p.title, p.slug, p.excerpt, p.banner_image_bkey,
				p.status, p.visibility, p.published_at, p.created_at, p.updated_at,
				cb.id AS created_by_id, cb.name AS created_by_name,
				(SELECT COUNT(*) FROM comments WHERE page_id = p.id AND deleted_at IS NULL) as comments_count
			FROM pages p
			INNER JOIN users cb ON cb.id = p.created_by_id
			WHERE %s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, whereClause, orderBy, limitArg, offsetArg), args...)

		if err != nil {
			return errors.Wrap(err, "failed to list pages")
		}

		q.Result = make([]*entity.Page, len(pages))
		for i, p := range pages {
			q.Result[i] = p.toModel(ctx)
		}

		return nil
	})
}

func togglePageReaction(ctx context.Context, c *cmd.TogglePageReaction) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var added bool
		err := trx.Scalar(&added, `
			WITH toggle_reaction AS (
				INSERT INTO page_reactions (page_id, user_id, emoji, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (page_id, user_id, emoji) DO NOTHING
				RETURNING true AS added
			),
			delete_existing AS (
				DELETE FROM page_reactions
				WHERE page_id = $1 AND user_id = $2 AND emoji = $3
				AND NOT EXISTS (SELECT 1 FROM toggle_reaction)
				RETURNING false AS added
			)
			SELECT COALESCE(
				(SELECT added FROM toggle_reaction),
				(SELECT added FROM delete_existing),
				false
			)
		`, c.Page.ID, user.ID, c.Emoji, time.Now())

		if err != nil {
			return errors.Wrap(err, "failed to toggle page reaction")
		}

		c.Result = added
		return nil
	})
}

func togglePageSubscription(ctx context.Context, c *cmd.TogglePageSubscription) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var subscribed bool
		err := trx.Scalar(&subscribed, `
			WITH toggle_sub AS (
				INSERT INTO page_subscriptions (page_id, user_id, created_at)
				VALUES ($1, $2, $3)
				ON CONFLICT (page_id, user_id) DO NOTHING
				RETURNING true AS subscribed
			),
			delete_existing AS (
				DELETE FROM page_subscriptions
				WHERE page_id = $1 AND user_id = $2
				AND NOT EXISTS (SELECT 1 FROM toggle_sub)
				RETURNING false AS subscribed
			)
			SELECT COALESCE(
				(SELECT subscribed FROM toggle_sub),
				(SELECT subscribed FROM delete_existing),
				false
			)
		`, c.PageID, user.ID, time.Now())

		if err != nil {
			return errors.Wrap(err, "failed to toggle page subscription")
		}

		c.Result = subscribed
		return nil
	})
}

func userSubscribedToPage(ctx context.Context, q *query.UserSubscribedToPage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			q.Result = false
			return nil
		}

		exists, err := trx.Exists("SELECT 1 FROM page_subscriptions WHERE page_id = $1 AND user_id = $2", q.PageID, user.ID)
		if err != nil {
			return errors.Wrap(err, "failed to check page subscription")
		}

		q.Result = exists
		return nil
	})
}

func getPageSubscribers(ctx context.Context, q *query.GetPageSubscribers) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		users := []*dbUser{}
		err := trx.Select(&users, `
			SELECT u.id, u.name, u.email, u.role, u.status
			FROM users u
			INNER JOIN page_subscriptions ps ON ps.user_id = u.id
			WHERE ps.page_id = $1
		`, q.PageID)

		if err != nil {
			return errors.Wrap(err, "failed to get page subscribers")
		}

		q.Result = make([]*entity.User, len(users))
		for i, u := range users {
			q.Result[i] = u.toModel(ctx)
		}

		return nil
	})
}

func savePageDraft(ctx context.Context, c *cmd.SavePageDraft) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		draftDataJSON, _ := json.Marshal(c.DraftData)

		_, err := trx.Execute(`
			INSERT INTO page_drafts (
				page_id, tenant_id, user_id, title, slug, content, excerpt,
				banner_image_bkey, meta_description, show_toc, draft_data, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (page_id, user_id) DO UPDATE SET
				title = EXCLUDED.title,
				slug = EXCLUDED.slug,
				content = EXCLUDED.content,
				excerpt = EXCLUDED.excerpt,
				banner_image_bkey = EXCLUDED.banner_image_bkey,
				meta_description = EXCLUDED.meta_description,
				show_toc = EXCLUDED.show_toc,
				draft_data = EXCLUDED.draft_data,
				updated_at = EXCLUDED.updated_at
		`, c.PageID, tenant.ID, user.ID, c.Title, c.Slug, c.Content, c.Excerpt,
			c.BannerImageBKey, c.MetaDescription, c.ShowTOC, draftDataJSON, time.Now())

		return errors.Wrap(err, "failed to save page draft")
	})
}

func getPageDraft(ctx context.Context, q *query.GetPageDraft) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		draft := struct {
			ID              int            `db:"id"`
			PageID          int            `db:"page_id"`
			UserID          int            `db:"user_id"`
			Title           string         `db:"title"`
			Slug            string         `db:"slug"`
			Content         string         `db:"content"`
			Excerpt         dbx.NullString `db:"excerpt"`
			BannerImageBKey dbx.NullString `db:"banner_image_bkey"`
			MetaDescription dbx.NullString `db:"meta_description"`
			ShowTOC         bool           `db:"show_toc"`
			DraftData       dbx.NullString `db:"draft_data"`
			UpdatedAt       time.Time      `db:"updated_at"`
		}{}

		err := trx.Get(&draft, `
			SELECT id, page_id, user_id, title, slug, content, excerpt,
				banner_image_bkey, meta_description, show_toc, draft_data, updated_at
			FROM page_drafts
			WHERE page_id = $1 AND user_id = $2
		`, q.PageID, q.UserID)

		if err == sql.ErrNoRows {
			q.Result = nil
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "failed to get page draft")
		}

		q.Result = &entity.PageDraft{
			ID:        draft.ID,
			PageID:    draft.PageID,
			UserID:    draft.UserID,
			Title:     draft.Title,
			Slug:      draft.Slug,
			Content:   draft.Content,
			ShowTOC:   draft.ShowTOC,
			UpdatedAt: draft.UpdatedAt,
		}

		if draft.Excerpt.Valid {
			q.Result.Excerpt = draft.Excerpt.String
		}
		if draft.BannerImageBKey.Valid {
			q.Result.BannerImageBKey = draft.BannerImageBKey.String
		}
		if draft.MetaDescription.Valid {
			q.Result.MetaDescription = draft.MetaDescription.String
		}
		if draft.DraftData.Valid {
			q.Result.DraftData = draft.DraftData.String
		}

		return nil
	})
}

func publishScheduledPages(ctx context.Context, c *cmd.PublishScheduledPages) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var count int
		err := trx.Get(&count, `
			WITH updated AS (
				UPDATE pages
				SET status = 'published', published_at = NOW()
				WHERE status = 'scheduled'
				AND scheduled_for <= NOW()
				RETURNING id
			)
			SELECT COUNT(*) FROM updated
		`)

		if err != nil {
			return errors.Wrap(err, "failed to publish scheduled pages")
		}

		c.Result = count
		return nil
	})
}

func getAllPublishedPages(ctx context.Context, q *query.GetAllPublishedPages) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		pages := []*dbPage{}
		err := trx.Select(&pages, `
			SELECT id, title, slug, visibility, updated_at
			FROM pages
			WHERE tenant_id = $1 AND status = 'published' AND visibility = 'public'
			ORDER BY updated_at DESC
		`, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get all published pages")
		}

		q.Result = make([]*entity.Page, len(pages))
		for i, p := range pages {
			q.Result[i] = &entity.Page{
				ID:         p.ID,
				Title:      p.Title,
				Slug:       p.Slug,
				Visibility: entity.PageVisibility(p.Visibility),
				UpdatedAt:  p.UpdatedAt,
			}
		}

		return nil
	})
}

func getCommentsByPage(ctx context.Context, q *query.GetCommentsByPage) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]*entity.Comment, 0)

		comments := []*dbComment{}
		userID := 0
		if user != nil {
			userID = user.ID
		}

		err := trx.Select(&comments, `
			WITH agg_reactions AS (
				SELECT 
					comment_id,
					json_agg(json_build_object(
						'emoji', emoji,
						'count', count,
						'includesMe', CASE WHEN $3 = ANY(user_ids) THEN true ELSE false END
					) ORDER BY count DESC) as reaction_counts
				FROM (
					SELECT 
						comment_id, 
						emoji, 
						COUNT(*) as count,
						array_agg(user_id) as user_ids
					FROM reactions
					WHERE comment_id IN (SELECT id FROM comments WHERE page_id = $1)
					GROUP BY comment_id, emoji
				) r
				GROUP BY comment_id
			)
			SELECT c.id, c.content, c.created_at, c.edited_at,
				u.id AS user_id, u.name AS user_name, u.email AS user_email,
				u.role AS user_role, u.status AS user_status,
				u.avatar_type AS user_avatar_type, u.avatar_bkey AS user_avatar_bkey,
				e.id AS edited_by_id, e.name AS edited_by_name, e.email AS edited_by_email,
				e.role AS edited_by_role, e.status AS edited_by_status,
				ar.reaction_counts,
				c.moderation_pending
			FROM comments c
			INNER JOIN users u ON u.id = c.user_id
			LEFT JOIN users e ON e.id = c.edited_by_id
			LEFT JOIN agg_reactions ar ON ar.comment_id = c.id
			WHERE c.page_id = $1 AND c.tenant_id = $2
			AND c.deleted_at IS NULL
			ORDER BY c.created_at ASC
		`, q.Page.ID, tenant.ID, userID)

		if err != nil {
			return errors.Wrap(err, "failed to get comments by page")
		}

		for _, c := range comments {
			q.Result = append(q.Result, c.toModel(ctx))
		}

		return nil
	})
}

func addPageComment(ctx context.Context, c *cmd.AddPageComment) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var id int
		if err := trx.Get(&id, `
			INSERT INTO comments (tenant_id, page_id, content, user_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, tenant.ID, c.Page.ID, c.Content, user.ID, time.Now()); err != nil {
			return errors.Wrap(err, "failed to add page comment")
		}

		q := &query.GetCommentByID{CommentID: id}
		if err := getCommentByID(ctx, q); err != nil {
			return err
		}
		c.Result = q.Result

		return nil
	})
}

func refreshPageEmbeddedData(ctx context.Context, c *cmd.RefreshPageEmbeddedData) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var content string
		if err := trx.Get(&content, `
			SELECT content FROM pages WHERE id = $1 AND tenant_id = $2
		`, c.PageID, tenant.ID); err != nil {
			return errors.Wrap(err, "failed to get page content")
		}

		cachedData, err := pages.RefreshEmbeddedData(ctx, content)
		if err != nil {
			return errors.Wrap(err, "failed to refresh embedded data")
		}

		cachedJSON, err := pages.MarshalCachedData(cachedData)
		if err != nil {
			return errors.Wrap(err, "failed to marshal cached data")
		}

		_, err = trx.Execute(`
			UPDATE pages SET cached_embedded_data = $1, cached_at = $2
			WHERE id = $3 AND tenant_id = $4
		`, cachedJSON, time.Now(), c.PageID, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to update cached data")
		}

		return nil
	})
}
