-- +goose Up
-- +goose StatementBegin
ALTER TABLE audit_events ALTER COLUMN actor_id DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_events ALTER COLUMN actor_id SET NOT NULL;
-- +goose StatementEnd
