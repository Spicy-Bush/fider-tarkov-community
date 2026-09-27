ALTER TABLE pages ADD COLUMN allow_comment_images BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE comments ADD COLUMN parent_id INTEGER;
ALTER TABLE comments ADD COLUMN submission_id TEXT;
ALTER TABLE comments ADD COLUMN submission_hash TEXT;

ALTER TABLE comments ADD CONSTRAINT comments_parent_precedes_reply CHECK (parent_id < id);
ALTER TABLE comments ADD UNIQUE (id, tenant_id, post_id);
ALTER TABLE comments ADD UNIQUE (id, tenant_id, page_id);

ALTER TABLE comments ADD CONSTRAINT comments_parent_post
    FOREIGN KEY (parent_id, tenant_id, post_id)
    REFERENCES comments (id, tenant_id, post_id) ON DELETE CASCADE;

ALTER TABLE comments ADD CONSTRAINT comments_parent_page
    FOREIGN KEY (parent_id, tenant_id, page_id)
    REFERENCES comments (id, tenant_id, page_id) ON DELETE CASCADE;

CREATE FUNCTION preserve_comment_parent() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.parent_id IS DISTINCT FROM OLD.parent_id
       OR NEW.post_id IS DISTINCT FROM OLD.post_id
       OR NEW.page_id IS DISTINCT FROM OLD.page_id
       OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION 'Comment ownership and parent cannot be changed';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER comments_preserve_parent
BEFORE UPDATE OF parent_id, post_id, page_id, tenant_id ON comments
FOR EACH ROW EXECUTE FUNCTION preserve_comment_parent();

CREATE INDEX comments_post_roots ON comments (tenant_id, post_id, created_at, id)
    WHERE post_id IS NOT NULL AND parent_id IS NULL;
CREATE INDEX comments_page_roots ON comments (tenant_id, page_id, created_at, id)
    WHERE page_id IS NOT NULL AND parent_id IS NULL;
CREATE INDEX comments_children ON comments (parent_id, created_at, id) WHERE parent_id IS NOT NULL;
CREATE UNIQUE INDEX comments_submission_identity ON comments (tenant_id, user_id, submission_id)
    WHERE submission_id IS NOT NULL;

ALTER TABLE attachments ALTER COLUMN post_id DROP NOT NULL;
ALTER TABLE attachments ADD CONSTRAINT attachments_owner CHECK (post_id IS NOT NULL OR comment_id IS NOT NULL);
ALTER TABLE attachments DROP CONSTRAINT attachments_comment_id_fkey;
ALTER TABLE attachments ADD CONSTRAINT attachments_comment_id_fkey
    FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE;

ALTER TABLE reactions DROP CONSTRAINT reactions_comment_id_fkey;
ALTER TABLE reactions ADD CONSTRAINT reactions_comment_id_fkey
    FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE;
