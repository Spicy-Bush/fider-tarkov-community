create table if not exists sponsorship_packages (
    id              serial primary key,
    tenant_id       int not null references tenants(id) on delete cascade,
    slug            varchar(50) not null,
    name            varchar(100) not null,
    description     text not null default '',
    slots           text not null default '',
    duration_days   int not null default 30,
    sort            int not null default 0,
    created_at      timestamptz not null default now(),
    constraint uq_sponsorship_packages_tenant_slug unique (tenant_id, slug)
);

create index if not exists idx_sponsorship_packages_tenant on sponsorship_packages(tenant_id);

create table if not exists sponsorship_campaigns (
    id                  serial primary key,
    tenant_id           int not null references tenants(id) on delete cascade,
    name                varchar(200) not null,
    slots               text not null default '',
    creative_image_url  text not null default '',
    creative_html       text not null default '',
    click_url           text not null,
    start_at            timestamptz not null,
    end_at              timestamptz not null,
    weight              int not null default 100,
    locale              varchar(10) not null default 'all',
    enabled             boolean not null default true,
    clicks              int not null default 0,
    package_id          int references sponsorship_packages(id) on delete set null,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    constraint chk_sponsorship_campaigns_locale check (locale in ('all','en','ru')),
    constraint chk_sponsorship_campaigns_weight check (weight >= 0),
    constraint chk_sponsorship_campaigns_dates check (end_at > start_at)
);

create index if not exists idx_sponsorship_campaigns_tenant on sponsorship_campaigns(tenant_id);
create index if not exists idx_sponsorship_campaigns_active
    on sponsorship_campaigns(tenant_id, enabled, start_at, end_at);
