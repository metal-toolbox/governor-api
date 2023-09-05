-- +goose Up
-- +goose StatementBegin
INSERT INTO application_types (id, name, slug, description, logo_url, created_at, updated_at, deleted_at)
SELECT
	gen_random_uuid (),
	kind,
	kind,
	'',
	'',
	NOW(),
	NOW(),
	NULL
FROM
	applications
GROUP BY
	kind
ON CONFLICT DO NOTHING;

UPDATE
	applications
SET
	type_id = application_types.id
FROM
	application_types
WHERE
	applications.kind = application_types.slug;

ALTER TABLE applications DROP COLUMN kind;

DROP INDEX IF EXISTS applications_slug_key CASCADE;

ALTER TABLE applications ADD CONSTRAINT applications_slug_type_key UNIQUE (slug, type_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS applications_slug_type_key CASCADE;

ALTER TABLE applications ADD CONSTRAINT applications_slug_key UNIQUE (slug) WHERE deleted_at IS NULL;

ALTER TABLE applications ADD COLUMN IF NOT EXISTS kind STRING;

UPDATE
	applications
SET
	kind = application_types.slug
FROM
	application_types
WHERE
	applications.type_id = application_types.id;

ALTER TABLE applications ALTER COLUMN kind SET NOT NULL;

-- not deleting from application_types here because there isn't a good way to know if manual changes have been made
-- to the rows and the above transaction should be replayable anyway

-- +goose StatementEnd
