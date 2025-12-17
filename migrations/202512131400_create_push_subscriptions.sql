CREATE TABLE IF NOT EXISTS push_subscriptions (
    id SERIAL PRIMARY KEY,
    tenant_id INT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    key_p256dh TEXT NOT NULL,
    key_auth TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_push_subscriptions_unique 
    ON push_subscriptions(tenant_id, user_id, endpoint);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user 
    ON push_subscriptions(tenant_id, user_id);

