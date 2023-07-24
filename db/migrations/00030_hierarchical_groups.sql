-- +goose Up
-- +goose StatementBegin
CREATE TABLE group_hierarchies (
    id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
    parent_group_id UUID NOT NULL REFERENCES groups(id),
    member_group_id UUID NOT NULL REFERENCES groups(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NULL
);
CREATE INDEX ON group_hierarchies (member_group_id) STORING (parent_group_id);
CREATE INDEX ON group_hierarchies (parent_group_id) STORING (member_group_id);
CREATE INDEX ON group_memberships (group_id);
CREATE INDEX ON group_memberships (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS group_memberships_user_id_idx;
DROP INDEX IF EXISTS group_memberships_group_id_idx;
DROP TABLE IF EXISTS group_hierarchies;
-- +goose StatementEnd