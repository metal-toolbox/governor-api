-- +goose Up
-- +goose StatementBegin
CREATE TABLE notification_types (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  description STRING NOT NULL,
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
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT notification_targets_slug_key UNIQUE (slug) WHERE deleted_at IS NULL
);

CREATE TABLE notification_preferences (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  notification_type_id UUID NOT NULL REFERENCES notification_types(id) ON DELETE CASCADE ON UPDATE CASCADE, 
  notification_target_id UUID NOT NULL REFERENCES notification_targets(id) ON DELETE CASCADE ON UPDATE CASCADE,
  PRIMARY KEY (user_id, notification_type_id, notification_target_id),

  enabled BOOL NOT NULL DEFAULT false
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE notification_types;
DROP TABLE notification_targets;
DROP TABLE notification_preferences;
-- +goose StatementEnd
