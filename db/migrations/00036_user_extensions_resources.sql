-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_extension_resources (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
	resource jsonb NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL,

  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
  extension_resource_definition_id UUID NOT NULL REFERENCES extension_resource_definitions(id) ON DELETE CASCADE ON UPDATE CASCADE,

  INDEX (user_id, extension_resource_definition_id, deleted_at) STORING (resource, created_at, updated_at)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_extension_resources;
-- +goose StatementEnd
