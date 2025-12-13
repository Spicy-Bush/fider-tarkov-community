CREATE TABLE IF NOT EXISTS canned_responses (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  type VARCHAR(50) NOT NULL,
  title VARCHAR(100) NOT NULL,
  content TEXT NOT NULL,
  duration VARCHAR(10),
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL,
  created_by_id INTEGER,
  CONSTRAINT canned_responses_tenant_unique UNIQUE (tenant_id, type, title)
);

CREATE INDEX canned_responses_tenant_type_idx ON canned_responses (tenant_id, type);
