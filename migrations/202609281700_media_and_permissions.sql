CREATE TABLE command_receipts (
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    submission_id VARCHAR(128) NOT NULL,
    fingerprint TEXT NOT NULL,
    result JSONB NOT NULL,
    PRIMARY KEY (tenant_id, user_id, kind, submission_id),
    FOREIGN KEY (user_id, tenant_id) REFERENCES users(id, tenant_id) ON DELETE CASCADE
);

INSERT INTO command_receipts(tenant_id,user_id,kind,submission_id,fingerprint,result)
SELECT tenant_id,user_id,'post',submission_id,submission_hash,to_jsonb(id)
FROM posts WHERE submission_id IS NOT NULL AND user_id IS NOT NULL;

INSERT INTO command_receipts(tenant_id,user_id,kind,submission_id,fingerprint,result)
SELECT tenant_id,user_id,'comment',submission_id,submission_hash,to_jsonb(id)
FROM comments WHERE submission_id IS NOT NULL AND user_id IS NOT NULL;

INSERT INTO command_receipts(tenant_id,user_id,kind,submission_id,fingerprint,result)
SELECT tenant_id,user_id,'comment-edit',submission_id,submission_hash,'null'::jsonb
FROM comment_edit_receipts;

DROP TABLE comment_edit_receipts;
ALTER TABLE posts DROP COLUMN submission_id, DROP COLUMN submission_hash, DROP COLUMN submission_result;
ALTER TABLE comments DROP COLUMN submission_id, DROP COLUMN submission_hash;

ALTER TABLE tenants ADD COLUMN role_permissions JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN role_post_responses JSONB NOT NULL DEFAULT '{}';
DROP FUNCTION visible_posts_for(integer, text, integer);
CREATE FUNCTION visible_posts_for(viewer_tenant_id integer, viewer_can_moderate boolean, viewer_user_id integer)
RETURNS SETOF posts LANGUAGE sql STABLE PARALLEL RESTRICTED AS $$
    SELECT p.* FROM posts p
    WHERE p.tenant_id = viewer_tenant_id AND p.status <> 6
      AND (NOT p.moderation_pending OR p.user_id = viewer_user_id OR viewer_can_moderate);
$$;
CREATE TABLE feedback_export_presets (
    id VARCHAR(32) NOT NULL CHECK (id ~ '^[0-9a-f]{32}$'),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id),
    name VARCHAR(60),
    recipe JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by_id INTEGER,
    PRIMARY KEY (tenant_id, id),
    -- Deleted identities prevent a late create retry from restoring a removed preset.
    CHECK ((name IS NULL) = (recipe IS NULL)),
    FOREIGN KEY (updated_by_id, tenant_id) REFERENCES users(id, tenant_id)
);

CREATE UNIQUE INDEX feedback_export_presets_name ON feedback_export_presets (tenant_id, lower(name));

-- Keep feedback export picks index-ordered; the status list must match the export queries.
CREATE INDEX idx_posts_export_votes ON posts (tenant_id, (upvotes - downvotes) DESC, id DESC)
    WHERE status IN (0, 1, 2, 3, 4, 5) AND NOT moderation_pending;
CREATE INDEX idx_posts_export_comments ON posts (tenant_id, comments_count DESC, id DESC)
    WHERE status IN (0, 1, 2, 3, 4, 5) AND NOT moderation_pending;
CREATE INDEX idx_posts_export_controversy ON posts (tenant_id, ((upvotes + downvotes) * LEAST(upvotes, downvotes)::float8 / GREATEST(upvotes, downvotes, 1)) DESC, id DESC)
    WHERE status IN (0, 1, 2, 3, 4, 5) AND NOT moderation_pending;
CREATE TABLE media_assets (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key varchar(512) NOT NULL,
    page_id integer REFERENCES pages(id) ON DELETE SET NULL,
    name text,
    content_type varchar(200),
    size bigint CHECK (size >= 0),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    width integer NOT NULL DEFAULT 0 CHECK (width >= 0),
    height integer NOT NULL DEFAULT 0 CHECK (height >= 0),
    is_public boolean NOT NULL DEFAULT false,
    storage_source text DEFAULT 'sql',
    cataloged_at timestamptz DEFAULT NOW(),
    deletion_requested_at timestamptz,
    deleted_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    next_deletion_attempt timestamptz NOT NULL DEFAULT NOW(),
    storage_modified_at timestamptz,
    state text GENERATED ALWAYS AS (
        CASE WHEN deleted_at IS NOT NULL THEN 'deleted'
             WHEN deletion_requested_at IS NOT NULL THEN 'deleting'
             WHEN storage_source IS NOT NULL THEN 'ready'
             WHEN name IS NULL THEN 'unknown'
             ELSE 'unavailable' END
    ) STORED,
    PRIMARY KEY (tenant_id, key),
    CHECK (deleted_at IS NULL OR deletion_requested_at IS NOT NULL),
    CHECK (storage_source IS NULL OR (name IS NOT NULL AND content_type IS NOT NULL AND size IS NOT NULL))
);

CREATE INDEX media_assets_created ON media_assets (tenant_id, created_at DESC, key DESC)
    WHERE deleted_at IS NULL AND storage_source IS NOT NULL;

CREATE TABLE media_asset_refs (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key text NOT NULL,
    kind text NOT NULL,
    owner_id integer NOT NULL,
    field text NOT NULL,
    PRIMARY KEY (tenant_id, key, kind, owner_id, field)
);

CREATE INDEX media_asset_refs_owner ON media_asset_refs (tenant_id, kind, owner_id);

-- Media owner schema generated from mediaowners.All.
CREATE VIEW media_reference_sources AS
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

CREATE VIEW media_reference_scopes AS
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
FROM page_drafts o;

CREATE FUNCTION media_text_may_reference(text)
RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(strpos($1, '/') > 0
        OR strpos($1, '%') > 0
        OR strpos($1, '&') > 0
        OR strpos($1,  E'\\') > 0, false);
$$;

CREATE TABLE media_reference_changes (
    transaction_id bigint NOT NULL,
    tenant_id integer NOT NULL,
    kind text NOT NULL,
    owner_id integer NOT NULL,
    source jsonb NOT NULL,
    PRIMARY KEY (transaction_id, tenant_id, kind, owner_id)
);

CREATE FUNCTION check_media_reference_completion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM media_reference_changes WHERE transaction_id=txid_current()) THEN
        RAISE EXCEPTION 'Media reference changes must be completed before commit' USING ERRCODE='23514';
    END IF;
    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER media_reference_completion
AFTER INSERT OR UPDATE ON media_reference_changes
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_media_reference_completion();

CREATE FUNCTION "capture_media_tenant"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."id" AND kind='tenant'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."id" AND kind='tenant'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."logo_bkey" IS NOT DISTINCT FROM NEW."logo_bkey" AND OLD."welcome_message" IS NOT DISTINCT FROM NEW."welcome_message" AND OLD."message_banner" IS NOT DISTINCT FROM NEW."message_banner" AND OLD."custom_css" IS NOT DISTINCT FROM NEW."custom_css" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."logo_bkey"::text, '') <> '' OR media_text_may_reference(NEW."welcome_message"::text) OR media_text_may_reference(NEW."message_banner"::text) OR media_text_may_reference(NEW."custom_css"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."id", 'kind', 'tenant'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."id",'tenant'::text,NEW."id",jsonb_build_object('logo_bkey', NEW."logo_bkey", 'welcome_message', NEW."welcome_message", 'message_banner', NEW."message_banner", 'custom_css', NEW."custom_css"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_tenant_references" AFTER INSERT OR DELETE OR UPDATE OF "logo_bkey", "welcome_message", "message_banner", "custom_css" ON "tenants"
FOR EACH ROW EXECUTE FUNCTION "capture_media_tenant"();

CREATE FUNCTION "capture_media_post"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='post'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='post'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."description" IS NOT DISTINCT FROM NEW."description" AND OLD."response" IS NOT DISTINCT FROM NEW."response" AND OLD."locked_settings" IS NOT DISTINCT FROM NEW."locked_settings" THEN
        RETURN NULL;
    END IF;

    IF NOT (media_text_may_reference(NEW."description"::text) OR media_text_may_reference(NEW."response"::text) OR media_text_may_reference(NEW."locked_settings"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'post'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'post'::text,NEW."id",jsonb_build_object('description', NEW."description", 'response', NEW."response", 'locked_settings', NEW."locked_settings"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_post_references" AFTER INSERT OR DELETE OR UPDATE OF "description", "response", "locked_settings" ON "posts"
FOR EACH ROW EXECUTE FUNCTION "capture_media_post"();

CREATE FUNCTION "capture_media_page"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='page'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='page'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."banner_image_bkey" IS NOT DISTINCT FROM NEW."banner_image_bkey" AND OLD."og_image_bkey" IS NOT DISTINCT FROM NEW."og_image_bkey" AND OLD."content" IS NOT DISTINCT FROM NEW."content" AND OLD."excerpt" IS NOT DISTINCT FROM NEW."excerpt" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."banner_image_bkey"::text, '') <> '' OR COALESCE(NEW."og_image_bkey"::text, '') <> '' OR media_text_may_reference(NEW."content"::text) OR media_text_may_reference(NEW."excerpt"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'page'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'page'::text,NEW."id",jsonb_build_object('banner_image_bkey', NEW."banner_image_bkey", 'og_image_bkey', NEW."og_image_bkey", 'content', NEW."content", 'excerpt', NEW."excerpt"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_page_references" AFTER INSERT OR DELETE OR UPDATE OF "banner_image_bkey", "og_image_bkey", "content", "excerpt" ON "pages"
FOR EACH ROW EXECUTE FUNCTION "capture_media_page"();

CREATE FUNCTION "capture_media_user"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='user'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='user'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."avatar_bkey" IS NOT DISTINCT FROM NEW."avatar_bkey" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."avatar_bkey"::text, '') <> '') THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'user'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'user'::text,NEW."id",jsonb_build_object('avatar_bkey', NEW."avatar_bkey"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_user_references" AFTER INSERT OR DELETE OR UPDATE OF "avatar_bkey" ON "users"
FOR EACH ROW EXECUTE FUNCTION "capture_media_user"();

CREATE FUNCTION "capture_media_comment"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='comment'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='comment'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."content" IS NOT DISTINCT FROM NEW."content" THEN
        RETURN NULL;
    END IF;

    IF NOT (media_text_may_reference(NEW."content"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'comment'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'comment'::text,NEW."id",jsonb_build_object('content', NEW."content"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_comment_references" AFTER INSERT OR DELETE OR UPDATE OF "content" ON "comments"
FOR EACH ROW EXECUTE FUNCTION "capture_media_comment"();

CREATE FUNCTION "capture_media_moderation"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='moderation:' || OLD.content_type AND owner_id=OLD."content_id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='moderation:' || OLD.content_type AND owner_id=OLD."content_id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."content_type" IS NOT DISTINCT FROM NEW."content_type" AND OLD."state" IS NOT DISTINCT FROM NEW."state" AND OLD."text_content" IS NOT DISTINCT FROM NEW."text_content" AND OLD."blob_keys" IS NOT DISTINCT FROM NEW."blob_keys" AND OLD."fallback_profile" IS NOT DISTINCT FROM NEW."fallback_profile" THEN
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'moderation:' || NEW.content_type,NEW."content_id",jsonb_build_object('content_type', NEW."content_type", 'state', NEW."state", 'text_content', NEW."text_content", 'blob_keys', NEW."blob_keys", 'fallback_profile', NEW."fallback_profile"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_moderation_references" AFTER INSERT OR DELETE OR UPDATE OF "content_type", "state", "text_content", "blob_keys", "fallback_profile" ON "moderation_checks"
FOR EACH ROW EXECUTE FUNCTION "capture_media_moderation"();

CREATE FUNCTION "capture_media_attachment"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='attachment'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='attachment'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."attachment_bkey" IS NOT DISTINCT FROM NEW."attachment_bkey" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."attachment_bkey"::text, '') <> '') THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'attachment'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'attachment'::text,NEW."id",jsonb_build_object('attachment_bkey', NEW."attachment_bkey"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_attachment_references" AFTER INSERT OR DELETE OR UPDATE OF "attachment_bkey" ON "attachments"
FOR EACH ROW EXECUTE FUNCTION "capture_media_attachment"();

CREATE FUNCTION "capture_media_draft"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='draft'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='draft'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."banner_image_bkey" IS NOT DISTINCT FROM NEW."banner_image_bkey" AND OLD."content" IS NOT DISTINCT FROM NEW."content" AND OLD."excerpt" IS NOT DISTINCT FROM NEW."excerpt" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."banner_image_bkey"::text, '') <> '' OR media_text_may_reference(NEW."content"::text) OR media_text_may_reference(NEW."excerpt"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'draft'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'draft'::text,NEW."id",jsonb_build_object('banner_image_bkey', NEW."banner_image_bkey", 'content', NEW."content", 'excerpt', NEW."excerpt"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_draft_references" AFTER INSERT OR DELETE OR UPDATE OF "banner_image_bkey", "content", "excerpt" ON "page_drafts"
FOR EACH ROW EXECUTE FUNCTION "capture_media_draft"();

CREATE FUNCTION "capture_media_oauth"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='oauth'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='oauth'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."logo_bkey" IS NOT DISTINCT FROM NEW."logo_bkey" THEN
        RETURN NULL;
    END IF;

    IF NOT (COALESCE(NEW."logo_bkey"::text, '') <> '') THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'oauth'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'oauth'::text,NEW."id",jsonb_build_object('logo_bkey', NEW."logo_bkey"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_oauth_references" AFTER INSERT OR DELETE OR UPDATE OF "logo_bkey" ON "oauth_providers"
FOR EACH ROW EXECUTE FUNCTION "capture_media_oauth"();

CREATE FUNCTION "capture_media_creative"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='creative'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='creative'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."image_url" IS NOT DISTINCT FROM NEW."image_url" AND OLD."html" IS NOT DISTINCT FROM NEW."html" THEN
        RETURN NULL;
    END IF;

    IF NOT (media_text_may_reference(NEW."image_url"::text) OR media_text_may_reference(NEW."html"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'creative'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'creative'::text,NEW."id",jsonb_build_object('image_url', NEW."image_url", 'html', NEW."html"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_creative_references" AFTER INSERT OR DELETE OR UPDATE OF "image_url", "html" ON "creative_versions"
FOR EACH ROW EXECUTE FUNCTION "capture_media_creative"();

CREATE FUNCTION "capture_media_response"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='response'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='response'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."content" IS NOT DISTINCT FROM NEW."content" THEN
        RETURN NULL;
    END IF;

    IF NOT (media_text_may_reference(NEW."content"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'response'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'response'::text,NEW."id",jsonb_build_object('content', NEW."content"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_response_references" AFTER INSERT OR DELETE OR UPDATE OF "content" ON "canned_responses"
FOR EACH ROW EXECUTE FUNCTION "capture_media_response"();

CREATE FUNCTION "capture_media_navigation"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='navigation'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='navigation'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."url" IS NOT DISTINCT FROM NEW."url" THEN
        RETURN NULL;
    END IF;

    IF NOT (media_text_may_reference(NEW."url"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'navigation'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'navigation'::text,NEW."id",jsonb_build_object('url', NEW."url"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_navigation_references" AFTER INSERT OR DELETE OR UPDATE OF "url" ON "navigation_links"
FOR EACH ROW EXECUTE FUNCTION "capture_media_navigation"();

CREATE FUNCTION "capture_media_webhook"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD."tenant_id" AND kind='webhook'::text AND owner_id=OLD."id";
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD."tenant_id" AND kind='webhook'::text AND owner_id=OLD."id";
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND OLD."url" IS NOT DISTINCT FROM NEW."url" AND OLD."content" IS NOT DISTINCT FROM NEW."content" THEN
        RETURN NULL;
    END IF;

    IF NOT (media_text_may_reference(NEW."url"::text) OR media_text_may_reference(NEW."content"::text)) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW."tenant_id", 'kind', 'webhook'::text, 'owner_id', NEW."id", 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW."tenant_id",'webhook'::text,NEW."id",jsonb_build_object('url', NEW."url", 'content', NEW."content"))
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

CREATE TRIGGER "media_webhook_references" AFTER INSERT OR DELETE OR UPDATE OF "url", "content" ON "webhooks"
FOR EACH ROW EXECUTE FUNCTION "capture_media_webhook"();

CREATE FUNCTION lock_media_reference_owners(requested_tenant integer, requested_key text)
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

CREATE FUNCTION unlink_media_reference_fields(integer, text, boolean, boolean, boolean)
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

END;
$$;
-- End generated media owner schema.

ALTER TABLE media_asset_refs ADD FOREIGN KEY (tenant_id,key)
    REFERENCES media_assets(tenant_id,key);

CREATE VIEW media_references AS
    SELECT r.tenant_id, r.key, r.kind, r.owner_id AS id, r.field,
           COALESCE(s.scope, 'active') AS scope
    FROM media_asset_refs r
    LEFT JOIN media_reference_scopes s
        ON s.tenant_id=r.tenant_id AND s.kind=r.kind AND s.owner_id=r.owner_id;

CREATE FUNCTION media_reference_blocks(scope text, force boolean, include_deleted boolean, include_drafts boolean)
RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT NOT (force OR (include_deleted AND scope='deleted') OR (include_drafts AND scope='draft'));
$$;

CREATE FUNCTION media_reference_flags(
    requested_tenant integer,
    requested_keys text[],
    include_deleted boolean DEFAULT false,
    include_drafts boolean DEFAULT false
)
RETURNS TABLE (key text, is_in_use boolean, has_protected_references boolean)
LANGUAGE sql STABLE AS $$
    SELECT r.key, true, bool_or(media_reference_blocks(r.scope, false, include_deleted, include_drafts))
    FROM media_references r
    WHERE r.tenant_id=requested_tenant AND r.key=ANY(requested_keys)
    GROUP BY r.key;
$$;

CREATE FUNCTION replace_media_references(sources jsonb) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    unavailable_key text;
BEGIN
    INSERT INTO media_assets(tenant_id, key, storage_source, cataloged_at)
    SELECT DISTINCT s.tenant_id, r.key, NULL::text, NULL::timestamptz
    FROM jsonb_to_recordset(sources) s(tenant_id integer, refs jsonb)
    CROSS JOIN LATERAL jsonb_to_recordset(s.refs) r(key text)
    ORDER BY s.tenant_id, r.key ON CONFLICT DO NOTHING;

    PERFORM 1 FROM media_assets a
    JOIN (
        SELECT DISTINCT s.tenant_id, r.key
        FROM jsonb_to_recordset(sources) s(tenant_id integer, refs jsonb)
        CROSS JOIN LATERAL jsonb_to_recordset(s.refs) r(key text)
    ) requested USING (tenant_id, key)
    ORDER BY a.tenant_id, a.key FOR SHARE OF a;

    SELECT r.key INTO unavailable_key
    FROM jsonb_to_recordset(sources) s(tenant_id integer, kind text, owner_id integer, refs jsonb)
    CROSS JOIN LATERAL jsonb_to_recordset(s.refs) r(key text, field text)
    JOIN media_assets a ON a.tenant_id=s.tenant_id AND a.key=r.key
    WHERE a.state IN ('deleting', 'deleted')
      AND NOT EXISTS (
          SELECT 1 FROM media_asset_refs prior
          WHERE prior.tenant_id=s.tenant_id AND prior.key=r.key
            AND prior.kind=s.kind AND prior.owner_id=s.owner_id AND prior.field=r.field
      )
      AND NOT (s.kind IN ('moderation:post', 'moderation:comment') AND r.field='text_content' AND EXISTS (
          SELECT 1 FROM media_asset_refs prior
          WHERE prior.tenant_id=s.tenant_id AND prior.key=r.key
            AND prior.kind=split_part(s.kind, ':', 2) AND prior.owner_id=s.owner_id
      ))
      AND NOT (s.kind IN ('page', 'draft') AND r.field IN ('content', 'excerpt') AND EXISTS (
          SELECT 1 FROM page_drafts draft
          JOIN media_asset_refs prior ON prior.tenant_id=draft.tenant_id AND prior.key=r.key AND prior.field=r.field
          WHERE draft.tenant_id=s.tenant_id AND draft.shared
            AND ((s.kind='draft' AND draft.id=s.owner_id AND prior.kind='page' AND prior.owner_id=draft.page_id)
              OR (s.kind='page' AND draft.page_id=s.owner_id AND prior.kind='draft' AND prior.owner_id=draft.id))
      ))
    LIMIT 1;
    IF unavailable_key IS NOT NULL THEN
        RAISE EXCEPTION 'Media asset % is being deleted', unavailable_key USING ERRCODE='23503';
    END IF;

    DELETE FROM media_asset_refs old
    USING jsonb_to_recordset(sources) s(tenant_id integer, kind text, owner_id integer)
    WHERE old.tenant_id=s.tenant_id AND old.kind=s.kind AND old.owner_id=s.owner_id;

    INSERT INTO media_asset_refs(tenant_id, key, kind, owner_id, field)
    SELECT s.tenant_id, r.key, s.kind, s.owner_id, r.field
    FROM jsonb_to_recordset(sources) s(tenant_id integer, kind text, owner_id integer, refs jsonb)
    CROSS JOIN LATERAL jsonb_to_recordset(s.refs) r(key text, field text);

    DELETE FROM media_reference_changes pending
    USING jsonb_to_recordset(sources) s(tenant_id integer, kind text, owner_id integer)
    WHERE pending.transaction_id=txid_current_if_assigned()
      AND pending.tenant_id=s.tenant_id AND pending.kind=s.kind AND pending.owner_id=s.owner_id;
END;
$$;

CREATE TABLE media_thumbnails (
    tenant_id integer NOT NULL,
    key varchar(512) NOT NULL,
    size integer NOT NULL CHECK (size IN (200, 512)),
    content bytea NOT NULL,
    PRIMARY KEY (tenant_id, key, size),
    FOREIGN KEY (tenant_id, key) REFERENCES media_assets (tenant_id, key) ON DELETE CASCADE
);


CREATE INDEX media_assets_name ON media_assets (tenant_id, lower(name), key)
    WHERE deleted_at IS NULL;

CREATE INDEX media_assets_search ON media_assets USING gin (lower(name) gin_trgm_ops)
    WHERE deleted_at IS NULL;

CREATE INDEX media_assets_size ON media_assets (tenant_id, size, key)
    WHERE deleted_at IS NULL;

CREATE INDEX media_deletion_pending ON media_assets (next_deletion_attempt, tenant_id, key)
WHERE deletion_requested_at IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX media_assets_source ON media_assets (tenant_id, storage_source, created_at DESC, key DESC)
WHERE deleted_at IS NULL;

CREATE TABLE media_inventory (
    tenant_id integer NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    storage_source text NOT NULL,
    version bigint NOT NULL DEFAULT 0,
    cursor text NOT NULL DEFAULT '',
    scanned bigint NOT NULL DEFAULT 0,
    skipped bigint NOT NULL DEFAULT 0,
    completed_at timestamptz,
    scan_started_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    retry_after timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, storage_source)
);
CREATE FUNCTION media_file_listing(
    requested_tenant integer,
    search_pattern text,
    requested_type text,
    requested_usage text,
    created_before timestamptz,
    include_deleted boolean DEFAULT false,
    include_drafts boolean DEFAULT false,
    requested_source text DEFAULT 'sql'
)
RETURNS TABLE (
    key varchar,
    name text,
    content_type varchar,
    size bigint,
    created_at timestamptz,
    width integer,
    height integer,
    state text,
    last_error text
)
LANGUAGE sql STABLE AS $$
    SELECT a.key, a.name, a.content_type, a.size, a.created_at, a.width, a.height,
           a.state,
           a.last_error
    FROM media_assets a
    WHERE a.tenant_id = requested_tenant AND a.deleted_at IS NULL AND a.storage_source=requested_source
      AND (search_pattern = '' OR lower(a.name) LIKE search_pattern ESCAPE '\')
      AND (requested_type = 'all' OR split_part(a.key, '/', 1) = requested_type)
      AND (
          created_before IS NULL
          OR (a.created_at <= created_before AND a.cataloged_at <= created_before)
      )
      AND (
          requested_usage = 'all'
          OR (
              (requested_usage='used' OR a.deletion_requested_at IS NULL)
              AND (a.key IN (
                  SELECT r.key
                  FROM media_asset_refs r
                  LEFT JOIN media_reference_scopes s
                      ON (include_deleted OR include_drafts)
                      AND s.tenant_id=r.tenant_id AND s.kind=r.kind AND s.owner_id=r.owner_id
                  WHERE r.tenant_id=requested_tenant
                    AND media_reference_blocks(
                        CASE WHEN r.kind='draft' THEN 'draft' ELSE COALESCE(s.scope, 'active') END,
                        false, include_deleted, include_drafts
                    )
              )) = (requested_usage='used')
          )
      );
$$;
CREATE TABLE media_inventory_candidates (
    tenant_id integer NOT NULL,
    storage_source text NOT NULL,
    key varchar(512) NOT NULL,
    cataloged_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, storage_source, key),
    FOREIGN KEY (tenant_id, storage_source) REFERENCES media_inventory(tenant_id, storage_source) ON DELETE CASCADE
);
CREATE UNIQUE INDEX pages_tenant_identity ON pages (tenant_id, id);
ALTER TABLE media_assets
    ADD FOREIGN KEY (tenant_id, page_id) REFERENCES pages(tenant_id, id);
CREATE UNIQUE INDEX page_topics_tenant_identity ON page_topics (tenant_id, id);
CREATE UNIQUE INDEX page_tags_tenant_identity ON page_tags (tenant_id, id);

UPDATE pages child
SET parent_page_id = NULL
FROM pages parent
WHERE child.parent_page_id = parent.id AND child.tenant_id <> parent.tenant_id;

ALTER TABLE pages DROP CONSTRAINT pages_parent_page_id_fkey;
ALTER TABLE pages ADD CONSTRAINT pages_parent_tenant_fkey
    FOREIGN KEY (tenant_id, parent_page_id) REFERENCES pages (tenant_id, id);

ALTER TABLE page_authors ADD COLUMN tenant_id int;
ALTER TABLE page_topics_map ADD COLUMN tenant_id int;
ALTER TABLE page_tags_map ADD COLUMN tenant_id int;

UPDATE page_authors relation SET tenant_id = page.tenant_id
FROM pages page WHERE page.id = relation.page_id;

UPDATE page_topics_map relation SET tenant_id = page.tenant_id
FROM pages page WHERE page.id = relation.page_id;

UPDATE page_tags_map relation SET tenant_id = page.tenant_id
FROM pages page WHERE page.id = relation.page_id;

DELETE FROM page_authors relation USING users author
WHERE relation.user_id = author.id AND relation.tenant_id <> author.tenant_id;

DELETE FROM page_topics_map relation USING page_topics topic
WHERE relation.topic_id = topic.id AND relation.tenant_id <> topic.tenant_id;

DELETE FROM page_tags_map relation USING page_tags tag
WHERE relation.tag_id = tag.id AND relation.tenant_id <> tag.tenant_id;

ALTER TABLE page_authors
    ALTER COLUMN tenant_id SET NOT NULL,
    DROP CONSTRAINT page_authors_page_id_fkey,
    DROP CONSTRAINT page_authors_user_id_fkey,
    ADD FOREIGN KEY (tenant_id, page_id) REFERENCES pages (tenant_id, id) ON DELETE CASCADE,
    ADD FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE page_topics_map
    ALTER COLUMN tenant_id SET NOT NULL,
    DROP CONSTRAINT page_topics_map_page_id_fkey,
    DROP CONSTRAINT page_topics_map_topic_id_fkey,
    ADD FOREIGN KEY (tenant_id, page_id) REFERENCES pages (tenant_id, id) ON DELETE CASCADE,
    ADD FOREIGN KEY (tenant_id, topic_id) REFERENCES page_topics (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE page_tags_map
    ALTER COLUMN tenant_id SET NOT NULL,
    DROP CONSTRAINT page_tags_map_page_id_fkey,
    DROP CONSTRAINT page_tags_map_tag_id_fkey,
    ADD FOREIGN KEY (tenant_id, page_id) REFERENCES pages (tenant_id, id) ON DELETE CASCADE,
    ADD FOREIGN KEY (tenant_id, tag_id) REFERENCES page_tags (tenant_id, id) ON DELETE CASCADE;

CREATE TABLE ad_placement_settings (
    tenant_id int NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    placement_id text NOT NULL REFERENCES ad_placements(id) ON DELETE CASCADE,
    adsense_slot_id text NOT NULL DEFAULT '',
    adsense_format text NOT NULL DEFAULT '',
    empty_policy text NOT NULL DEFAULT 'collapse' CHECK (empty_policy IN ('collapse', 'reserve')),
    PRIMARY KEY (tenant_id, placement_id)
);

INSERT INTO ad_placement_settings (tenant_id, placement_id, adsense_slot_id, adsense_format, empty_policy)
SELECT tenant.id, placement.id, placement.adsense_slot_id, placement.adsense_format, placement.empty_policy
FROM tenants tenant CROSS JOIN ad_placements placement;

ALTER TABLE ad_placements
    DROP COLUMN adsense_slot_id,
    DROP COLUMN empty_policy;
ALTER TABLE ad_placements RENAME COLUMN adsense_format TO default_adsense_format;

UPDATE ad_placements SET default_adsense_format = CASE id
    WHEN 'feed_native' THEN 'fluid'
    WHEN 'sidebar_top' THEN 'rectangle'
    WHEN 'post_below_title' THEN 'horizontal'
    WHEN 'pages_header' THEN 'horizontal'
    ELSE ''
END;

ALTER TABLE page_drafts
    ADD COLUMN shared boolean NOT NULL DEFAULT false,
    ADD COLUMN collaborative_state bytea,
    DROP CONSTRAINT unique_page_user_draft,
    ADD CHECK (shared = (collaborative_state IS NOT NULL));

CREATE UNIQUE INDEX page_drafts_personal ON page_drafts (page_id, user_id) WHERE NOT shared;
CREATE UNIQUE INDEX page_drafts_shared ON page_drafts (tenant_id, page_id) WHERE shared;


CREATE FUNCTION page_is_visible(
    status text, visibility text, allowed_roles jsonb,
    viewer_role text, viewer_can_manage boolean
)
RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT viewer_can_manage OR (status = 'published' AND (
        visibility IN ('public', 'unlisted') OR
        (visibility = 'private' AND viewer_role <> '' AND COALESCE(allowed_roles ? viewer_role, false))
    ));
$$;

CREATE FUNCTION image_access(
    integer, text, text, integer, text,
    boolean, boolean, integer[], boolean, integer
)
RETURNS TABLE(allowed boolean, version text)
LANGUAGE plpgsql STABLE AS $$
BEGIN
    RETURN QUERY
    WITH viewer AS (
        SELECT $1::integer AS tenant_id, $2::text AS key, $3::text AS storage_source,
               $4::integer AS user_id, $5::text AS role, $6::boolean AS manage_pages,
               $7::boolean AS moderate_posts, $8::integer[] AS moderated_roles,
               $9::boolean AS active
    ), visible_pages AS NOT MATERIALIZED (
        SELECT page.id, page.banner_image_bkey, page.og_image_bkey
        FROM pages page, viewer
        WHERE page.tenant_id = viewer.tenant_id AND page_is_visible(
            page.status, page.visibility, page.allowed_roles, viewer.role, viewer.manage_pages
        )
    )
    SELECT asset.deletion_requested_at IS NULL AND (
        COALESCE(asset.storage_source = viewer.storage_source AND (
            asset.is_public OR (viewer.active AND (
                EXISTS (
                    SELECT 1 FROM media_asset_refs reference
                    JOIN page_drafts draft ON draft.tenant_id = reference.tenant_id AND draft.id = reference.owner_id
                    WHERE reference.tenant_id = viewer.tenant_id AND reference.key = viewer.key AND reference.kind = 'draft'
                      AND draft.banner_image_bkey = viewer.key
                      AND (draft.user_id = viewer.user_id OR (draft.shared AND viewer.manage_pages))
                ) OR (viewer.manage_pages AND asset.page_id IS NOT NULL)
            ))
        ), false)
        OR EXISTS (
            SELECT 1 FROM media_asset_refs reference
            JOIN visible_pages page ON page.id = reference.owner_id
            WHERE reference.tenant_id = viewer.tenant_id AND reference.key = viewer.key AND reference.kind = 'page'
              AND (page.banner_image_bkey = viewer.key OR page.og_image_bkey = viewer.key)
        )
        OR EXISTS (
            SELECT 1 FROM attachments attachment
            LEFT JOIN comments comment ON comment.tenant_id = attachment.tenant_id AND comment.id = attachment.comment_id
            LEFT JOIN users author ON author.tenant_id = comment.tenant_id AND author.id = comment.user_id
            LEFT JOIN posts post ON post.tenant_id = attachment.tenant_id AND post.id = COALESCE(attachment.post_id, comment.post_id)
            WHERE attachment.tenant_id = viewer.tenant_id AND attachment.attachment_bkey = viewer.key
              AND (attachment.comment_id IS NULL OR (
                comment.deleted_at IS NULL AND (NOT comment.moderation_pending OR comment.user_id = viewer.user_id
                    OR COALESCE(author.role, 0) = ANY(viewer.moderated_roles))
              ))
              AND ((post.status <> $10 AND (NOT post.moderation_pending OR post.user_id = viewer.user_id OR viewer.moderate_posts))
                OR EXISTS (SELECT 1 FROM visible_pages page WHERE page.id = comment.page_id))
        )
    ) AS allowed,
    CASE WHEN asset.storage_source = viewer.storage_source THEN COALESCE(asset.cataloged_at::text, '') ELSE '' END AS version
    FROM viewer
    LEFT JOIN media_assets asset ON asset.tenant_id = viewer.tenant_id AND asset.key = viewer.key;
END;
$$;
ALTER TABLE notification_deliveries
    ADD COLUMN mention_ids INTEGER[] NOT NULL DEFAULT '{}',
    ADD COLUMN edited BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE notification_deliveries
SET mention_ids = ARRAY(
        SELECT value::integer
        FROM jsonb_array_elements_text(COALESCE(NULLIF(payload #> '{comment,mentionIds}', 'null'::jsonb), '[]'::jsonb)) AS mention(value)
    ),
    edited = COALESCE((payload #>> '{comment,edited}')::boolean, FALSE);

ALTER TABLE notification_deliveries DROP COLUMN payload;
DROP INDEX idx_posts_newest;
DROP INDEX idx_posts_most_wanted;
DROP INDEX idx_posts_most_discussed;

CREATE INDEX idx_posts_newest ON posts (tenant_id, created_at DESC, id DESC)
    WHERE status <> 6;
CREATE INDEX idx_posts_most_wanted ON posts (tenant_id, (upvotes - downvotes) DESC, id DESC)
    WHERE status <> 6;
CREATE INDEX idx_posts_most_discussed ON posts (tenant_id, comments_count DESC, id DESC)
    WHERE status <> 6;
DROP INDEX idx_posts_tenant_status;
CREATE INDEX idx_posts_tenant_status ON posts (tenant_id, status)
    INCLUDE (moderation_pending, user_id)
    WHERE status <> 6;
