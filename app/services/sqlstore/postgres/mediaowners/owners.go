package mediaowners

const ReferenceIntroducers = "/%&\\"

type Parent struct {
	Kind string
	ID   string
}

type Owner struct {
	Kind     string
	Table    string
	Tenant   string
	ID       string
	From     string
	Title    string
	URL      string
	Scope    string
	Keys     []string
	Markup   []string
	Extra    []string
	Parents  []Parent
	Unlink   string
}

var All = []Owner{
	{
		Kind: "tenant", Table: "tenants", Tenant: "id", ID: "id",
		From: "tenants o", Title: "o.name", URL: "'/admin/general'",
		Keys: []string{"logo_bkey"},
		Markup: []string{"welcome_message", "message_banner", "custom_css"},
		Unlink: `UPDATE tenants SET logo_bkey='' WHERE id=$1 AND logo_bkey=$2 AND $3`,
	},
	{
		Kind: "post", Table: "posts", Tenant: "tenant_id", ID: "id",
		From: "posts o", Title: "o.title", URL: "'/posts/' || o.number || '/' || o.slug",
		Scope: "CASE WHEN o.status=6 THEN 'deleted' ELSE 'active' END",
		Markup: []string{"description", "response", "locked_settings"},
	},
	{
		Kind: "page", Table: "pages", Tenant: "tenant_id", ID: "id",
		From: "pages o", Title: "o.title", URL: "'/pages/' || o.slug",
		Scope: "CASE WHEN o.status='draft' THEN 'draft' ELSE 'active' END",
		Keys: []string{"banner_image_bkey", "og_image_bkey"}, Markup: []string{"content", "excerpt"},
		Unlink: `UPDATE pages SET
			banner_image_bkey=CASE WHEN banner_image_bkey=$2 THEN '' ELSE banner_image_bkey END,
			og_image_bkey=CASE WHEN og_image_bkey=$2 THEN NULL ELSE og_image_bkey END
			WHERE tenant_id=$1 AND (banner_image_bkey=$2 OR og_image_bkey=$2)
			  AND NOT media_reference_blocks(CASE WHEN status='draft' THEN 'draft' ELSE 'active' END, $3, $4, $5)`,
	},
	{
		Kind: "user", Table: "users", Tenant: "tenant_id", ID: "id",
		From: "users o", Title: "o.name", URL: "'/profile/' || o.id",
		Keys: []string{"avatar_bkey"},
		Unlink: `UPDATE users SET avatar_bkey='', avatar_type=1 WHERE tenant_id=$1 AND avatar_bkey=$2 AND $3`,
	},
	{
		Kind: "comment", Table: "comments", Tenant: "tenant_id", ID: "id",
		From: `comments o
			LEFT JOIN posts p ON p.id=o.post_id AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=o.page_id AND page.tenant_id=o.tenant_id`,
		Title: "'Comment #' || o.id",
		URL: `CASE WHEN o.page_id IS NOT NULL THEN '/pages/' || page.slug
			ELSE '/posts/' || p.number || '/' || p.slug END || '#comment-' || o.id`,
		Scope: `CASE WHEN o.deleted_at IS NOT NULL OR p.status=6 THEN 'deleted'
			WHEN page.status='draft' THEN 'draft' ELSE 'active' END`,
		Markup: []string{"content"},
		Parents: []Parent{{Kind: "post", ID: "p.id"}, {Kind: "page", ID: "page.id"}},
	},
	{
		Kind: "moderation", Table: "moderation_checks", Tenant: "tenant_id", ID: "content_id",
		From: "moderation_checks o",
		Title: "'Awaiting moderation: ' || o.content_type || ' #' || o.content_id", URL: "'/admin/moderation'",
		Extra: []string{"content_type", "state", "text_content", "blob_keys", "fallback_profile"},
		Parents: []Parent{
			{Kind: "post", ID: "CASE WHEN o.content_type='post' THEN o.content_id END"},
			{Kind: "comment", ID: "CASE WHEN o.content_type='comment' THEN o.content_id END"},
			{Kind: "user", ID: "CASE WHEN o.content_type='avatar' THEN o.content_id END"},
		},
		Unlink: `UPDATE moderation_checks
			SET fallback_profile=jsonb_build_object('avatar_type', 1, 'avatar_bkey', '')
			WHERE tenant_id=$1 AND fallback_profile->>'avatar_bkey'=$2 AND $3;
			UPDATE moderation_checks SET blob_keys=blob_keys-$2, revision=revision+1,
				state=CASE WHEN content_type='avatar' THEN 'canceled' ELSE 'pending' END,
				fallback_profile=CASE WHEN content_type='avatar' THEN NULL ELSE fallback_profile END,
				attempts=0, next_attempt_at=NOW(), last_error='', updated_at=NOW()
			WHERE tenant_id=$1 AND state IN ('pending', 'running', 'failed') AND blob_keys ? $2 AND $3`,
	},
	{
		Kind: "attachment", Table: "attachments", Tenant: "tenant_id", ID: "id",
		From: `attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id`,
		Title: "CASE WHEN o.comment_id IS NOT NULL THEN 'Comment #' || o.comment_id ELSE p.title END",
		URL: `CASE WHEN c.page_id IS NOT NULL THEN '/pages/' || page.slug
			ELSE '/posts/' || p.number || '/' || p.slug END ||
			CASE WHEN o.comment_id IS NOT NULL THEN '#comment-' || o.comment_id ELSE '' END`,
		Scope: `CASE WHEN c.deleted_at IS NOT NULL OR p.status=6 THEN 'deleted'
			WHEN page.status='draft' THEN 'draft' ELSE 'active' END`,
		Keys: []string{"attachment_bkey"},
		Parents: []Parent{{Kind: "post", ID: "p.id"}, {Kind: "page", ID: "page.id"}, {Kind: "comment", ID: "c.id"}},
		Unlink: `DELETE FROM attachments a USING media_references r
			WHERE a.tenant_id=$1 AND a.attachment_bkey=$2
			  AND r.tenant_id=a.tenant_id AND r.kind='attachment' AND r.id=a.id AND r.key=a.attachment_bkey
			  AND NOT media_reference_blocks(r.scope,$3,$4,$5)`,
	},
	{
		Kind: "draft", Table: "page_drafts", Tenant: "tenant_id", ID: "id",
		From: "page_drafts o", Title: "COALESCE(o.title,'Page draft')", URL: "'/admin/pages/edit/' || o.page_id",
		Scope: "'draft'::text", Keys: []string{"banner_image_bkey"}, Markup: []string{"content", "excerpt"},
		Parents: []Parent{{Kind: "page", ID: "o.page_id"}},
		Unlink: `UPDATE page_drafts SET banner_image_bkey='', updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND banner_image_bkey=$2 AND NOT shared
			  AND NOT media_reference_blocks('draft',$3,$4,$5)`,
	},
	{
		Kind: "oauth", Table: "oauth_providers", Tenant: "tenant_id", ID: "id",
		From: "oauth_providers o", Title: "o.display_name", URL: "'/admin/authentication'",
		Keys: []string{"logo_bkey"},
		Unlink: `UPDATE oauth_providers SET logo_bkey='' WHERE tenant_id=$1 AND logo_bkey=$2 AND $3`,
	},
	{
		Kind: "sponsor", Table: "sponsor_creatives", Tenant: "tenant_id", ID: "id",
		From: "sponsor_creatives o", Title: "COALESCE(NULLIF(o.data->>'headline', ''), 'Sponsor creative #' || o.id)", URL: "'/admin/sponsorship'",
		Scope: "CASE WHEN o.data->>'state'='approved' THEN 'active' ELSE 'draft' END",
		Keys: []string{"image_key", "logo_key"},
		Unlink: `UPDATE sponsor_creatives SET
			image_key=CASE WHEN image_key=$2 THEN '' ELSE image_key END,
			logo_key=CASE WHEN logo_key=$2 THEN '' ELSE logo_key END,
			data=jsonb_set(data,'{state}','"review"'), revision=revision+1
			WHERE tenant_id=$1 AND (image_key=$2 OR logo_key=$2)
			  AND NOT media_reference_blocks(CASE WHEN data->>'state'='approved' THEN 'active' ELSE 'draft' END,$3,$4,$5)`,
	},
	{
		Kind: "creative", Table: "creative_versions", Tenant: "tenant_id", ID: "id",
		From: "creative_versions o", Title: "'Creative version #' || o.id", URL: "'/admin/sponsorship'",
		Markup: []string{"image_url", "html"},
	},
	{
		Kind: "response", Table: "canned_responses", Tenant: "tenant_id", ID: "id",
		From: "canned_responses o", Title: "o.title", URL: "'/admin/responses'",
		Markup: []string{"content"},
	},
	{
		Kind: "navigation", Table: "navigation_links", Tenant: "tenant_id", ID: "id",
		From: "navigation_links o", Title: "o.title", URL: "'/admin/content'",
		Markup: []string{"url"},
	},
	{
		Kind: "webhook", Table: "webhooks", Tenant: "tenant_id", ID: "id",
		From: "webhooks o", Title: "o.name", URL: "'/admin/webhooks'",
		Markup: []string{"url", "content"},
	},
}
