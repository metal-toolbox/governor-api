-- +goose Up
-- +goose StatementBegin
ALTER TABLE extension_resource_definitions ADD COLUMN IF NOT EXISTS admin_group UUID NULL REFERENCES groups(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE extension_resource_definitions DROP COLUMN IF EXISTS admin_group;
-- +goose StatementEnd
