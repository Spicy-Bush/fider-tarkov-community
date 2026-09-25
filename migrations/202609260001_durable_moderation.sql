CREATE TABLE moderation_checks (
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL CHECK (content_type IN ('post', 'comment', 'name', 'avatar')),
    content_id INTEGER NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    claim BIGINT NOT NULL DEFAULT 0,
    state TEXT NOT NULL CHECK (state IN ('pending', 'running', 'complete', 'failed', 'canceled', 'rejected')),
    text_content TEXT NOT NULL,
    blob_keys JSONB NOT NULL DEFAULT '[]',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    result JSONB,
    fallback_profile JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, content_type, content_id),
    CONSTRAINT moderation_avatar_payload CHECK (
        content_type <> 'avatar' OR state IN ('complete', 'canceled') OR jsonb_array_length(blob_keys) = 1
    )
);

-- Running checks become eligible again when their claim expires
CREATE INDEX moderation_checks_due ON moderation_checks(next_attempt_at) WHERE state IN ('pending', 'running');
CREATE INDEX moderation_profiles_due ON moderation_checks(next_attempt_at)
    WHERE state IN ('pending', 'running') AND content_type IN ('name', 'avatar');
CREATE INDEX users_published_avatar ON users(tenant_id, avatar_bkey) WHERE avatar_bkey <> '';

CREATE TABLE moderation_provider (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO moderation_provider(id) VALUES (1);

CREATE TABLE moderation_provider_slots (
    id INTEGER PRIMARY KEY,
    claim BIGINT NOT NULL DEFAULT 0,
    lease_until TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO moderation_provider_slots(id) SELECT generate_series(1, 32);

-- Existing hides without a provider result came from staff actions
UPDATE posts SET moderation_data = '{"source":"staff"}' WHERE moderation_pending AND moderation_data IS NULL;
UPDATE comments SET moderation_data = '{"source":"staff"}' WHERE moderation_pending AND moderation_data IS NULL;
