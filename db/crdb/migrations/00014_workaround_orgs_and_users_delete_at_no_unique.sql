-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS organizations_slug_key CASCADE;
ALTER TABLE organizations ADD CONSTRAINT IF NOT EXISTS organizations_slug_key_1 UNIQUE (slug) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS users_email_key CASCADE;
ALTER TABLE users ADD CONSTRAINT IF NOT EXISTS users_email_key_1 UNIQUE (email) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS users_external_id_key CASCADE;
ALTER TABLE users ADD CONSTRAINT IF NOT EXISTS users_external_id_key_1 UNIQUE (external_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX organizations_slug_key_1 CASCADE; 
ALTER TABLE organizations ADD CONSTRAINT organizations_slug_key UNIQUE (slug);
DROP INDEX users_email_key_1 CASCADE;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
DROP INDEX users_external_id_key_1 CASCADE; 
ALTER TABLE users ADD CONSTRAINT users_external_id_key UNIQUE (external_id);
-- +goose StatementEnd
