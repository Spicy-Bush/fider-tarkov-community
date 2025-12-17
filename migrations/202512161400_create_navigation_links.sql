create table if not exists navigation_links (
    id              serial primary key,
    tenant_id       int not null,
    title           varchar(100) not null,
    url             varchar(500) not null,
    display_order   int not null default 0,
    location        varchar(20) not null,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    foreign key (tenant_id) references tenants(id) on delete cascade
);

create index if not exists idx_navigation_links_tenant on navigation_links(tenant_id);
create index if not exists idx_navigation_links_location on navigation_links(tenant_id, location);