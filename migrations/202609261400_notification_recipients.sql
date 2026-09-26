DELETE FROM post_notification_deliveries WHERE completed_at IS NOT NULL;
DROP INDEX post_notification_deliveries_pending;
ALTER TABLE post_notification_deliveries DROP COLUMN completed_at;
ALTER TABLE post_notification_deliveries ADD COLUMN prepared BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE post_notification_deliveries ADD UNIQUE (post_id, tenant_id);
CREATE INDEX post_notification_deliveries_pending
ON post_notification_deliveries (available_at) WHERE NOT prepared;

CREATE TABLE post_notification_recipients (
    post_id INTEGER NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel TEXT NOT NULL CHECK (channel IN ('email', 'push', 'webhook')),
    recipient_id INTEGER NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    send_individually BOOLEAN NOT NULL DEFAULT FALSE,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    PRIMARY KEY (post_id, channel, recipient_id),
    FOREIGN KEY (post_id, tenant_id) REFERENCES post_notification_deliveries(post_id, tenant_id) ON DELETE CASCADE
);
CREATE INDEX post_notification_recipients_due ON post_notification_recipients (available_at);
