-- +goose Up
-- +goose StatementBegin

-- Users table
CREATE TYPE user_status AS ENUM ('pending', 'active', 'suspended');
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

CREATE UNIQUE INDEX users_external_id_key_1 ON public.users (external_id ASC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_email_key_2 ON public.users (lower(email) ASC) WHERE deleted_at IS NULL;
CREATE INDEX users_name_key ON public.users (name ASC) WHERE deleted_at IS NULL;

-- Groups table
CREATE TABLE public.groups (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  description TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ NULL,
  note TEXT NOT NULL DEFAULT ''::TEXT,
  approver_group UUID NULL,
  CONSTRAINT groups_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX groups_slug_key_1 ON public.groups (slug ASC) WHERE deleted_at IS NULL;

-- Group memberships table
CREATE TABLE public.group_memberships (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL,
  user_id UUID NOT NULL,
  is_admin BOOL NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  admin_expires_at TIMESTAMPTZ NULL,
  CONSTRAINT group_memberships_pkey PRIMARY KEY (id)
);

CREATE INDEX group_memberships_group_id_idx ON public.group_memberships (group_id ASC);
CREATE INDEX group_memberships_user_id_idx ON public.group_memberships (user_id ASC);

-- Group membership requests table
CREATE TYPE request_kind AS ENUM ('new_member', 'admin_promotion');
CREATE TABLE public.group_membership_requests (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL,
  user_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  is_admin BOOL NOT NULL DEFAULT false,
  note TEXT NOT NULL DEFAULT ''::TEXT,
  expires_at TIMESTAMPTZ NULL,
  kind request_kind NOT NULL DEFAULT 'new_member',
  admin_expires_at TIMESTAMPTZ NULL,
  CONSTRAINT group_membership_requests_pkey PRIMARY KEY (id)
);

-- Organizations table
CREATE TABLE public.organizations (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  slug TEXT NOT NULL,
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT organizations_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX organizations_slug_key_1 ON public.organizations (slug ASC) WHERE deleted_at IS NULL;

-- Application types table
CREATE TABLE public.application_types (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  description TEXT NOT NULL,
  logo_url TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT application_types_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX application_types_slug_key ON public.application_types (slug ASC) WHERE deleted_at IS NULL;

-- Applications table
CREATE TABLE public.applications (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  approver_group_id UUID NULL,
  type_id UUID NULL,
  CONSTRAINT applications_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX applications_slug_type_key ON public.applications (slug ASC, type_id ASC) WHERE deleted_at IS NULL;

-- Audit events table
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

-- Group organizations table
CREATE TABLE public.group_organizations (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL,
  organization_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  CONSTRAINT group_organizations_pkey PRIMARY KEY (id)
);

-- Group applications table
CREATE TABLE public.group_applications (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL,
  application_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT group_applications_pkey PRIMARY KEY (id)
);

-- Group application requests table
CREATE TABLE public.group_application_requests (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL,
  application_id UUID NOT NULL,
  approver_group_id UUID NOT NULL,
  requester_user_id UUID NOT NULL,
  note varchar(1024) NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  CONSTRAINT group_application_requests_pkey PRIMARY KEY (id)
);

-- Group hierarchies table
CREATE TABLE public.group_hierarchies (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  parent_group_id UUID NOT NULL,
  member_group_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  CONSTRAINT group_hierarchies_pkey PRIMARY KEY (id)
);

CREATE INDEX group_hierarchies_member_group_id_idx ON public.group_hierarchies (member_group_id ASC) INCLUDE (parent_group_id);
CREATE INDEX group_hierarchies_parent_group_id_idx ON public.group_hierarchies (parent_group_id ASC) INCLUDE (member_group_id);

-- Notification types table
CREATE TABLE public.notification_types (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  description TEXT NOT NULL,
  default_enabled BOOL NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT notification_types_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX notification_types_slug_key ON public.notification_types (slug ASC) WHERE deleted_at IS NULL;
CREATE INDEX notification_types_deleted_at_idx ON public.notification_types (deleted_at ASC) INCLUDE (slug, default_enabled);

-- Notification targets table
CREATE TABLE public.notification_targets (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  description TEXT NOT NULL,
  default_enabled BOOL NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT notification_targets_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX notification_targets_slug_key ON public.notification_targets (slug ASC) WHERE deleted_at IS NULL;
CREATE INDEX notification_targets_deleted_at_idx ON public.notification_targets (deleted_at ASC) INCLUDE (slug, default_enabled);

-- Notification preferences table
CREATE TABLE public.notification_preferences (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  notification_type_id UUID NOT NULL,
  notification_target_id UUID NULL,
  notification_target_id_null_string UUID NULL GENERATED ALWAYS AS (COALESCE(notification_target_id, '00000000-0000-0000-0000-000000000000'::UUID)) STORED,
  enabled BOOL NOT NULL DEFAULT false,
  CONSTRAINT notification_preferences_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX unique_user_type_target ON public.notification_preferences (user_id ASC, notification_type_id ASC, notification_target_id_null_string ASC);
CREATE INDEX notification_preferences_user_id_notification_target_id_notification_type_id_idx ON public.notification_preferences (user_id ASC, notification_target_id ASC, notification_type_id ASC) INCLUDE (enabled, notification_target_id_null_string);

-- Notification defaults materialized view
CREATE MATERIALIZED VIEW public.notification_defaults (
  target_id,
  target_slug,
  type_id,
  type_slug,
  default_enabled
) AS SELECT
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
(
  SELECT
    '00000000-0000-0000-0000-000000000000'::UUID AS target_id,
    '' AS target_slug,
    types.id AS type_id,
    types.slug AS type_slug,
    types.default_enabled AS default_enabled
  FROM
    public.notification_types AS types
  WHERE
    types.deleted_at IS NULL
);

-- Extensions table
CREATE TYPE extension_status AS ENUM ('online', 'offline');
CREATE TABLE public.extensions (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  enabled BOOL NOT NULL,
  slug TEXT NOT NULL,
  status extension_status NOT NULL DEFAULT 'offline',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  CONSTRAINT extensions_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX extension_slug_key ON public.extensions (slug ASC) WHERE deleted_at IS NULL;
CREATE INDEX extensions_slug_deleted_at_idx ON public.extensions (slug ASC, deleted_at ASC) INCLUDE (name, description, enabled, status, created_at, updated_at);
CREATE INDEX extensions_deleted_at_idx ON public.extensions (deleted_at ASC) INCLUDE (name, description, enabled, slug, status, created_at, updated_at);

-- Extension resource definitions table
CREATE TYPE extension_resource_scope AS ENUM ('user', 'system');

CREATE TABLE public.extension_resource_definitions (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  enabled BOOL NOT NULL DEFAULT true,
  slug_singular TEXT NOT NULL,
  slug_plural TEXT NOT NULL,
  version TEXT NOT NULL,
  scope extension_resource_scope NOT NULL,
  schema JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  extension_id UUID NOT NULL,
  admin_group UUID NULL,
  CONSTRAINT extension_resource_definitions_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX erd_unique_slug_singular_key ON public.extension_resource_definitions (slug_singular ASC, extension_id ASC, version ASC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX erd_unique_slug_plural_key ON public.extension_resource_definitions (slug_plural ASC, extension_id ASC, version ASC) WHERE deleted_at IS NULL;
CREATE INDEX extension_resource_definitions_extension_id_deleted_at_idx ON public.extension_resource_definitions (extension_id ASC, deleted_at ASC) INCLUDE (name, description, enabled, slug_singular, slug_plural, version, scope, schema, created_at, updated_at);

-- System extension resources table
CREATE TABLE public.system_extension_resources (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  resource JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  extension_resource_definition_id UUID NOT NULL,
  CONSTRAINT system_extension_resources_pkey PRIMARY KEY (id)
);

-- User extension resources table
CREATE TABLE public.user_extension_resources (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  resource JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()::TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ NULL,
  user_id UUID NOT NULL,
  extension_resource_definition_id UUID NOT NULL,
  CONSTRAINT user_extension_resources_pkey PRIMARY KEY (id)
);

CREATE INDEX user_extension_resources_user_id_extension_resource_definition_id_deleted_at_idx ON public.user_extension_resources (user_id ASC, extension_resource_definition_id ASC, deleted_at ASC) INCLUDE (resource, created_at, updated_at);

-- Foreign key constraints
ALTER TABLE public.groups ADD CONSTRAINT groups_approver_group_fkey FOREIGN KEY (approver_group) REFERENCES public.groups(id);
ALTER TABLE public.group_memberships ADD CONSTRAINT fk_group_id_ref_groups FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_memberships ADD CONSTRAINT fk_user_id_ref_users FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE public.group_membership_requests ADD CONSTRAINT fk_group_id_ref_groups FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_membership_requests ADD CONSTRAINT fk_user_id_ref_users FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE public.applications ADD CONSTRAINT applications_approver_group_id_fkey FOREIGN KEY (approver_group_id) REFERENCES public.groups(id);
ALTER TABLE public.applications ADD CONSTRAINT applications_type_id_fkey FOREIGN KEY (type_id) REFERENCES public.application_types(id);
ALTER TABLE public.audit_events ADD CONSTRAINT fk_actor_id_ref_users FOREIGN KEY (actor_id) REFERENCES public.users(id);
ALTER TABLE public.audit_events ADD CONSTRAINT fk_subject_group_id_ref_groups FOREIGN KEY (subject_group_id) REFERENCES public.groups(id);
ALTER TABLE public.audit_events ADD CONSTRAINT fk_subject_user_id_ref_users FOREIGN KEY (subject_user_id) REFERENCES public.users(id);
ALTER TABLE public.audit_events ADD CONSTRAINT fk_subject_organization_id_ref_organizations FOREIGN KEY (subject_organization_id) REFERENCES public.organizations(id);
ALTER TABLE public.audit_events ADD CONSTRAINT audit_events_subject_application_id_fkey FOREIGN KEY (subject_application_id) REFERENCES public.applications(id);
ALTER TABLE public.group_organizations ADD CONSTRAINT fk_group_id_ref_groups FOREIGN KEY (group_id) REFERENCES public.groups(id);
ALTER TABLE public.group_organizations ADD CONSTRAINT fk_organization_id_ref_organizations FOREIGN KEY (organization_id) REFERENCES public.organizations(id) ON DELETE CASCADE ON UPDATE CASCADE;
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

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop foreign key constraints
ALTER TABLE public.user_extension_resources DROP CONSTRAINT user_extension_resources_extension_resource_definition_id_fkey;
ALTER TABLE public.user_extension_resources DROP CONSTRAINT user_extension_resources_user_id_fkey;
ALTER TABLE public.system_extension_resources DROP CONSTRAINT system_extension_resources_extension_resource_definition_id_fkey;
ALTER TABLE public.extension_resource_definitions DROP CONSTRAINT extension_resource_definitions_admin_group_fkey;
ALTER TABLE public.extension_resource_definitions DROP CONSTRAINT extension_resource_definitions_extension_id_fkey;
ALTER TABLE public.notification_preferences DROP CONSTRAINT notification_preferences_notification_target_id_fkey;
ALTER TABLE public.notification_preferences DROP CONSTRAINT notification_preferences_notification_type_id_fkey;
ALTER TABLE public.notification_preferences DROP CONSTRAINT notification_preferences_user_id_fkey;
ALTER TABLE public.group_hierarchies DROP CONSTRAINT group_hierarchies_member_group_id_fkey;
ALTER TABLE public.group_hierarchies DROP CONSTRAINT group_hierarchies_parent_group_id_fkey;
ALTER TABLE public.group_application_requests DROP CONSTRAINT group_application_requests_requester_user_id_fkey;
ALTER TABLE public.group_application_requests DROP CONSTRAINT group_application_requests_approver_group_id_fkey;
ALTER TABLE public.group_application_requests DROP CONSTRAINT group_application_requests_application_id_fkey;
ALTER TABLE public.group_application_requests DROP CONSTRAINT group_application_requests_group_id_fkey;
ALTER TABLE public.group_applications DROP CONSTRAINT group_applications_application_id_fkey;
ALTER TABLE public.group_applications DROP CONSTRAINT group_applications_group_id_fkey;
ALTER TABLE public.group_organizations DROP CONSTRAINT fk_organization_id_ref_organizations;
ALTER TABLE public.group_organizations DROP CONSTRAINT fk_group_id_ref_groups;
ALTER TABLE public.audit_events DROP CONSTRAINT audit_events_subject_application_id_fkey;
ALTER TABLE public.audit_events DROP CONSTRAINT fk_subject_organization_id_ref_organizations;
ALTER TABLE public.audit_events DROP CONSTRAINT fk_subject_user_id_ref_users;
ALTER TABLE public.audit_events DROP CONSTRAINT fk_subject_group_id_ref_groups;
ALTER TABLE public.audit_events DROP CONSTRAINT fk_actor_id_ref_users;
ALTER TABLE public.applications DROP CONSTRAINT applications_type_id_fkey;
ALTER TABLE public.applications DROP CONSTRAINT applications_approver_group_id_fkey;
ALTER TABLE public.group_membership_requests DROP CONSTRAINT fk_user_id_ref_users;
ALTER TABLE public.group_membership_requests DROP CONSTRAINT fk_group_id_ref_groups;
ALTER TABLE public.group_memberships DROP CONSTRAINT fk_user_id_ref_users;
ALTER TABLE public.group_memberships DROP CONSTRAINT fk_group_id_ref_groups;
ALTER TABLE public.groups DROP CONSTRAINT groups_approver_group_fkey;

-- Drop indexes
DROP INDEX user_extension_resources_user_id_extension_resource_definition_id_deleted_at_idx;
DROP INDEX extension_resource_definitions_extension_id_deleted_at_idx;
DROP INDEX erd_unique_slug_plural_key;
DROP INDEX erd_unique_slug_singular_key;
DROP INDEX extensions_deleted_at_idx;
DROP INDEX extensions_slug_deleted_at_idx;
DROP INDEX extension_slug_key;
DROP INDEX notification_preferences_user_id_notification_target_id_notification_type_id_idx;
DROP INDEX unique_user_type_target;
DROP INDEX notification_targets_deleted_at_idx;
DROP INDEX notification_targets_slug_key;
DROP INDEX notification_types_deleted_at_idx;
DROP INDEX notification_types_slug_key;
DROP INDEX group_hierarchies_parent_group_id_idx;
DROP INDEX group_hierarchies_member_group_id_idx;
DROP INDEX applications_slug_type_key;
DROP INDEX application_types_slug_key;
DROP INDEX organizations_slug_key_1;
DROP INDEX group_memberships_user_id_idx;
DROP INDEX group_memberships_group_id_idx;
DROP INDEX groups_slug_key_1;
DROP INDEX users_name_key;
DROP INDEX users_email_key_2;
DROP INDEX users_external_id_key_1;

-- Drop materialized view
DROP MATERIALIZED VIEW public.notification_defaults;

-- Drop tables
DROP TABLE public.user_extension_resources;
DROP TABLE public.system_extension_resources;
DROP TABLE public.extension_resource_definitions;
DROP TABLE public.extensions;
DROP TABLE public.notification_preferences;
DROP TABLE public.notification_targets;
DROP TABLE public.notification_types;
DROP TABLE public.group_hierarchies;
DROP TABLE public.group_application_requests;
DROP TABLE public.group_applications;
DROP TABLE public.group_organizations;
DROP TABLE public.audit_events;
DROP TABLE public.applications;
DROP TABLE public.application_types;
DROP TABLE public.organizations;
DROP TABLE public.group_membership_requests;
DROP TABLE public.group_memberships;
DROP TABLE public.groups;
DROP TABLE public.users;

-- Drop types
DROP TYPE extension_resource_scope;
DROP TYPE extension_status;
DROP TYPE request_kind;
DROP TYPE user_status;

-- +goose StatementEnd
