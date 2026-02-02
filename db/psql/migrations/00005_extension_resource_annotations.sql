-- +goose Up
-- +goose StatementBegin
ALTER TABLE system_extension_resources
  ADD COLUMN annotations JSONB NOT NULL DEFAULT '{}'
;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE system_extension_resources DROP COLUMN IF EXISTS annotations;
-- +goose StatementEnd
