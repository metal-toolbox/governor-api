-- +goose Up
-- +goose NO TRANSACTION
ALTER TYPE user_status ADD VALUE 'suspended';

-- +goose NO TRANSACTION
-- +goose Down
ALTER TYPE user_status DROP VALUE 'suspended';
