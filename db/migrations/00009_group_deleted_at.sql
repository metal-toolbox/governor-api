-- +goose Up
-- +goose StatementBegin
ALTER TABLE groups ADD COLUMN deleted_at TIMESTAMPTZ NULL;
ALTER TABLE groups DROP COLUMN IF EXISTS is_active;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE groups DROP COLUMN deleted_at;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
-- +goose StatementEnd