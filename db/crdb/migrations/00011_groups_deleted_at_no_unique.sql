-- +goose Up
-- +goose StatementBegin
DROP INDEX groups_slug_key CASCADE;
ALTER TABLE groups ADD CONSTRAINT groups_slug_key_1 unique (slug) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX groups_slug_key_1 CASCADE;
ALTER TABLE groups ADD CONSTRAINT groups_slug_key UNIQUE (slug);
-- +goose StatementEnd
