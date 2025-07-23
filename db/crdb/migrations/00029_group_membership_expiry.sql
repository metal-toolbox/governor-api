-- +goose Up
-- +goose StatementBegin
ALTER TABLE group_memberships ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;
ALTER TABLE group_membership_requests ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE group_memberships DROP COLUMN IF EXISTS expires_at;
ALTER TABLE group_membership_requests DROP COLUMN IF EXISTS expires_at;
-- +goose StatementEnd
