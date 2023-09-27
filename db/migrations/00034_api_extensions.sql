-- +goose Up
-- +goose StatementBegin
CREATE TYPE extension_status AS ENUM('online', 'offline');
CREATE TYPE extension_resource_scope AS ENUM('user', 'system');

CREATE TABLE extensions (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  description STRING NOT NULL,
  enabled BOOL NOT NULL DEFAULT true,
  slug STRING NOT NULL,

	status extension_status NOT NULL DEFAULT 'online',

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  CONSTRAINT extension_slug_key UNIQUE (slug) WHERE deleted_at IS NULL,
  INDEX (slug, deleted_at) STORING (name, description, enabled, status, created_at, updated_at),
  INDEX (deleted_at) STORING (name, description, enabled, slug, status, created_at, updated_at)
);

CREATE TABLE extension_resource_definitions (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  description STRING NOT NULL,
  enabled BOOL NOT NULL DEFAULT true,

  slug_singular STRING NOT NULL,
  slug_plural STRING NOT NULL,
  version STRING NOT NULL,
	scope extension_resource_scope NOT NULL,
	schema JSONB NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

	extension_id UUID NOT NULL REFERENCES extensions(id) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT erd_unique_slug_singular_key UNIQUE (slug_singular, version) WHERE deleted_at IS NULL,
  CONSTRAINT erd_unique_slug_plural_key UNIQUE (slug_plural, version) WHERE deleted_at IS NULL,

  INDEX (extension_id, deleted_at) STORING (
    name, description, enabled, slug_singular, slug_plural, version, scope, schema, created_at, updated_at
  )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE extension_resource_definitions;
DROP TABLE extensions;

DROP TYPE extension_resource_scope;
DROP TYPE extension_status;
-- +goose StatementEnd
