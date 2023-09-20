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

  url STRING NOT NULL,
	status extension_status NOT NULL DEFAULT 'online',

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  CONSTRAINT extension_slug_key UNIQUE (slug) WHERE deleted_at IS NULL
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
  CONSTRAINT erd_unique_key UNIQUE (slug_singular) WHERE deleted_at IS NULL
);

CREATE TABLE user_extension_resources (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
	resource jsonb NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  extension_resource_definition_id UUID NOT NULL REFERENCES extension_resource_definitions(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE system_extension_resources (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
	resource jsonb NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  extension_resource_definition_id UUID NOT NULL REFERENCES extension_resource_definitions(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE system_extension_resources;
DROP TABLE user_extension_resources;
DROP TABLE extension_resource_definitions;
DROP TABLE extensions;

DROP TYPE extension_resource_scope;
DROP TYPE extension_status;
-- +goose StatementEnd
