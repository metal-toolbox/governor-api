-- +goose Up
-- +goose StatementBegin
CREATE TABLE notification_types (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  description STRING NOT NULL,
  default_enabled BOOL NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  CONSTRAINT notification_types_slug_key UNIQUE (slug) WHERE deleted_at IS NULL,
  INDEX (deleted_at) STORING (slug, default_enabled)
);

CREATE TABLE notification_targets (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  description STRING NOT NULL,
  default_enabled BOOL NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  CONSTRAINT notification_targets_slug_key UNIQUE (slug) WHERE deleted_at IS NULL,
  INDEX (deleted_at) STORING (slug, default_enabled)
);

CREATE TABLE notification_preferences (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  notification_type_id UUID NOT NULL REFERENCES notification_types(id) ON DELETE CASCADE ON UPDATE CASCADE, 
  notification_target_id UUID NULL REFERENCES notification_targets(id) ON DELETE CASCADE ON UPDATE CASCADE,
  notification_target_id_null_string UUID AS (IFNULL(notification_target_id, '00000000-0000-0000-0000-000000000000')) STORED,
  enabled BOOL NOT NULL DEFAULT false,

  CONSTRAINT unique_user_type_target UNIQUE (user_id, notification_type_id, notification_target_id_null_string),
  INDEX (user_id) STORING (notification_type_id, notification_target_id, notification_target_id_null_string, enabled),
  INDEX (user_id, notification_target_id, notification_type_id) STORING (enabled)
);

CREATE MATERIALIZED VIEW notification_defaults AS 
SELECT
  targets.id as target_id,
  targets.slug as target_slug,
  types.id as type_id,
  types.slug as type_slug,
  targets.default_enabled as default_enabled
FROM
  notification_types as types
CROSS JOIN notification_targets as targets
WHERE types.deleted_at IS NULL AND targets.deleted_at IS NULL
UNION (
  SELECT
    '00000000-0000-0000-0000-000000000000' as target_id,
    '' as target_slug,
    types.id as type_id,
    types.slug as type_slug,
    types.default_enabled as default_enabled
  FROM
    notification_types as types
  WHERE types.deleted_at IS NULL
);

CREATE INDEX ON notification_defaults (target_id, type_id) STORING (target_slug, type_slug, default_enabled);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE notification_types;
DROP TABLE notification_targets;
DROP TABLE notification_preferences;
DROP MATERIALIZED VIEW notification_defaults;
-- +goose StatementEnd
