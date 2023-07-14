-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX users_email_key_2 on users (LOWER(email)) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX users_email_key_2 CASCADE;
-- +goose StatementEnd
