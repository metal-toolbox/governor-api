-- +goose Up
-- +goose StatementBegin
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS parent_id UUID NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_events DROP COLUMN parent_id;
-- +goose StatementEnd
