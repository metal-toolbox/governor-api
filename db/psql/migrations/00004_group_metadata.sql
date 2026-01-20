-- +goose Up
-- +goose StatementBegin
ALTER TABLE groups ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE groups DROP COLUMN IF EXISTS metadata;
-- +goose StatementEnd
