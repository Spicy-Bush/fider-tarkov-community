CREATE TABLE reports (
    id              SERIAL PRIMARY KEY,
    tenant_id       INT NOT NULL REFERENCES tenants(id),
    reporter_id     INT NOT NULL REFERENCES users(id),
    reported_type   VARCHAR(20) NOT NULL,
    reported_id     INT NOT NULL,
    reason          VARCHAR(100) NOT NULL,
    details         TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_to     INT REFERENCES users(id),
    assigned_at     TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    resolved_by     INT REFERENCES users(id),
    resolution_note TEXT
);

ALTER TABLE reports ADD CONSTRAINT reports_type_check CHECK (reported_type IN ('post', 'comment'));
ALTER TABLE reports ADD CONSTRAINT reports_status_check CHECK (status IN ('pending', 'in_review', 'resolved', 'dismissed'));

CREATE INDEX idx_reports_tenant_status ON reports(tenant_id, status);
CREATE INDEX idx_reports_reported ON reports(tenant_id, reported_type, reported_id);
CREATE INDEX idx_reports_reporter ON reports(tenant_id, reporter_id);

CREATE TABLE report_reasons (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    slug        VARCHAR(50) NOT NULL,
    title       VARCHAR(100) NOT NULL,
    description TEXT,
    sort_order  INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, slug)
);

INSERT INTO report_reasons (tenant_id, slug, title, description, sort_order)
SELECT id, 'spam', 'Spam', 'Unsolicited or repetitive content', 1 FROM tenants;

INSERT INTO report_reasons (tenant_id, slug, title, description, sort_order)
SELECT id, 'harassment', 'Harassment', 'Abusive, threatening, or hateful content', 2 FROM tenants;

INSERT INTO report_reasons (tenant_id, slug, title, description, sort_order)
SELECT id, 'inappropriate', 'Inappropriate', 'Content that violates community guidelines', 3 FROM tenants;

INSERT INTO report_reasons (tenant_id, slug, title, description, sort_order)
SELECT id, 'other', 'Other', 'Other reason not listed above', 4 FROM tenants;
