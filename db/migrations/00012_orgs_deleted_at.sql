-- +goose Up
-- +goose StatementBegin
ALTER TABLE organizations ADD COLUMN deleted_at TIMESTAMPTZ NULL;
ALTER TABLE organizations ALTER COLUMN slug SET NOT NULL;
-- +goose StatementEnd
​
-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations DROP COLUMN deleted_at TIMESTAMPTZ NULL;
ALTER TABLE organizations ALTER COLUMN slug DROP NOT NULL;
-- +goose StatementEnd
