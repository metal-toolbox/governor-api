-- +goose Up
-- +goose StatementBegin
CREATE TABLE organizations (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE group_organizations (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL REFERENCES groups(id),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE ON UPDATE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS subject_organization_id UUID NULL REFERENCES organizations(id);

ALTER TABLE groups DROP COLUMN IF EXISTS github_org;

ALTER TABLE groups DROP COLUMN IF EXISTS github_team;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_events DROP COLUMN subject_organization_id;

ALTER TABLE groups ADD COLUMN IF NOT EXISTS github_org STRING NOT NULL DEFAULT '';

ALTER TABLE groups ADD COLUMN IF NOT EXISTS github_team STRING NOT NULL DEFAULT '';

DROP TABLE group_organizations;

DROP TABLE organizations;
-- +goose StatementEnd
