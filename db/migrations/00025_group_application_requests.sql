-- +goose Up
-- +goose StatementBegin
CREATE TABLE group_application_requests (
    id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE ON UPDATE CASCADE,
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE ON UPDATE CASCADE,
    approver_group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE ON UPDATE CASCADE,
    requester_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
    note STRING(1024) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE group_application_requests;
-- +goose StatementEnd
