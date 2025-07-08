-- +goose Up
-- +goose StatementBegin
CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended');

CREATE TABLE public.users (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  external_id TEXT NULL,
  name TEXT NOT NULL,
  email TEXT NOT NULL,
  login_count INT8 NOT NULL DEFAULT 0::INT8,
  avatar_url TEXT NULL,
  last_login_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  github_id INT8 NULL,
  github_username TEXT NULL,
  deleted_at TIMESTAMPTZ NULL,
  status user_status NULL DEFAULT 'active',
  metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
  CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX users_external_id_key_1 ON public.users (external_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_email_key_2 ON public.users (lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX users_name_key ON public.users (name) WHERE deleted_at IS NULL;

CREATE TABLE public.groups (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	description TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ NULL,
	note TEXT NOT NULL DEFAULT '',
	approver_group UUID NULL,
	CONSTRAINT groups_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX groups_slug_key_1 ON public.groups (slug) WHERE deleted_at IS NULL;

CREATE TABLE public.group_memberships (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	group_id UUID NOT NULL,
	user_id UUID NOT NULL,
	is_admin BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NULL,
	admin_expires_at TIMESTAMPTZ NULL,
	CONSTRAINT group_memberships_pkey PRIMARY KEY (id)
);

CREATE INDEX group_memberships_group_id_idx ON public.group_memberships (group_id);
CREATE INDEX group_memberships_user_id_idx ON public.group_memberships (user_id);

CREATE TYPE request_kind AS ENUM ('new_member', 'admin_promotion', 'membership_extension');

CREATE TABLE public.group_membership_requests (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	group_id UUID NOT NULL,
	user_id UUID NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	is_admin BOOLEAN NOT NULL DEFAULT false,
	note TEXT NOT NULL DEFAULT '',
	expires_at TIMESTAMPTZ NULL,
	kind request_kind NOT NULL DEFAULT 'new_member',
	admin_expires_at TIMESTAMPTZ NULL,
	CONSTRAINT group_membership_requests_pkey PRIMARY KEY (id)
);

CREATE TABLE public.organizations (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	slug TEXT NOT NULL,
	deleted_at TIMESTAMPTZ NULL,
	CONSTRAINT organizations_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX organizations_slug_key_1 ON public.organizations (slug) WHERE deleted_at IS NULL;

CREATE TABLE public.application_types (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	description TEXT NOT NULL,
	logo_url TEXT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	CONSTRAINT application_types_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX application_types_slug_key ON public.application_types (slug) WHERE deleted_at IS NULL;

CREATE TABLE public.applications (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	approver_group_id UUID NULL,
	type_id UUID NULL,
	CONSTRAINT applications_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX applications_slug_type_key ON public.applications (slug, type_id) WHERE deleted_at IS NULL;

CREATE TABLE public.audit_events (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	actor_id UUID NULL,
	action TEXT NOT NULL,
	message TEXT NOT NULL,
	changeset TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
	subject_group_id UUID NULL,
	subject_user_id UUID NULL,
	created_at TIMESTAMPTZ NOT NULL,
	subject_organization_id UUID NULL,
	subject_application_id UUID NULL,
	parent_id UUID NULL,
	CONSTRAINT audit_events_pkey PRIMARY KEY (id)
);

CREATE TABLE public.group_organizations (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	group_id UUID NOT NULL,
	organization_id UUID NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT group_organizations_pkey PRIMARY KEY (id)
);

CREATE TABLE public.group_applications (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	group_id UUID NOT NULL,
	application_id UUID NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	CONSTRAINT group_applications_pkey PRIMARY KEY (id)
);

CREATE TABLE public.group_application_requests (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	group_id UUID NOT NULL,
	application_id UUID NOT NULL,
	approver_group_id UUID NOT NULL,
	requester_user_id UUID NOT NULL,
	note TEXT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT group_application_requests_pkey PRIMARY KEY (id)
);

CREATE TABLE public.group_hierarchies (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	parent_group_id UUID NOT NULL,
	member_group_id UUID NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NULL,
	CONSTRAINT group_hierarchies_pkey PRIMARY KEY (id)
);

CREATE INDEX group_hierarchies_member_group_id_idx ON public.group_hierarchies (member_group_id);
CREATE INDEX group_hierarchies_parent_group_id_idx ON public.group_hierarchies (parent_group_id);

CREATE TABLE public.notification_types (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	description TEXT NOT NULL,
	default_enabled BOOLEAN NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	CONSTRAINT notification_types_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX notification_types_slug_key ON public.notification_types (slug) WHERE deleted_at IS NULL;
CREATE INDEX notification_types_deleted_at_idx ON public.notification_types (deleted_at);

CREATE TABLE public.notification_targets (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	description TEXT NOT NULL,
	default_enabled BOOLEAN NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	CONSTRAINT notification_targets_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX notification_targets_slug_key ON public.notification_targets (slug) WHERE deleted_at IS NULL;
CREATE INDEX notification_targets_deleted_at_idx ON public.notification_targets (deleted_at);

CREATE TABLE public.notification_preferences (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	user_id UUID NOT NULL,
	notification_type_id UUID NOT NULL,
	notification_target_id UUID NULL,
	notification_target_id_null_string UUID NULL GENERATED ALWAYS AS (COALESCE(notification_target_id, '00000000-0000-0000-0000-000000000000'::UUID)) STORED,
	enabled BOOLEAN NOT NULL DEFAULT false,
	CONSTRAINT notification_preferences_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX unique_user_type_target ON public.notification_preferences (user_id, notification_type_id, notification_target_id_null_string);
CREATE INDEX notification_preferences_user_id_notification_target_id_notification_type_id_idx ON public.notification_preferences (user_id, notification_target_id, notification_type_id);

CREATE MATERIALIZED VIEW public.notification_defaults AS
SELECT
	targets.id AS target_id,
	targets.slug AS target_slug,
	types.id AS type_id,
	types.slug AS type_slug,
	targets.default_enabled AS default_enabled
FROM
	public.notification_types AS types
	CROSS JOIN public.notification_targets AS targets
WHERE
	(types.deleted_at IS NULL) AND (targets.deleted_at IS NULL)
UNION
SELECT
	'00000000-0000-0000-0000-000000000000'::UUID AS target_id,
	'' AS target_slug,
	types.id AS type_id,
	types.slug AS type_slug,
	types.default_enabled AS default_enabled
FROM
	public.notification_types AS types
WHERE
	types.deleted_at IS NULL;

CREATE TYPE extension_status AS ENUM ('online', 'offline', 'error');

CREATE TABLE public.extensions (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	slug TEXT NOT NULL,
	status extension_status NOT NULL DEFAULT 'offline',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	CONSTRAINT extensions_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX extension_slug_key ON public.extensions (slug) WHERE deleted_at IS NULL;
CREATE INDEX extensions_slug_deleted_at_idx ON public.extensions (slug, deleted_at);
CREATE INDEX extensions_deleted_at_idx ON public.extensions (deleted_at);

CREATE TYPE extension_resource_scope AS ENUM ('system', 'user', 'organization');

CREATE TABLE public.extension_resource_definitions (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT true,
	slug_singular TEXT NOT NULL,
	slug_plural TEXT NOT NULL,
	version TEXT NOT NULL,
	scope extension_resource_scope NOT NULL,
	schema JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	extension_id UUID NOT NULL,
	admin_group UUID NULL,
	CONSTRAINT extension_resource_definitions_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX erd_unique_slug_singular_key ON public.extension_resource_definitions (slug_singular, extension_id, version) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX erd_unique_slug_plural_key ON public.extension_resource_definitions (slug_plural, extension_id, version) WHERE deleted_at IS NULL;
CREATE INDEX extension_resource_definitions_extension_id_deleted_at_idx ON public.extension_resource_definitions (extension_id, deleted_at);

CREATE TABLE public.system_extension_resources (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	resource JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	extension_resource_definition_id UUID NOT NULL,
	CONSTRAINT system_extension_resources_pkey PRIMARY KEY (id)
);

CREATE TABLE public.user_extension_resources (
	id UUID NOT NULL DEFAULT gen_random_uuid(),
	resource JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at TIMESTAMPTZ NULL,
	user_id UUID NOT NULL,
	extension_resource_definition_id UUID NOT NULL,
	CONSTRAINT user_extension_resources_pkey PRIMARY KEY (id)
);

CREATE INDEX user_extension_resources_user_id_extension_resource_definition_id_deleted_at_idx ON public.user_extension_resources (user_id, extension_resource_definition_id, deleted_at);

ALTER TABLE public.groups ADD CONSTRAINT groups_approver_group_fkey FOREIGN KEY (approver_group) REFERENCES public.groups(id);
ALTER TABLE public.group_memberships ADD CONSTRAINT group_memberships_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_memberships ADD CONSTRAINT group_memberships_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE public.group_membership_requests ADD CONSTRAINT group_membership_requests_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_membership_requests ADD CONSTRAINT group_membership_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE public.applications ADD CONSTRAINT applications_approver_group_id_fkey FOREIGN KEY (approver_group_id) REFERENCES public.groups(id);
ALTER TABLE public.applications ADD CONSTRAINT applications_type_id_fkey FOREIGN KEY (type_id) REFERENCES public.application_types(id);
ALTER TABLE public.audit_events ADD CONSTRAINT audit_events_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES public.users(id);
ALTER TABLE public.audit_events ADD CONSTRAINT audit_events_subject_group_id_fkey FOREIGN KEY (subject_group_id) REFERENCES public.groups(id);
ALTER TABLE public.audit_events ADD CONSTRAINT audit_events_subject_user_id_fkey FOREIGN KEY (subject_user_id) REFERENCES public.users(id);
ALTER TABLE public.audit_events ADD CONSTRAINT audit_events_subject_organization_id_fkey FOREIGN KEY (subject_organization_id) REFERENCES public.organizations(id);
ALTER TABLE public.audit_events ADD CONSTRAINT audit_events_subject_application_id_fkey FOREIGN KEY (subject_application_id) REFERENCES public.applications(id);
ALTER TABLE public.group_organizations ADD CONSTRAINT group_organizations_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_organizations ADD CONSTRAINT group_organizations_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.group_applications ADD CONSTRAINT group_applications_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_applications ADD CONSTRAINT group_applications_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.group_application_requests ADD CONSTRAINT group_application_requests_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.groups(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.group_application_requests ADD CONSTRAINT group_application_requests_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.group_application_requests ADD CONSTRAINT group_application_requests_approver_group_id_fkey FOREIGN KEY (approver_group_id) REFERENCES public.groups(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.group_application_requests ADD CONSTRAINT group_application_requests_requester_user_id_fkey FOREIGN KEY (requester_user_id) REFERENCES public.users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.group_hierarchies ADD CONSTRAINT group_hierarchies_parent_group_id_fkey FOREIGN KEY (parent_group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_hierarchies ADD CONSTRAINT group_hierarchies_member_group_id_fkey FOREIGN KEY (member_group_id) REFERENCES public.groups(id);
ALTER TABLE public.notification_preferences ADD CONSTRAINT notification_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.notification_preferences ADD CONSTRAINT notification_preferences_notification_type_id_fkey FOREIGN KEY (notification_type_id) REFERENCES public.notification_types(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.notification_preferences ADD CONSTRAINT notification_preferences_notification_target_id_fkey FOREIGN KEY (notification_target_id) REFERENCES public.notification_targets(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.extension_resource_definitions ADD CONSTRAINT extension_resource_definitions_extension_id_fkey FOREIGN KEY (extension_id) REFERENCES public.extensions(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.extension_resource_definitions ADD CONSTRAINT extension_resource_definitions_admin_group_fkey FOREIGN KEY (admin_group) REFERENCES public.groups(id);
ALTER TABLE public.system_extension_resources ADD CONSTRAINT system_extension_resources_extension_resource_definition_id_fkey FOREIGN KEY (extension_resource_definition_id) REFERENCES public.extension_resource_definitions(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.user_extension_resources ADD CONSTRAINT user_extension_resources_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE public.user_extension_resources ADD CONSTRAINT user_extension_resources_extension_resource_definition_id_fkey FOREIGN KEY (extension_resource_definition_id) REFERENCES public.extension_resource_definitions(id) ON DELETE CASCADE ON UPDATE CASCADE;
-- Validate foreign key constraints. These can fail if there was unvalidated data during the SHOW CREATE ALL TABLES
ALTER TABLE public.groups VALIDATE CONSTRAINT groups_approver_group_fkey;
ALTER TABLE public.group_memberships VALIDATE CONSTRAINT group_memberships_group_id_fkey;
ALTER TABLE public.group_memberships VALIDATE CONSTRAINT group_memberships_user_id_fkey;
ALTER TABLE public.group_membership_requests VALIDATE CONSTRAINT group_membership_requests_group_id_fkey;
ALTER TABLE public.group_membership_requests VALIDATE CONSTRAINT group_membership_requests_user_id_fkey;
ALTER TABLE public.applications VALIDATE CONSTRAINT applications_approver_group_id_fkey;
ALTER TABLE public.applications VALIDATE CONSTRAINT applications_type_id_fkey;
ALTER TABLE public.audit_events VALIDATE CONSTRAINT audit_events_actor_id_fkey;
ALTER TABLE public.audit_events VALIDATE CONSTRAINT audit_events_subject_group_id_fkey;
ALTER TABLE public.audit_events VALIDATE CONSTRAINT audit_events_subject_user_id_fkey;
ALTER TABLE public.audit_events VALIDATE CONSTRAINT audit_events_subject_organization_id_fkey;
ALTER TABLE public.audit_events VALIDATE CONSTRAINT audit_events_subject_application_id_fkey;
ALTER TABLE public.group_organizations VALIDATE CONSTRAINT group_organizations_group_id_fkey;
ALTER TABLE public.group_organizations VALIDATE CONSTRAINT group_organizations_organization_id_fkey;
ALTER TABLE public.group_applications VALIDATE CONSTRAINT group_applications_group_id_fkey;
ALTER TABLE public.group_applications VALIDATE CONSTRAINT group_applications_application_id_fkey;
ALTER TABLE public.group_application_requests VALIDATE CONSTRAINT group_application_requests_group_id_fkey;
ALTER TABLE public.group_application_requests VALIDATE CONSTRAINT group_application_requests_application_id_fkey;
ALTER TABLE public.group_application_requests VALIDATE CONSTRAINT group_application_requests_approver_group_id_fkey;
ALTER TABLE public.group_application_requests VALIDATE CONSTRAINT group_application_requests_requester_user_id_fkey;
ALTER TABLE public.group_hierarchies VALIDATE CONSTRAINT group_hierarchies_parent_group_id_fkey;
ALTER TABLE public.group_hierarchies VALIDATE CONSTRAINT group_hierarchies_member_group_id_fkey;
ALTER TABLE public.notification_preferences VALIDATE CONSTRAINT notification_preferences_user_id_fkey;
ALTER TABLE public.notification_preferences VALIDATE CONSTRAINT notification_preferences_notification_type_id_fkey;
ALTER TABLE public.notification_preferences VALIDATE CONSTRAINT notification_preferences_notification_target_id_fkey;
ALTER TABLE public.extension_resource_definitions VALIDATE CONSTRAINT extension_resource_definitions_extension_id_fkey;
ALTER TABLE public.extension_resource_definitions VALIDATE CONSTRAINT extension_resource_definitions_admin_group_fkey;
ALTER TABLE public.system_extension_resources VALIDATE CONSTRAINT system_extension_resources_extension_resource_definition_id_fkey;
ALTER TABLE public.user_extension_resources VALIDATE CONSTRAINT user_extension_resources_user_id_fkey;
ALTER TABLE public.user_extension_resources VALIDATE CONSTRAINT user_extension_resources_extension_resource_definition_id_fkey;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
