ALTER TABLE posts ADD COLUMN last_activity_at TIMESTAMPTZ;

UPDATE posts p
SET last_activity_at = GREATEST(
    p.created_at,
    COALESCE((SELECT MAX(pv.created_at) FROM post_votes pv WHERE pv.post_id = p.id AND pv.tenant_id = p.tenant_id), p.created_at),
    COALESCE((SELECT MAX(c.created_at) FROM comments c WHERE c.post_id = p.id AND c.tenant_id = p.tenant_id AND c.deleted_at IS NULL), p.created_at)
);

ALTER TABLE posts ALTER COLUMN last_activity_at SET NOT NULL;
ALTER TABLE posts ALTER COLUMN last_activity_at SET DEFAULT NOW();

CREATE OR REPLACE FUNCTION update_post_last_activity()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE posts SET last_activity_at = NOW() WHERE id = NEW.post_id AND tenant_id = NEW.tenant_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_post_last_activity_self()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_activity_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_posts_last_activity
    BEFORE INSERT OR UPDATE OF title, description ON posts
    FOR EACH ROW
    EXECUTE FUNCTION update_post_last_activity_self();

CREATE TRIGGER trg_post_votes_last_activity
    AFTER INSERT ON post_votes
    FOR EACH ROW
    EXECUTE FUNCTION update_post_last_activity();

CREATE TRIGGER trg_comments_last_activity
    AFTER INSERT OR UPDATE OF content ON comments
    FOR EACH ROW
    WHEN (NEW.deleted_at IS NULL)
    EXECUTE FUNCTION update_post_last_activity();

