ALTER TABLE post_notification_deliveries RENAME TO notification_deliveries;
ALTER TABLE post_notification_recipients RENAME TO notification_recipients;

ALTER TABLE notification_deliveries ADD COLUMN id BIGSERIAL;
ALTER TABLE notification_deliveries ADD UNIQUE (id, tenant_id);
ALTER TABLE notification_recipients ADD COLUMN delivery_id BIGINT;

UPDATE notification_recipients recipient
SET delivery_id = delivery.id
FROM notification_deliveries delivery
WHERE delivery.post_id = recipient.post_id AND delivery.tenant_id = recipient.tenant_id;

ALTER TABLE notification_recipients DROP CONSTRAINT post_notification_recipients_post_id_tenant_id_fkey;
ALTER TABLE notification_recipients DROP CONSTRAINT post_notification_recipients_pkey;
ALTER TABLE notification_recipients DROP COLUMN post_id;
ALTER TABLE notification_recipients ALTER COLUMN delivery_id SET NOT NULL;
ALTER TABLE notification_recipients ADD PRIMARY KEY (delivery_id, channel, recipient_id);
ALTER TABLE notification_recipients ADD FOREIGN KEY (delivery_id, tenant_id)
    REFERENCES notification_deliveries (id, tenant_id) ON DELETE CASCADE;

ALTER TABLE notification_deliveries DROP CONSTRAINT post_notification_deliveries_pkey;
ALTER TABLE notification_deliveries ADD PRIMARY KEY (id);
ALTER TABLE notification_deliveries ALTER COLUMN post_id DROP NOT NULL;
ALTER TABLE notification_deliveries ADD COLUMN comment_id INTEGER REFERENCES comments(id) ON DELETE CASCADE;
ALTER TABLE notification_deliveries ADD CHECK (num_nonnulls(post_id, comment_id) = 1);
ALTER TABLE notification_deliveries RENAME COLUMN post TO payload;
UPDATE notification_deliveries SET payload = jsonb_build_object('post', payload);

CREATE TABLE comment_edit_receipts (
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    submission_id TEXT NOT NULL,
    comment_id INTEGER NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    submission_hash TEXT NOT NULL,
    PRIMARY KEY (tenant_id, user_id, submission_id)
);

CREATE INDEX attachments_comment ON attachments (tenant_id, comment_id);
CREATE INDEX attachments_key ON attachments (tenant_id, attachment_bkey);

ALTER TABLE notifications ADD COLUMN comment_id INTEGER REFERENCES comments(id) ON DELETE CASCADE;
CREATE INDEX notifications_comment ON notifications (comment_id) WHERE comment_id IS NOT NULL;
