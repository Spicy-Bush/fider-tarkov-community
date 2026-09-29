package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pagedoc"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pages"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
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
	AllowCommentImages bool           `db:"allow_comment_images"`
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
		ID:                 p.ID,
		Title:              p.Title,
		Slug:               p.Slug,
		Content:            p.Content,
		Status:             entity.PageStatus(p.Status),
		Visibility:         entity.PageVisibility(p.Visibility),
		AllowComments:      p.AllowComments,
		AllowCommentImages: p.AllowCommentImages,
		AllowReactions:     p.AllowReactions,
		ShowTOC:            p.ShowTOC,
		CreatedAt:          p.CreatedAt,
		UpdatedAt:          p.UpdatedAt,
		CreatedBy:          p.CreatedBy.toModel(ctx),
		UpdatedBy:          p.UpdatedBy.toModel(ctx),
		CommentsCount:      p.CommentsCount,
		EmbeddedPosts:      []*entity.Post{},
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
	if p.CachedAt.Valid {
		page.CachedAt = &p.CachedAt.Time
	}

	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	page.Permissions = page.AllowedActions(user, tenant)

	return page
}

func getPageBySlug(ctx context.Context, q *query.GetPageBySlug) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		page := &dbPage{}
		err := trx.Get(page, `
			SELECT p.id, p.title, p.slug, p.content, p.excerpt, p.banner_image_bkey,
				p.status, p.visibility, p.allowed_roles, p.parent_page_id,
				p.allow_comments, p.allow_comment_images, p.allow_reactions, p.show_toc,
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
		return loadPageRelations(ctx, trx, user, q.Result, page.CachedEmbeddedData.String)
	})
}

func getPageByID(ctx context.Context, q *query.GetPageByID) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		page := &dbPage{}
		err := trx.Get(page, `
			SELECT p.id, p.title, p.slug, p.content, p.excerpt, p.banner_image_bkey,
				p.status, p.visibility, p.allowed_roles, p.parent_page_id,
				p.allow_comments, p.allow_comment_images, p.allow_reactions, p.show_toc,
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
		return loadPageRelations(ctx, trx, user, q.Result, page.CachedEmbeddedData.String)
	})
}

func loadPageRelations(ctx context.Context, trx *dbx.Trx, user *entity.User, page *entity.Page, cachedEmbeddedData string) error {
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	cached, err := pages.UnmarshalCachedData(cachedEmbeddedData)
	if err != nil {
		return err
	}
	if cached != nil && len(cached.PostIDs) > 0 {
		statuses := []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
			enum.PostCompleted,
			enum.PostDeclined,
			enum.PostDuplicate,
			enum.PostArchived,
		}
		selection := buildPostsByIDsQuery(tenant, user, statuses, cached.PostIDs)
		var records []*dbPost
		if err := trx.Select(&records, selection.SQL, selection.Params...); err != nil {
			return errors.Wrap(err, "failed to load embedded posts")
		}

		postsByID := make(map[int]*entity.Post, len(records))
		for _, record := range records {
			postsByID[record.ID] = record.toModel(ctx)
		}
		for _, id := range cached.PostIDs {
			if post, visible := postsByID[id]; visible {
				page.EmbeddedPosts = append(page.EmbeddedPosts, post)
			}
		}
	}

	var dbAuthors []*dbUser
	err = trx.Select(&dbAuthors, `
		SELECT u.id, u.name, u.email, u.role, u.status, u.avatar_type, u.avatar_bkey
		FROM users u
		INNER JOIN page_authors pa ON pa.user_id = u.id AND pa.tenant_id = u.tenant_id
		WHERE pa.page_id = $1 AND pa.tenant_id = $2
		ORDER BY pa.display_order
	`, page.ID, tenant.ID)
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
		SELECT pt.id, pt.name, pt.slug,
			COALESCE(pt.description, '') AS description, COALESCE(pt.color, '') AS color
		FROM page_topics pt
		INNER JOIN page_topics_map ptm ON ptm.topic_id = pt.id AND ptm.tenant_id = pt.tenant_id
		WHERE ptm.page_id = $1 AND ptm.tenant_id = $2
	`, page.ID, tenant.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get page topics")
	}
	page.Topics = topics

	tags := []*entity.PageTag{}
	err = trx.Select(&tags, `
		SELECT t.id, t.name, t.slug
		FROM page_tags t
		INNER JOIN page_tags_map tm ON tm.tag_id = t.id AND tm.tenant_id = t.tenant_id
		WHERE tm.page_id = $1 AND tm.tenant_id = $2
	`, page.ID, tenant.ID)
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
	if err := prepareImages(ctx, []*dto.ImageUpload{c.BannerImage}, "pages"); err != nil {
		return err
	}
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := validatePageParent(trx, tenant.ID, c.ParentPageID); err != nil {
			return err
		}
		if err := claimPageBanner(ctx, c.BannerImage, ""); err != nil {
			return err
		}
		slug, err := pages.GenerateSlug(ctx, c.Title, c.Slug, 0)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		if c.BannerImage != nil && !c.BannerImage.Remove && c.BannerImage.Upload != nil {
			if err := bus.Dispatch(ctx, &cmd.UploadImage{
				Image:  c.BannerImage,
				Folder: "pages",
			}); err != nil {
				return err
			}
		}

		var bannerBKey string
		if c.BannerImage != nil && !c.BannerImage.Remove {
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
			meta_description, canonical_url, cached_embedded_data, cached_at, allow_comment_images
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
		) RETURNING id
	`, tenant.ID, c.Title, slug, c.Content, c.Excerpt,
			bannerBKey, c.Status, c.Visibility, allowedRolesJSON,
			c.ParentPageID, c.AllowComments, c.AllowReactions, c.ShowTOC,
			c.ScheduledFor, getPublishedAt(c.Status),
			time.Now(), time.Now(), user.ID, user.ID,
			c.MetaDescription, canonicalURL, cachedJSON, time.Now(), c.AllowCommentImages)

		if err != nil {
			return errors.Wrap(err, "failed to create page")
		}

		authors := c.Authors
		if len(authors) == 0 {
			authors = []int{user.ID}
		}
		if err := setPageAuthors(trx, tenant.ID, id, authors); err != nil {
			return err
		}
		if err := setPageTopics(trx, tenant.ID, id, c.Topics); err != nil {
			return err
		}
		if err := setPageTags(trx, tenant.ID, id, c.Tags); err != nil {
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
	if err := prepareImages(ctx, []*dto.ImageUpload{c.BannerImage}, "pages"); err != nil {
		return err
	}
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := validatePageParent(trx, tenant.ID, c.ParentPageID); err != nil {
			return err
		}
		var currentBanner string
		if err := trx.Scalar(&currentBanner, `
			SELECT COALESCE(banner_image_bkey, '') FROM pages
			WHERE tenant_id=$1 AND id=$2 FOR UPDATE
		`, tenant.ID, c.PageID); err != nil {
			return err
		}
		if err := claimPageBanner(ctx, c.BannerImage, currentBanner); err != nil {
			return err
		}
		slug, err := pages.GenerateSlug(ctx, c.Title, c.Slug, c.PageID)
		if err != nil {
			return errors.Wrap(err, "failed to generate slug")
		}

		if c.BannerImage != nil && !c.BannerImage.Remove && c.BannerImage.Upload != nil {
			if err := bus.Dispatch(ctx, &cmd.UploadImage{
				Image:  c.BannerImage,
				Folder: "pages",
			}); err != nil {
				return err
			}
		}

		var bannerBKey string
		if c.BannerImage != nil && !c.BannerImage.Remove {
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
			meta_description = $17, canonical_url = $18, cached_embedded_data = $19, cached_at = $20,
			allow_comment_images = $23
		WHERE id = $21 AND tenant_id = $22
	`, c.Title, slug, c.Content, c.Excerpt, bannerBKey,
			c.Status, c.Visibility, allowedRolesJSON, c.ParentPageID,
			c.AllowComments, c.AllowReactions, c.ShowTOC,
			c.ScheduledFor, getPublishedAt(c.Status),
			time.Now(), user.ID, c.MetaDescription, canonicalURL,
			cachedJSON, time.Now(), c.PageID, tenant.ID, c.AllowCommentImages)

		if err != nil {
			return errors.Wrap(err, "failed to update page")
		}

		authors := c.Authors
		if len(authors) == 0 {
			authors = []int{user.ID}
		}
		if err := setPageAuthors(trx, tenant.ID, c.PageID, authors); err != nil {
			return err
		}
		if err := setPageTopics(trx, tenant.ID, c.PageID, c.Topics); err != nil {
			return err
		}
		if err := setPageTags(trx, tenant.ID, c.PageID, c.Tags); err != nil {
			return err
		}

		page := &query.GetPageByID{ID: c.PageID}
		if err := getPageByID(ctx, page); err != nil {
			return err
		}
		if _, err := trx.Execute(`
			DELETE FROM page_drafts WHERE tenant_id=$1 AND page_id=$2 AND user_id=$3 AND NOT shared
		`, tenant.ID, c.PageID, user.ID); err != nil {
			return err
		}
		c.Result = page.Result
		return nil
	})
}

func claimPageBanner(ctx context.Context, banner *dto.ImageUpload, currentKey string) error {
	if banner == nil || banner.Remove || banner.Upload != nil || banner.BlobKey == "" || banner.BlobKey == currentKey {
		return nil
	}

	claim := &query.CanUseStoredImage{Key: banner.BlobKey, MaxKilobytes: 5000}
	if err := canUseStoredImage(ctx, claim); err != nil {
		return err
	}
	if !claim.Result {
		return validate.Failed("The stored banner is unavailable or belongs to another account.")
	}
	return nil
}

func deletePage(ctx context.Context, c *cmd.DeletePage) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("DELETE FROM pages WHERE id = $1 AND tenant_id = $2", c.PageID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete page")
		}
		return nil
	})
}

func pageRelationFailure(field string) error {
	result := validate.Success()
	result.AddFieldFailure(field, "One or more selected items do not exist")
	return result
}

func validatePageParent(trx *dbx.Trx, tenantID int, parentID *int) error {
	if parentID == nil {
		return nil
	}

	var id int
	err := trx.Scalar(&id, "SELECT id FROM pages WHERE tenant_id = $1 AND id = $2 FOR KEY SHARE", tenantID, *parentID)
	if errors.Cause(err) == app.ErrNotFound {
		return pageRelationFailure("parentPageId")
	}
	return err
}

func setPageAuthors(trx *dbx.Trx, tenantID, pageID int, authorIDs []int) error {
	_, err := trx.Execute("DELETE FROM page_authors WHERE tenant_id = $1 AND page_id = $2", tenantID, pageID)
	if err != nil {
		return errors.Wrap(err, "failed to delete page authors")
	}

	count, err := trx.Execute(`
		INSERT INTO page_authors (tenant_id, page_id, user_id, display_order)
		SELECT $1, $2, author.id, selected.position - 1
		FROM unnest($3::int[]) WITH ORDINALITY AS selected(id, position)
		JOIN users author ON author.id = selected.id AND author.tenant_id = $1
	`, tenantID, pageID, pq.Array(authorIDs))
	if err != nil {
		return errors.Wrap(err, "failed to insert page authors")
	}
	if count != int64(len(authorIDs)) {
		return pageRelationFailure("authors")
	}

	return nil
}

func setPageTopics(trx *dbx.Trx, tenantID, pageID int, topicIDs []int) error {
	_, err := trx.Execute("DELETE FROM page_topics_map WHERE tenant_id = $1 AND page_id = $2", tenantID, pageID)
	if err != nil {
		return errors.Wrap(err, "failed to delete page topics")
	}

	count, err := trx.Execute(`
		INSERT INTO page_topics_map (tenant_id, page_id, topic_id)
		SELECT $1, $2, id FROM page_topics WHERE tenant_id = $1 AND id = ANY($3)
	`, tenantID, pageID, pq.Array(topicIDs))
	if err != nil {
		return errors.Wrap(err, "failed to insert page topics")
	}
	if count != int64(len(topicIDs)) {
		return pageRelationFailure("topics")
	}

	return nil
}

func setPageTags(trx *dbx.Trx, tenantID, pageID int, tagIDs []int) error {
	_, err := trx.Execute("DELETE FROM page_tags_map WHERE tenant_id = $1 AND page_id = $2", tenantID, pageID)
	if err != nil {
		return errors.Wrap(err, "failed to delete page tags")
	}

	count, err := trx.Execute(`
		INSERT INTO page_tags_map (tenant_id, page_id, tag_id)
		SELECT $1, $2, id FROM page_tags WHERE tenant_id = $1 AND id = ANY($3)
	`, tenantID, pageID, pq.Array(tagIDs))
	if err != nil {
		return errors.Wrap(err, "failed to insert page tags")
	}
	if count != int64(len(tagIDs)) {
		return pageRelationFailure("tags")
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		conditions := []string{"p.tenant_id = $1"}
		args := []interface{}{tenant.ID}
		argCount := 1

		if q.Query != "" {
			argCount++
			conditions = append(conditions, fmt.Sprintf(`(
				COALESCE(edit.title, p.title) ILIKE $%d OR COALESCE(edit.content, p.content) ILIKE $%d OR
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

		const pageSource = `pages p LEFT JOIN page_drafts edit
			ON edit.tenant_id=p.tenant_id AND edit.page_id=p.id AND edit.shared AND p.status='draft'`
		err := trx.Get(&q.TotalCount, "SELECT COUNT(*) FROM "+pageSource+" WHERE "+whereClause, args...)
		if err != nil {
			return errors.Wrap(err, "failed to count pages")
		}

		orderBy := "p.created_at DESC"
		switch q.View {
		case "oldest":
			orderBy = "p.created_at ASC"
		case "updated":
			orderBy = "COALESCE(edit.updated_at, p.updated_at) DESC"
		case "alphabetical":
			orderBy = "COALESCE(NULLIF(edit.title, ''), p.title) ASC"
		}

		argCount++
		limitArg := argCount
		argCount++
		offsetArg := argCount
		args = append(args, q.Limit, q.Offset)

		pages := []*dbPage{}
		err = trx.Select(&pages, fmt.Sprintf(`
			SELECT p.id, COALESCE(NULLIF(edit.title, ''), p.title) AS title, p.slug,
				COALESCE(edit.excerpt, p.excerpt) AS excerpt,
				COALESCE(edit.banner_image_bkey, p.banner_image_bkey) AS banner_image_bkey,
				p.status, p.visibility, p.allowed_roles, p.allow_reactions,
				p.published_at, p.created_at, COALESCE(edit.updated_at, p.updated_at) AS updated_at,
				cb.id AS created_by_id, cb.name AS created_by_name,
				(SELECT COUNT(*) FROM comments WHERE page_id = p.id AND deleted_at IS NULL) as comments_count
			FROM %s
			INNER JOIN users cb ON cb.id = p.created_by_id
			WHERE %s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, pageSource, whereClause, orderBy, limitArg, offsetArg), args...)

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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		owner := &query.GetDiscussion{PageID: c.Page.ID, LockOwner: true}
		if err := getDiscussion(ctx, owner); err != nil {
			return err
		}

		if !owner.Result.Permissions(user, tenant).React {
			return validate.Unauthorized()
		}

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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		owner := &query.GetDiscussion{PageID: c.PageID, LockOwner: true}
		if err := getDiscussion(ctx, owner); err != nil {
			return err
		}
		if !owner.Result.CanSubscribeToPage(user, tenant) {
			return validate.Unauthorized()
		}

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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		page := &query.GetPageByID{ID: q.PageID}
		if err := getPageByID(ctx, page); err != nil {
			return err
		}

		users := []*dbUser{}
		err := trx.Select(&users, `
			SELECT u.id, u.name, u.email, u.role, u.status
			FROM users u
			INNER JOIN page_subscriptions ps ON ps.user_id = u.id
			WHERE ps.page_id = $1 AND u.tenant_id = $2 AND u.status = $3
		`, q.PageID, tenant.ID, enum.UserActive)

		if err != nil {
			return errors.Wrap(err, "failed to get page subscribers")
		}

		q.Result = make([]*entity.User, 0, len(users))
		for _, candidate := range users {
			recipient := candidate.toModel(ctx)
			if page.Result.CanView(recipient, tenant) {
				q.Result = append(q.Result, recipient)
			}
		}

		return nil
	})
}

func getPageDraft(ctx context.Context, q *query.GetPageDraft) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil || q.UserID != user.ID || !entity.Can(user, tenant, entity.ManagePages) {
			return validate.Unauthorized()
		}
		var saved struct {
			ID              int       `db:"id"`
			Title           string    `db:"title"`
			Slug            string    `db:"slug"`
			Content         string    `db:"content"`
			Excerpt         string    `db:"excerpt"`
			BannerImageBKey string    `db:"banner_image_bkey"`
			MetaDescription string    `db:"meta_description"`
			ShowTOC         bool      `db:"show_toc"`
			UpdatedAt       time.Time `db:"updated_at"`
		}
		err := trx.Get(&saved, `
			SELECT id, title, slug, content, COALESCE(excerpt, '') AS excerpt,
			       COALESCE(banner_image_bkey, '') AS banner_image_bkey,
			       COALESCE(meta_description, '') AS meta_description, show_toc, updated_at
			FROM page_drafts
			WHERE tenant_id = $1 AND page_id = $2 AND user_id = $3 AND NOT shared
		`, tenant.ID, q.PageID, user.ID)
		if errors.Cause(err) == app.ErrNotFound {
			q.Result = nil
			return nil
		}
		if err != nil {
			return err
		}

		q.Result = &entity.PageDraft{
			ID:              saved.ID,
			PageID:          q.PageID,
			UserID:          user.ID,
			Title:           saved.Title,
			Slug:            saved.Slug,
			Content:         saved.Content,
			Excerpt:         saved.Excerpt,
			BannerImageBKey: saved.BannerImageBKey,
			MetaDescription: saved.MetaDescription,
			ShowTOC:         saved.ShowTOC,
			UpdatedAt:       saved.UpdatedAt,
		}
		return nil
	})
}

func publishScheduledPages(ctx context.Context, c *cmd.PublishScheduledPages) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var published []*struct {
			ID           int       `db:"id"`
			TenantID     int       `db:"tenant_id"`
			ScheduledFor time.Time `db:"scheduled_for"`
		}
		err := trx.Select(&published, `
			UPDATE pages SET status='published', published_at=NOW()
			WHERE status='scheduled' AND scheduled_for<=NOW()
			RETURNING id, tenant_id, scheduled_for
		`)
		if err != nil {
			return errors.Wrap(err, "failed to publish scheduled pages")
		}

		for _, publishedPage := range published {
			saved, err := readPageEdit(trx, publishedPage.TenantID, publishedPage.ID)
			if errors.Cause(err) == app.ErrNotFound {
				continue
			}
			if err != nil {
				return err
			}
			page, err := pagedoc.Read(saved.State)
			if err != nil {
				return err
			}
			if page.Status != entity.PageStatusScheduled || page.ScheduledFor == nil || !page.ScheduledFor.Equal(publishedPage.ScheduledFor) {
				continue
			}

			state, err := pagedoc.SetStatus(saved.State, entity.PageStatusPublished)
			if err != nil {
				return err
			}
			page.Status = entity.PageStatusPublished
			if _, err := savePageEdit(trx, publishedPage.TenantID, publishedPage.ID, state, page); err != nil {
				return err
			}
		}
		c.Result = len(published)
		return nil
	})
}

func getAllPublishedPages(ctx context.Context, q *query.GetAllPublishedPages) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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

func refreshPageEmbeddedData(ctx context.Context, c *cmd.RefreshPageEmbeddedData) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
