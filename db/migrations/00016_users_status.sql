-- +goose Up
-- +goose StatementBegin
CREATE TYPE user_status AS ENUM ('pending', 'active');
ALTER TABLE users ADD COLUMN IF NOT EXISTS status user_status DEFAULT 'active';
-- +goose StatementEnd

-- +goose NO TRANSACTION
-- +goose Down
ALTER TABLE users DROP COLUMN status;
DROP TYPE IF EXISTS user_status;
