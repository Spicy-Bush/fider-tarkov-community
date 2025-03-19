ALTER TABLE users ADD COLUMN visual_role INTEGER DEFAULT 0;
COMMENT ON COLUMN users.visual_role IS 'Visual role for display purposes only (0 = use actual role)';