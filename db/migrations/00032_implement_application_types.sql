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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE applications ADD COLUMN IF NOT EXISTS kind STRING NOT NULL;

UPDATE
	applications
SET
	kind = application_types.slug
FROM
	application_types
WHERE
	applications.type_id = application_types.id;
-- +goose StatementEnd
