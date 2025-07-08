-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  external_id STRING NOT NULL UNIQUE,
  name STRING NOT NULL,
  email STRING NOT NULL UNIQUE,
  login_count INT NOT NULL DEFAULT 0,
  avatar_url STRING NULL,
  last_login_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE groups (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL UNIQUE,
  description STRING NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE group_memberships (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL REFERENCES groups(id),
  user_id UUID NOT NULL REFERENCES users(id),
  is_admin BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE group_membership_requests (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL REFERENCES groups(id),
  user_id UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE audit_events (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  actor_id UUID NOT NULL REFERENCES users(id),
  action STRING NOT NULL,
  message STRING NOT NULL,
  changeset STRING[] NOT NULL DEFAULT ARRAY[],
  subject_group_id UUID NULL REFERENCES groups(id),
  subject_user_id UUID NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE audit_events;
DROP TABLE group_membership_requests;
DROP TABLE group_memberships;
DROP TABLE groups;
DROP TABLE users;
-- +goose StatementEnd
