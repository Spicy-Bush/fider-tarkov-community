ALTER TABLE tenants ADD COLUMN last_post_number INTEGER NOT NULL DEFAULT 0;
UPDATE tenants t SET last_post_number = COALESCE(
    (SELECT MAX(p.number) FROM posts p WHERE p.tenant_id = t.id), 0
);

CREATE FUNCTION allocate_post_number() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.number IS NULL THEN
        UPDATE tenants SET last_post_number = last_post_number + 1
        WHERE id = NEW.tenant_id
        RETURNING last_post_number INTO NEW.number;
    ELSE
        -- Imported numbers shouldnt be reused by other submissions
        UPDATE tenants SET last_post_number = GREATEST(last_post_number, NEW.number)
        WHERE id = NEW.tenant_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_allocate_post_number BEFORE INSERT ON posts
FOR EACH ROW EXECUTE FUNCTION allocate_post_number();

CREATE VIEW visible_posts AS SELECT * FROM posts WHERE status <> 6;

DROP TRIGGER trg_post_votes_last_activity ON post_votes;
CREATE TRIGGER trg_post_votes_last_activity
AFTER INSERT OR UPDATE OF vote_type ON post_votes
FOR EACH ROW EXECUTE FUNCTION update_post_last_activity();

ALTER TABLE posts ADD COLUMN submission_id TEXT;
ALTER TABLE posts ADD COLUMN submission_hash TEXT;
ALTER TABLE posts ADD COLUMN submission_result JSONB;
CREATE UNIQUE INDEX posts_submission_identity
ON posts (tenant_id, user_id, submission_id) WHERE submission_id IS NOT NULL;

CREATE TABLE post_notification_deliveries (
    post_id INTEGER PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    post JSONB NOT NULL,
    base_url TEXT NOT NULL,
    locale TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    last_error TEXT,
    FOREIGN KEY (post_id, tenant_id) REFERENCES posts(id, tenant_id) ON DELETE CASCADE
);
CREATE INDEX post_notification_deliveries_pending
ON post_notification_deliveries (available_at) WHERE completed_at IS NULL;

-- Revisions have to survive a vote being removed to stop stale retries
CREATE TABLE post_vote_revisions (
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (post_id, user_id),
    FOREIGN KEY (post_id, tenant_id) REFERENCES posts(id, tenant_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, tenant_id) REFERENCES users(id, tenant_id) ON DELETE CASCADE
);

CREATE FUNCTION advance_vote_revision() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        INSERT INTO post_vote_revisions (post_id, user_id, tenant_id, revision)
        SELECT OLD.post_id, OLD.user_id, OLD.tenant_id, 1
        WHERE EXISTS (SELECT 1 FROM posts WHERE id = OLD.post_id)
          AND EXISTS (SELECT 1 FROM users WHERE id = OLD.user_id)
        ON CONFLICT (post_id, user_id) DO UPDATE SET revision = post_vote_revisions.revision + 1;
        RETURN OLD;
    END IF;
    INSERT INTO post_vote_revisions (post_id, user_id, tenant_id, revision)
    VALUES (NEW.post_id, NEW.user_id, NEW.tenant_id, 1)
    ON CONFLICT (post_id, user_id) DO UPDATE SET revision = post_vote_revisions.revision + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_post_vote_revision AFTER INSERT OR UPDATE OR DELETE ON post_votes
FOR EACH ROW EXECUTE FUNCTION advance_vote_revision();