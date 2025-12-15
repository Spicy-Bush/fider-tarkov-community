ALTER TABLE posts ADD COLUMN moderation_pending BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE posts ADD COLUMN moderation_data JSONB;
ALTER TABLE comments ADD COLUMN moderation_pending BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE comments ADD COLUMN moderation_data JSONB;

CREATE INDEX idx_posts_moderation_pending ON posts(id) WHERE moderation_pending = TRUE;
CREATE INDEX idx_comments_moderation_pending ON comments(id) WHERE moderation_pending = TRUE;

ALTER TABLE reports ALTER COLUMN reporter_id DROP NOT NULL;

