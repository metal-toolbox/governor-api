-- +goose Up
-- +goose StatementBegin
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS slug STRING UNIQUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations DROP COLUMN slug;
-- +goose StatementEnd
