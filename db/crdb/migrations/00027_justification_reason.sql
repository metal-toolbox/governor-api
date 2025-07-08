-- +goose Up
-- +goose StatementBegin
ALTER TABLE groups ADD COLUMN IF NOT EXISTS note STRING NOT NULL DEFAULT '';
ALTER TABLE group_membership_requests
    ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS note STRING NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE groups DROP COLUMN IF EXISTS note;
ALTER TABLE group_membership_requests
    DROP COLUMN IF EXISTS  is_admin,
    DROP COLUMN IF EXISTS  note;
-- +goose StatementEnd
