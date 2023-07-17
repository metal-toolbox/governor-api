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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE notification_types;
DROP TABLE notification_targets;
-- +goose StatementEnd
