ALTER TABLE notifications ALTER COLUMN post_id DROP NOT NULL;
COMMENT ON COLUMN notifications.post_id IS 'Reference to a post. Can now be NULL for system notifications like mutes and warnings.';