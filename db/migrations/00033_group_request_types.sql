-- +goose Up
-- +goose StatementBegin
ALTER TABLE group_membership_requests ADD COLUMN IF NOT EXISTS kind STRING NOT NULL DEFAULT 'new_member';
ALTER TABLE group_memberships ADD COLUMN IF NOT EXISTS admin_expires_at TIMESTAMPTZ NULL;
ALTER TABLE group_membership_requests ADD COLUMN IF NOT EXISTS admin_expires_at TIMESTAMPTZ NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE group_membership_requests DROP COLUMN IF EXISTS admin_expires_at;
ALTER TABLE group_memberships DROP COLUMN IF EXISTS admin_expires_at;
ALTER TABLE group_membership_requests DROP COLUMN kind;
-- +goose StatementEnd