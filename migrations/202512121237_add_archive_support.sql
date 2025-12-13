ALTER TABLE posts ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS archived_from_status INT NULL;

CREATE INDEX IF NOT EXISTS idx_posts_archive_candidates ON posts (tenant_id, status, last_activity_at, (upvotes - downvotes), comments_count)
WHERE status NOT IN (6, 7);

DROP INDEX IF EXISTS idx_posts_tenant_status;
CREATE INDEX IF NOT EXISTS idx_posts_tenant_status ON posts (tenant_id, status)
WHERE status NOT IN (6, 7);

DROP INDEX IF EXISTS idx_posts_newest;
CREATE INDEX IF NOT EXISTS idx_posts_newest ON posts (tenant_id, status, id DESC)
WHERE status NOT IN (6, 7);

DROP INDEX IF EXISTS idx_posts_most_wanted;
CREATE INDEX IF NOT EXISTS idx_posts_most_wanted ON posts (tenant_id, status, (upvotes - downvotes) DESC)
WHERE status NOT IN (6, 7);

DROP INDEX IF EXISTS idx_posts_most_discussed;
CREATE INDEX IF NOT EXISTS idx_posts_most_discussed ON posts (tenant_id, status, comments_count DESC)
WHERE status NOT IN (6, 7);

DROP INDEX IF EXISTS idx_posts_trending;
CREATE INDEX IF NOT EXISTS idx_posts_trending ON posts (tenant_id, status, last_activity_at DESC)
WHERE status NOT IN (6, 7);

