-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS public.users_external_id_key_1;
ALTER TABLE public.users DROP COLUMN external_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The column comes back empty: the down step of 00007 refills it from the
-- primary identity provider's user_external_ids records.
ALTER TABLE public.users ADD COLUMN external_id TEXT NULL;
CREATE UNIQUE INDEX users_external_id_key_1 ON public.users (external_id ASC) WHERE deleted_at IS NULL;
-- +goose StatementEnd
