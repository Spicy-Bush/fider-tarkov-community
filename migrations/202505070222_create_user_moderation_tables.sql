create table if not exists user_warnings (
  id          serial primary key,
  user_id     int not null,
  tenant_id   int not null,
  reason      text not null,
  created_at  timestamptz not null default now(),
  expires_at  timestamptz null,
  created_by  int not null,
  foreign key (user_id) references users(id),
  foreign key (tenant_id) references tenants(id),
  foreign key (created_by) references users(id)
);

create table if not exists user_mutes (
  id          serial primary key,
  user_id     int not null,
  tenant_id   int not null,
  reason      text not null,
  created_at  timestamptz not null default now(),
  expires_at  timestamptz null,
  created_by  int not null,
  foreign key (user_id) references users(id),
  foreign key (tenant_id) references tenants(id),
  foreign key (created_by) references users(id)
); 

CREATE INDEX IF NOT EXISTS idx_posts_user_tenant_status ON posts (user_id, tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_comments_user_tenant_deleted ON comments (user_id, tenant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_post_votes_user_tenant ON post_votes (user_id, tenant_id);

CREATE INDEX IF NOT EXISTS idx_user_warnings_user_tenant_expires ON user_warnings (user_id, tenant_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_user_mutes_user_tenant_expires ON user_mutes (user_id, tenant_id, expires_at);

CREATE INDEX IF NOT EXISTS idx_users_status_id ON users (status, id);