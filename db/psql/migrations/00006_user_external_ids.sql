-- +goose Up
-- +goose StatementBegin
CREATE TABLE public.user_external_ids (
  id          UUID        NOT NULL DEFAULT gen_random_uuid(),
  user_id     UUID        NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  iss         TEXT        NOT NULL,
  external_id TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT user_external_ids_pkey PRIMARY KEY (id),
  CONSTRAINT user_external_ids_iss_external_id_key UNIQUE (iss, external_id),
  CONSTRAINT user_external_ids_user_id_iss_key UNIQUE (user_id, iss),
  CONSTRAINT user_external_ids_iss_len CHECK (char_length(iss) BETWEEN 1 AND 512),
  CONSTRAINT user_external_ids_external_id_len CHECK (char_length(external_id) BETWEEN 1 AND 256)
);
CREATE INDEX user_external_ids_user_id_idx ON public.user_external_ids (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS public.user_external_ids;
-- +goose StatementEnd
