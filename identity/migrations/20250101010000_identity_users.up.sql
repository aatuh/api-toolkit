create table if not exists identity_users (
  id uuid primary key,
  identity_provider text not null,
  identity_subject text not null,
  email text not null,
  first_name text not null default '',
  last_name text not null default '',
  preferred_language text not null default 'en',
  created_at timestamptz not null,
  updated_at timestamptz not null,
  constraint identity_users_provider_subject_uniq unique (identity_provider, identity_subject)
);

create index if not exists identity_users_subject_idx on identity_users (identity_subject);
create index if not exists identity_users_created_idx on identity_users (created_at desc);

create table if not exists identity_user_roles (
  user_id uuid not null references identity_users(id) on delete cascade,
  role text not null,
  created_at timestamptz not null,
  constraint identity_user_roles_uniq unique (user_id, role)
);

create index if not exists identity_user_roles_user_idx on identity_user_roles (user_id);
