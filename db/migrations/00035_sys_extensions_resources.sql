-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd
