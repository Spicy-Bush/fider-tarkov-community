-- Pages feature migration

create table if not exists pages (
    id                      serial primary key,
    tenant_id               int not null,
    title                   varchar(200) not null,
    slug                    varchar(200) not null,
    content                 text not null,
    excerpt                 text,
    banner_image_bkey       varchar,
    status                  varchar(20) not null default 'draft',
    visibility              varchar(20) not null default 'public',
    allowed_roles           jsonb,
    parent_page_id          int,
    allow_comments          boolean not null default true,
    allow_reactions         boolean not null default true,
    show_toc                boolean not null default false,
    view_count              int not null default 0,
    scheduled_for           timestamptz,
    published_at            timestamptz,
    created_at              timestamptz not null default now(),
    updated_at              timestamptz not null default now(),
    created_by_id           int not null,
    updated_by_id           int not null,
    meta_description        text,
    meta_keywords           text,
    canonical_url           varchar,
    og_image_bkey           varchar,
    cached_embedded_data    jsonb,
    cached_at               timestamptz,
    foreign key (tenant_id) references tenants(id),
    foreign key (parent_page_id) references pages(id),
    foreign key (created_by_id) references users(id),
    foreign key (updated_by_id) references users(id),
    constraint unique_tenant_slug unique (tenant_id, slug)
);

create index idx_pages_tenant_status on pages(tenant_id, status);
create index idx_pages_tenant_visibility on pages(tenant_id, visibility);
create index idx_pages_scheduled on pages(scheduled_for) where scheduled_for is not null;

create table if not exists page_authors (
    page_id         int not null,
    user_id         int not null,
    display_order   int not null default 0,
    primary key (page_id, user_id),
    foreign key (page_id) references pages(id) on delete cascade,
    foreign key (user_id) references users(id) on delete cascade
);

create table if not exists page_topics (
    id          serial primary key,
    tenant_id   int not null,
    name        varchar(100) not null,
    slug        varchar(100) not null,
    description text,
    color       varchar(7),
    foreign key (tenant_id) references tenants(id),
    constraint unique_tenant_topic_slug unique (tenant_id, slug)
);

create table if not exists page_topics_map (
    page_id     int not null,
    topic_id    int not null,
    primary key (page_id, topic_id),
    foreign key (page_id) references pages(id) on delete cascade,
    foreign key (topic_id) references page_topics(id) on delete cascade
);

create table if not exists page_tags (
    id          serial primary key,
    tenant_id   int not null,
    name        varchar(100) not null,
    slug        varchar(100) not null,
    foreign key (tenant_id) references tenants(id),
    constraint unique_tenant_tag_slug unique (tenant_id, slug)
);

create table if not exists page_tags_map (
    page_id     int not null,
    tag_id      int not null,
    primary key (page_id, tag_id),
    foreign key (page_id) references pages(id) on delete cascade,
    foreign key (tag_id) references page_tags(id) on delete cascade
);

create table if not exists page_drafts (
    id                  serial primary key,
    page_id             int not null,
    tenant_id           int not null,
    user_id             int not null,
    title               varchar(200),
    slug                varchar(200),
    content             text,
    excerpt             text,
    banner_image_bkey   varchar,
    meta_description    text,
    show_toc            boolean not null default false,
    draft_data          jsonb,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    foreign key (page_id) references pages(id) on delete cascade,
    foreign key (tenant_id) references tenants(id),
    foreign key (user_id) references users(id),
    constraint unique_page_user_draft unique (page_id, user_id)
);

create index idx_page_drafts_user on page_drafts(user_id);

create table if not exists page_reactions (
    id          serial primary key,
    page_id     int not null,
    user_id     int not null,
    emoji       varchar(8) not null,
    created_at  timestamptz not null default now(),
    foreign key (page_id) references pages(id) on delete cascade,
    foreign key (user_id) references users(id) on delete cascade,
    constraint unique_page_reaction unique (page_id, user_id, emoji)
);

create index idx_page_reactions_page on page_reactions(page_id);

alter table comments add column if not exists page_id int;
alter table comments add constraint fk_comments_page_id foreign key (page_id) references pages(id) on delete cascade;

do $$ begin
    alter table comments add constraint check_post_or_page check (
        (post_id is not null and page_id is null) or 
        (post_id is null and page_id is not null)
    );
exception
    when duplicate_object then null;
end $$;

ALTER TABLE comments ALTER COLUMN post_id DROP NOT NULL;
create index if not exists idx_comments_page on comments(page_id) where page_id is not null;

create table if not exists page_subscriptions (
    page_id     int not null,
    user_id     int not null,
    created_at  timestamptz not null default now(),
    primary key (page_id, user_id),
    foreign key (page_id) references pages(id) on delete cascade,
    foreign key (user_id) references users(id) on delete cascade
);