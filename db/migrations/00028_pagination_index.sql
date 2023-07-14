-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS users_name_key ON users (name) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_name_key CASCADE;
-- +goose StatementEnd
