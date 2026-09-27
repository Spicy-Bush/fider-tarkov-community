-- Readers may retain a generation after its transaction rolls back.
CREATE SEQUENCE discussion_tree_generation;

CREATE TABLE discussion_tree_versions (
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    post_id INTEGER UNIQUE REFERENCES posts(id) ON DELETE CASCADE,
    page_id INTEGER UNIQUE REFERENCES pages(id) ON DELETE CASCADE,
    generation BIGINT NOT NULL,
    CHECK (num_nonnulls(post_id, page_id) = 1)
);

INSERT INTO discussion_tree_versions (tenant_id, post_id, generation)
SELECT tenant_id, post_id, nextval('discussion_tree_generation')
FROM (SELECT DISTINCT tenant_id, post_id FROM comments WHERE post_id IS NOT NULL) owners;

INSERT INTO discussion_tree_versions (tenant_id, page_id, generation)
SELECT tenant_id, page_id, nextval('discussion_tree_generation')
FROM (SELECT DISTINCT tenant_id, page_id FROM comments WHERE page_id IS NOT NULL) owners;

CREATE FUNCTION advance_discussion_tree_version(owner_tenant INTEGER, owner_post INTEGER, owner_page INTEGER)
RETURNS VOID AS $$
BEGIN
    IF owner_post IS NOT NULL THEN
        INSERT INTO discussion_tree_versions (tenant_id, post_id, generation)
        SELECT post.tenant_id, post.id, nextval('discussion_tree_generation')
        FROM posts post JOIN tenants tenant ON tenant.id = post.tenant_id
        WHERE post.id = owner_post AND post.tenant_id = owner_tenant
        ON CONFLICT (post_id) DO UPDATE SET generation = EXCLUDED.generation;
    ELSE
        INSERT INTO discussion_tree_versions (tenant_id, page_id, generation)
        SELECT page.tenant_id, page.id, nextval('discussion_tree_generation')
        FROM pages page JOIN tenants tenant ON tenant.id = page.tenant_id
        WHERE page.id = owner_page AND page.tenant_id = owner_tenant
        ON CONFLICT (page_id) DO UPDATE SET generation = EXCLUDED.generation;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION update_discussion_tree_version() RETURNS TRIGGER AS $$
BEGIN
    IF (OLD.deleted_at IS NULL) = (NEW.deleted_at IS NULL)
        AND OLD.created_at IS NOT DISTINCT FROM NEW.created_at THEN
        RETURN NULL;
    END IF;

    PERFORM advance_discussion_tree_version(NEW.tenant_id, NEW.post_id, NEW.page_id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION update_discussion_tree_versions() RETURNS TRIGGER AS $$
DECLARE
    owner RECORD;
BEGIN
    FOR owner IN
        SELECT DISTINCT tenant_id, post_id, page_id FROM changed_comments
        ORDER BY tenant_id, post_id, page_id
    LOOP
        PERFORM advance_discussion_tree_version(owner.tenant_id, owner.post_id, owner.page_id);
    END LOOP;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER comments_tree_update
AFTER UPDATE OF deleted_at, created_at ON comments
FOR EACH ROW EXECUTE FUNCTION update_discussion_tree_version();

CREATE TRIGGER comments_tree_insert AFTER INSERT ON comments
REFERENCING NEW TABLE AS changed_comments
FOR EACH STATEMENT EXECUTE FUNCTION update_discussion_tree_versions();

CREATE TRIGGER comments_tree_delete AFTER DELETE ON comments
REFERENCING OLD TABLE AS changed_comments
FOR EACH STATEMENT EXECUTE FUNCTION update_discussion_tree_versions();
