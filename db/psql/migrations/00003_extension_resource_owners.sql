-- +goose Up
-- +goose StatementBegin

ALTER TABLE system_extension_resources
  ADD COLUMN owner_id UUID REFERENCES groups(id) ON DELETE SET NULL,
  ADD COLUMN resource_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN messages JSONB[] NOT NULL DEFAULT ARRAY['{}'::JSONB]::JSONB[]
;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE system_extension_resources DROP COLUMN IF EXISTS messages;
ALTER TABLE system_extension_resources DROP COLUMN IF EXISTS resource_version;
ALTER TABLE system_extension_resources DROP COLUMN IF EXISTS owner_id;

-- +goose StatementEnd
