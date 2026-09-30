CREATE TABLE sponsor_campaigns (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    id serial NOT NULL,
    revision integer NOT NULL DEFAULT 1,
    state text NOT NULL,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    confirm_by timestamptz,
    data jsonb NOT NULL,
    PRIMARY KEY (tenant_id, id),
    CHECK (jsonb_typeof(data) = 'object')
);

CREATE INDEX sponsor_campaigns_delivery ON sponsor_campaigns(tenant_id, end_at)
WHERE state='booked';

CREATE TABLE sponsor_creatives (
    tenant_id integer NOT NULL,
    id serial NOT NULL,
    campaign_id integer NOT NULL,
    revision integer NOT NULL DEFAULT 1,
    image_key text NOT NULL DEFAULT '',
    logo_key text NOT NULL DEFAULT '',
    data jsonb NOT NULL,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, campaign_id) REFERENCES sponsor_campaigns(tenant_id, id),
    CHECK (jsonb_typeof(data) = 'object')
);

CREATE INDEX sponsor_creatives_campaign ON sponsor_creatives(tenant_id, campaign_id);

CREATE TABLE sponsor_placement_settings (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    placement_id text NOT NULL,
    enabled boolean NOT NULL,
    position text NOT NULL,
    every integer NOT NULL CHECK (every BETWEEN 0 AND 100),
    empty text NOT NULL CHECK (empty IN ('none', 'kofi', 'adsense')),
    adsense_slot_id text NOT NULL DEFAULT '',
    CHECK (empty <> 'adsense' OR adsense_slot_id ~ '^[0-9]{1,20}$'),
    PRIMARY KEY (tenant_id, placement_id)
);

CREATE TABLE sponsor_allocation_groups (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    day date NOT NULL,
    placement_id text NOT NULL,
    eligibility text NOT NULL,
    next_slot bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, day, placement_id, eligibility)
);

CREATE TABLE sponsor_allocations (
    tenant_id integer NOT NULL,
    day date NOT NULL,
    campaign_id integer NOT NULL,
    placement_id text NOT NULL,
    creative_id integer NOT NULL,
    eligible bigint NOT NULL DEFAULT 0,
    allocated bigint NOT NULL DEFAULT 0,
    expected bigint NOT NULL DEFAULT 0,
    clicks bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, day, campaign_id, placement_id, creative_id),
    FOREIGN KEY (tenant_id, campaign_id) REFERENCES sponsor_campaigns(tenant_id, id),
    FOREIGN KEY (tenant_id, creative_id) REFERENCES sponsor_creatives(tenant_id, id)
);

CREATE TABLE sponsor_changes (
    tenant_id integer NOT NULL,
    campaign_id integer NOT NULL,
    revision integer NOT NULL,
    state text NOT NULL,
    at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, campaign_id, revision),
    FOREIGN KEY (tenant_id, campaign_id) REFERENCES sponsor_campaigns(tenant_id, id)
);

CREATE TABLE sponsor_exclusions (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    page_type text NOT NULL CHECK (page_type IN ('post', 'page')),
    content_id integer NOT NULL,
    reason text NOT NULL,
    PRIMARY KEY (tenant_id, page_type, content_id)
);
CREATE OR REPLACE VIEW media_reference_sources AS
SELECT o."id" AS tenant_id, 'tenant'::text AS kind, o."id" AS owner_id,
    o.name AS title, '/admin/general' AS url,
    jsonb_build_object('logo_bkey', o."logo_bkey", 'welcome_message', o."welcome_message", 'message_banner', o."message_banner", 'custom_css', o."custom_css") AS source
FROM tenants o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'post'::text AS kind, o."id" AS owner_id,
    o.title AS title, '/posts/' || o.number || '/' || o.slug AS url,
    jsonb_build_object('description', o."description", 'response', o."response", 'locked_settings', o."locked_settings") AS source
FROM posts o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'page'::text AS kind, o."id" AS owner_id,
    o.title AS title, '/pages/' || o.slug AS url,
    jsonb_build_object('banner_image_bkey', o."banner_image_bkey", 'og_image_bkey', o."og_image_bkey", 'content', o."content", 'excerpt', o."excerpt") AS source
FROM pages o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'user'::text AS kind, o."id" AS owner_id,
    o.name AS title, '/profile/' || o.id AS url,
    jsonb_build_object('avatar_bkey', o."avatar_bkey") AS source
FROM users o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'comment'::text AS kind, o."id" AS owner_id,
    'Comment #' || o.id AS title, CASE WHEN o.page_id IS NOT NULL THEN '/pages/' || page.slug
			ELSE '/posts/' || p.number || '/' || p.slug END || '#comment-' || o.id AS url,
    jsonb_build_object('content', o."content") AS source
FROM comments o
			LEFT JOIN posts p ON p.id=o.post_id AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=o.page_id AND page.tenant_id=o.tenant_id
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'moderation:' || o.content_type AS kind, o."content_id" AS owner_id,
    'Awaiting moderation: ' || o.content_type || ' #' || o.content_id AS title, '/admin/moderation' AS url,
    jsonb_build_object('content_type', o."content_type", 'state', o."state", 'text_content', o."text_content", 'blob_keys', o."blob_keys", 'fallback_profile', o."fallback_profile") AS source
FROM moderation_checks o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'attachment'::text AS kind, o."id" AS owner_id,
    CASE WHEN o.comment_id IS NOT NULL THEN 'Comment #' || o.comment_id ELSE p.title END AS title, CASE WHEN c.page_id IS NOT NULL THEN '/pages/' || page.slug
			ELSE '/posts/' || p.number || '/' || p.slug END ||
			CASE WHEN o.comment_id IS NOT NULL THEN '#comment-' || o.comment_id ELSE '' END AS url,
    jsonb_build_object('attachment_bkey', o."attachment_bkey") AS source
FROM attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'draft'::text AS kind, o."id" AS owner_id,
    COALESCE(o.title,'Page draft') AS title, '/admin/pages/edit/' || o.page_id AS url,
    jsonb_build_object('banner_image_bkey', o."banner_image_bkey", 'content', o."content", 'excerpt', o."excerpt") AS source
FROM page_drafts o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'oauth'::text AS kind, o."id" AS owner_id,
    o.display_name AS title, '/admin/authentication' AS url,
    jsonb_build_object('logo_bkey', o."logo_bkey") AS source
FROM oauth_providers o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'sponsor'::text AS kind, o."id" AS owner_id,
    COALESCE(NULLIF(o.data->>'headline', ''), 'Sponsor creative #' || o.id) AS title, '/admin/sponsorship' AS url,
    jsonb_build_object('image_key', o."image_key", 'logo_key', o."logo_key") AS source
FROM sponsor_creatives o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'creative'::text AS kind, o."id" AS owner_id,
    'Creative version #' || o.id AS title, '/admin/sponsorship' AS url,
    jsonb_build_object('image_url', o."image_url", 'html', o."html") AS source
FROM creative_versions o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'response'::text AS kind, o."id" AS owner_id,
    o.title AS title, '/admin/responses' AS url,
    jsonb_build_object('content', o."content") AS source
FROM canned_responses o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'navigation'::text AS kind, o."id" AS owner_id,
    o.title AS title, '/admin/content' AS url,
    jsonb_build_object('url', o."url") AS source
FROM navigation_links o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'webhook'::text AS kind, o."id" AS owner_id,
    o.name AS title, '/admin/webhooks' AS url,
    jsonb_build_object('url', o."url", 'content', o."content") AS source
FROM webhooks o;

CREATE OR REPLACE VIEW media_reference_scopes AS
SELECT o."tenant_id" AS tenant_id, 'post'::text AS kind, o."id" AS owner_id, CASE WHEN o.status=6 THEN 'deleted' ELSE 'active' END AS scope
FROM posts o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'page'::text AS kind, o."id" AS owner_id, CASE WHEN o.status='draft' THEN 'draft' ELSE 'active' END AS scope
FROM pages o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'comment'::text AS kind, o."id" AS owner_id, CASE WHEN o.deleted_at IS NOT NULL OR p.status=6 THEN 'deleted'
			WHEN page.status='draft' THEN 'draft' ELSE 'active' END AS scope
FROM comments o
			LEFT JOIN posts p ON p.id=o.post_id AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=o.page_id AND page.tenant_id=o.tenant_id
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'attachment'::text AS kind, o."id" AS owner_id, CASE WHEN c.deleted_at IS NOT NULL OR p.status=6 THEN 'deleted'
			WHEN page.status='draft' THEN 'draft' ELSE 'active' END AS scope
FROM attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'draft'::text AS kind, o."id" AS owner_id, 'draft'::text AS scope
FROM page_drafts o
UNION ALL
SELECT o."tenant_id" AS tenant_id, 'sponsor'::text AS kind, o."id" AS owner_id, CASE WHEN o.data->>'state'='approved' THEN 'active' ELSE 'draft' END AS scope
FROM sponsor_creatives o;

CREATE INDEX comments_concealed_posts ON comments (tenant_id, post_id, parent_id, id)
    INCLUDE (user_id, deleted_at, moderation_pending)
    WHERE post_id IS NOT NULL AND (deleted_at IS NOT NULL OR moderation_pending);

CREATE INDEX comments_concealed_pages ON comments (tenant_id, page_id, parent_id, id)
    INCLUDE (user_id, deleted_at, moderation_pending)
    WHERE page_id IS NOT NULL AND (deleted_at IS NOT NULL OR moderation_pending);

CREATE OR REPLACE FUNCTION "capture_media_sponsor"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='sponsor'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='sponsor'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."image_key" IS NOT DISTINCT FROM NEW."image_key" AND OLD."logo_key" IS NOT DISTINCT FROM NEW."logo_key" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."image_key"::text, '') <> '' OR COALESCE(NEW."logo_key"::text, '') <> '') THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'sponsor'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'sponsor'::text,NEW."id",jsonb_build_object('image_key', NEW."image_key", 'logo_key', NEW."logo_key"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS "media_sponsor_references" ON "sponsor_creatives";
CREATE TRIGGER "media_sponsor_references" AFTER INSERT OR DELETE OR UPDATE OF "image_key", "logo_key" ON "sponsor_creatives"
FOR EACH ROW EXECUTE FUNCTION "capture_media_sponsor"();

CREATE OR REPLACE FUNCTION lock_media_reference_owners(requested_tenant integer, requested_key text)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    PERFORM 1 FROM "tenants" WHERE "id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM tenants o
        JOIN media_asset_refs r ON r.tenant_id=o."id" AND r.kind='tenant'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "posts" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM posts o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='post'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT p.id FROM comments o
			LEFT JOIN posts p ON p.id=o.post_id AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=o.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='comment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT CASE WHEN o.content_type='post' THEN o.content_id END FROM moderation_checks o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='moderation:' || o.content_type AND r.owner_id=o."content_id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT p.id FROM attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='attachment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "pages" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM pages o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='page'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT page.id FROM comments o
			LEFT JOIN posts p ON p.id=o.post_id AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=o.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='comment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT page.id FROM attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='attachment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT o.page_id FROM page_drafts o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='draft'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "users" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM users o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='user'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT CASE WHEN o.content_type='avatar' THEN o.content_id END FROM moderation_checks o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='moderation:' || o.content_type AND r.owner_id=o."content_id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "comments" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM comments o
			LEFT JOIN posts p ON p.id=o.post_id AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=o.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='comment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT CASE WHEN o.content_type='comment' THEN o.content_id END FROM moderation_checks o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='moderation:' || o.content_type AND r.owner_id=o."content_id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
        UNION
        SELECT c.id FROM attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='attachment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "moderation_checks" WHERE "tenant_id"=requested_tenant AND (content_type, content_id) IN (
        SELECT o.content_type, o."content_id" FROM moderation_checks o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='moderation:' || o.content_type AND r.owner_id=o."content_id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY (content_type, content_id) FOR UPDATE;

    PERFORM 1 FROM "attachments" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM attachments o
			LEFT JOIN comments c ON c.id=o.comment_id AND c.tenant_id=o.tenant_id
			LEFT JOIN posts p ON p.id=COALESCE(o.post_id,c.post_id) AND p.tenant_id=o.tenant_id
			LEFT JOIN pages page ON page.id=c.page_id AND page.tenant_id=o.tenant_id
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='attachment'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "page_drafts" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM page_drafts o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='draft'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "oauth_providers" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM oauth_providers o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='oauth'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "sponsor_creatives" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM sponsor_creatives o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='sponsor'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "creative_versions" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM creative_versions o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='creative'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "canned_responses" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM canned_responses o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='response'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "navigation_links" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM navigation_links o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='navigation'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

    PERFORM 1 FROM "webhooks" WHERE "tenant_id"=requested_tenant AND "id" IN (
        SELECT o."id" FROM webhooks o
        JOIN media_asset_refs r ON r.tenant_id=o."tenant_id" AND r.kind='webhook'::text AND r.owner_id=o."id"
        WHERE r.tenant_id=requested_tenant AND r.key=requested_key
    ) ORDER BY "id" FOR UPDATE;

END;
$$;

CREATE OR REPLACE FUNCTION unlink_media_reference_fields(integer, text, boolean, boolean, boolean)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    UPDATE tenants SET logo_bkey='' WHERE id=$1 AND logo_bkey=$2 AND $3;

    UPDATE pages SET
			banner_image_bkey=CASE WHEN banner_image_bkey=$2 THEN '' ELSE banner_image_bkey END,
			og_image_bkey=CASE WHEN og_image_bkey=$2 THEN NULL ELSE og_image_bkey END
			WHERE tenant_id=$1 AND (banner_image_bkey=$2 OR og_image_bkey=$2)
			  AND NOT media_reference_blocks(CASE WHEN status='draft' THEN 'draft' ELSE 'active' END, $3, $4, $5);

    UPDATE users SET avatar_bkey='', avatar_type=1 WHERE tenant_id=$1 AND avatar_bkey=$2 AND $3;

    UPDATE moderation_checks
			SET fallback_profile=jsonb_build_object('avatar_type', 1, 'avatar_bkey', '')
			WHERE tenant_id=$1 AND fallback_profile->>'avatar_bkey'=$2 AND $3;
			UPDATE moderation_checks SET blob_keys=blob_keys-$2, revision=revision+1,
				state=CASE WHEN content_type='avatar' THEN 'canceled' ELSE 'pending' END,
				fallback_profile=CASE WHEN content_type='avatar' THEN NULL ELSE fallback_profile END,
				attempts=0, next_attempt_at=NOW(), last_error='', updated_at=NOW()
			WHERE tenant_id=$1 AND state IN ('pending', 'running', 'failed') AND blob_keys ? $2 AND $3;

    DELETE FROM attachments a USING media_references r
			WHERE a.tenant_id=$1 AND a.attachment_bkey=$2
			  AND r.tenant_id=a.tenant_id AND r.kind='attachment' AND r.id=a.id AND r.key=a.attachment_bkey
			  AND NOT media_reference_blocks(r.scope,$3,$4,$5);

    UPDATE page_drafts SET banner_image_bkey='', updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND banner_image_bkey=$2 AND NOT shared
			  AND NOT media_reference_blocks('draft',$3,$4,$5);

    UPDATE oauth_providers SET logo_bkey='' WHERE tenant_id=$1 AND logo_bkey=$2 AND $3;

    UPDATE sponsor_creatives SET
			image_key=CASE WHEN image_key=$2 THEN '' ELSE image_key END,
			logo_key=CASE WHEN logo_key=$2 THEN '' ELSE logo_key END,
			data=jsonb_set(data,'{state}','"review"'), revision=revision+1
			WHERE tenant_id=$1 AND (image_key=$2 OR logo_key=$2)
			  AND NOT media_reference_blocks(CASE WHEN data->>'state'='approved' THEN 'active' ELSE 'draft' END,$3,$4,$5);

END;
$$;
