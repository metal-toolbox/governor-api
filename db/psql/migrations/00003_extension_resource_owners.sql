-- +goose Up
-- +goose StatementBegin

ALTER TABLE system_extension_resources ADD owner_id UUID REFERENCES groups(id) ON DELETE SET NULL;
ALTER TABLE system_extension_resources ADD resource_version BIGINT NOT NULL DEFAULT 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE system_extension_resources DROP COLUMN IF EXISTS resource_version;
ALTER TABLE system_extension_resources DROP COLUMN IF EXISTS owner_id;

-- +goose StatementEnd
