-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_pubkeys (
  id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  pubkey BYTEA NOT NULL,
  fingerprint STRING NOT NULL,
  comment STRING NOT NULL, 
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_pubkeys;

-- +goose StatementEnd
