-- +goose Up
-- +goose StatementBegin
CREATE TABLE applications (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  slug STRING NOT NULL,
  type STRING NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT applications_slug_key UNIQUE (slug) WHERE deleted_at IS NULL
);

CREATE TABLE group_applications (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL REFERENCES groups(id),
  application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE ON UPDATE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL
);

ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS subject_application_id UUID NULL REFERENCES applications(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_events DROP COLUMN subject_application_id;

DROP TABLE group_applications;

DROP TABLE applications;
-- +goose StatementEnd
