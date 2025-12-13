CREATE INDEX IF NOT EXISTS idx_posts_fts ON posts 
USING GIN (
    (setweight(to_tsvector('english', COALESCE(title, '')), 'A') || 
     setweight(to_tsvector('english', COALESCE(description, '')), 'B'))
);

CREATE INDEX IF NOT EXISTS idx_posts_title_trgm ON posts 
USING GIN (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_posts_description_trgm ON posts 
USING GIN (description gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_posts_tenant_status ON posts (tenant_id, status)
WHERE status != 6;

CREATE INDEX IF NOT EXISTS idx_posts_newest ON posts (tenant_id, status, id DESC)
WHERE status != 6;

CREATE INDEX IF NOT EXISTS idx_posts_most_wanted ON posts (tenant_id, status, (upvotes - downvotes) DESC)
WHERE status != 6;

CREATE INDEX IF NOT EXISTS idx_posts_most_discussed ON posts (tenant_id, status, comments_count DESC)
WHERE status != 6;

CREATE INDEX IF NOT EXISTS idx_posts_trending ON posts (tenant_id, status, last_activity_at DESC)
WHERE status != 6;

CREATE INDEX IF NOT EXISTS idx_post_votes_user_lookup ON post_votes (tenant_id, user_id, post_id);

CREATE INDEX IF NOT EXISTS idx_post_tags_lookup ON post_tags (tenant_id, tag_id, post_id);

