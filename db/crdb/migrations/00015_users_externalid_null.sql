-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN external_id DROP NOT NULL;
ALTER TABLE users ALTER COLUMN last_login_at DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN external_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN last_login_at SET NOT NULL;
-- +goose StatementEnd
