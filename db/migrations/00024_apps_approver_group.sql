-- +goose Up
-- +goose StatementBegin
ALTER TABLE applications ADD COLUMN IF NOT EXISTS approver_group_id UUID NULL REFERENCES groups(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE applications DROP COLUMN IF EXISTS approver_group_id;
-- +goose StatementEnd
