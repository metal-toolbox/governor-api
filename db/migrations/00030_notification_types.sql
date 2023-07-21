-- +goose Up
-- +goose StatementBegin
CREATE TABLE notification_types (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  description STRING NOT NULL,
  default_enabled BOOL NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT notification_types_slug_key UNIQUE (slug) WHERE deleted_at IS NULL
);

CREATE TABLE notification_targets (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  description STRING NOT NULL,
  default_enabled BOOL NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT notification_targets_slug_key UNIQUE (slug) WHERE deleted_at IS NULL
);

CREATE TABLE notification_preferences (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  notification_type_id UUID NOT NULL REFERENCES notification_types(id) ON DELETE CASCADE ON UPDATE CASCADE, 
  notification_target_id UUID NULL REFERENCES notification_targets(id) ON DELETE CASCADE ON UPDATE CASCADE,
  notification_target_id_null_string UUID AS (IFNULL(notification_target_id, '00000000-0000-0000-0000-000000000000')) STORED,
  enabled BOOL NOT NULL DEFAULT false,

  CONSTRAINT unique_user_type_target UNIQUE (user_id, notification_type_id, notification_target_id_null_string),
  INDEX (user_id) STORING (notification_type_id, notification_target_id, enabled)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE notification_types;
DROP TABLE notification_targets;
DROP TABLE notification_preferences;
-- +goose StatementEnd
