-- +goose Up
-- +goose StatementBegin
CREATE TABLE application_types (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  description STRING NOT NULL,
  logo_url STRING NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT application_types_slug_key UNIQUE (slug) WHERE deleted_at IS NULL
);

-- we allow type_id to be NULL for now so it works with existing applications
ALTER TABLE applications
  RENAME COLUMN type TO kind,
  ADD COLUMN IF NOT EXISTS type_id UUID NULL REFERENCES application_types(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE applications
  DROP COLUMN IF EXISTS type_id,
  RENAME COLUMN kind TO type;

DROP TABLE application_types;
-- +goose StatementEnd
