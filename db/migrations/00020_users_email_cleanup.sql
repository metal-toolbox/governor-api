-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS users_email_key_1 CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users ADD CONSTRAINT IF NOT EXISTS users_email_key_1 UNIQUE (email) WHERE deleted_at IS NULL;
-- +goose StatementEnd
