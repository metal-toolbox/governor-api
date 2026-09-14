-- +goose Up
-- +goose StatementBegin
ALTER TABLE extension_resource_definitions
  ADD COLUMN restrict_read BOOLEAN NOT NULL DEFAULT FALSE
;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE extension_resource_definitions DROP COLUMN IF EXISTS restrict_read;
-- +goose StatementEnd
