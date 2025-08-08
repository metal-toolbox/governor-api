-- +goose Up
-- +goose StatementBegin

-- Add unique constraint on group_membership_requests for a given user+group, so that there is only one pending request for a user-group combination at a time.

ALTER TABLE group_membership_requests ADD CONSTRAINT group_membership_requests_unique_user_group UNIQUE (user_id, group_id) ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE group_membership_requests DROP CONSTRAINT IF EXISTS group_membership_requests_unique_user_group ;

-- +goose StatementEnd
