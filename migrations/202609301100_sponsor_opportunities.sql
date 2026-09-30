CREATE TABLE sponsor_opportunities (
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    page_id TEXT NOT NULL,
    placement_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    day DATE NOT NULL,
    campaign_id INTEGER,
    creative_id INTEGER,
    expires_at TIMESTAMPTZ NOT NULL,
    clicked BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, page_id, placement_id, instance_id),
    FOREIGN KEY (tenant_id, campaign_id) REFERENCES sponsor_campaigns(tenant_id, id),
    FOREIGN KEY (tenant_id, creative_id) REFERENCES sponsor_creatives(tenant_id, id)
);

CREATE INDEX sponsor_opportunities_expiry ON sponsor_opportunities (expires_at);
