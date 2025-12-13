ALTER TABLE posts ADD COLUMN IF NOT EXISTS upvotes INT DEFAULT 0;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS downvotes INT DEFAULT 0;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS comments_count INT DEFAULT 0;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS recent_votes INT DEFAULT 0;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS recent_comments INT DEFAULT 0;

UPDATE posts p SET 
    upvotes = COALESCE((SELECT COUNT(*) FROM post_votes v WHERE v.post_id = p.id AND v.tenant_id = p.tenant_id AND v.vote_type > 0), 0),
    downvotes = COALESCE((SELECT COUNT(*) FROM post_votes v WHERE v.post_id = p.id AND v.tenant_id = p.tenant_id AND v.vote_type < 0), 0),
    comments_count = COALESCE((SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.tenant_id = p.tenant_id AND c.deleted_at IS NULL), 0),
    recent_votes = COALESCE((SELECT SUM(v.vote_type) FROM post_votes v WHERE v.post_id = p.id AND v.tenant_id = p.tenant_id AND v.created_at > CURRENT_DATE - INTERVAL '30 days'), 0),
    recent_comments = COALESCE((SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.tenant_id = p.tenant_id AND c.deleted_at IS NULL AND c.created_at > CURRENT_DATE - INTERVAL '30 days'), 0);

CREATE OR REPLACE FUNCTION update_post_vote_counts()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET 
            upvotes = upvotes + CASE WHEN NEW.vote_type > 0 THEN 1 ELSE 0 END,
            downvotes = downvotes + CASE WHEN NEW.vote_type < 0 THEN 1 ELSE 0 END
        WHERE id = NEW.post_id AND tenant_id = NEW.tenant_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET 
            upvotes = upvotes - CASE WHEN OLD.vote_type > 0 THEN 1 ELSE 0 END,
            downvotes = downvotes - CASE WHEN OLD.vote_type < 0 THEN 1 ELSE 0 END
        WHERE id = OLD.post_id AND tenant_id = OLD.tenant_id;
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        UPDATE posts SET 
            upvotes = upvotes 
                - CASE WHEN OLD.vote_type > 0 THEN 1 ELSE 0 END 
                + CASE WHEN NEW.vote_type > 0 THEN 1 ELSE 0 END,
            downvotes = downvotes 
                - CASE WHEN OLD.vote_type < 0 THEN 1 ELSE 0 END 
                + CASE WHEN NEW.vote_type < 0 THEN 1 ELSE 0 END
        WHERE id = NEW.post_id AND tenant_id = NEW.tenant_id;
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_post_comment_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET comments_count = comments_count + 1
        WHERE id = NEW.post_id AND tenant_id = NEW.tenant_id;
        RETURN NEW;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
            UPDATE posts SET comments_count = comments_count - 1
            WHERE id = NEW.post_id AND tenant_id = NEW.tenant_id;
        ELSIF OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN
            UPDATE posts SET comments_count = comments_count + 1
            WHERE id = NEW.post_id AND tenant_id = NEW.tenant_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.deleted_at IS NULL THEN
            UPDATE posts SET comments_count = comments_count - 1
            WHERE id = OLD.post_id AND tenant_id = OLD.tenant_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_post_votes_counts ON post_votes;
CREATE TRIGGER trg_post_votes_counts
    AFTER INSERT OR UPDATE OR DELETE ON post_votes
    FOR EACH ROW
    EXECUTE FUNCTION update_post_vote_counts();

DROP TRIGGER IF EXISTS trg_post_comments_count ON comments;
CREATE TRIGGER trg_post_comments_count
    AFTER INSERT OR UPDATE OF deleted_at OR DELETE ON comments
    FOR EACH ROW
    EXECUTE FUNCTION update_post_comment_count();

