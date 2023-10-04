delete from extension_resource_definitions;
INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000001', NULL, 'User1', 'user1@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');

INSERT INTO extensions (id, name, description, enabled, slug, status) 
VALUES ('00000001-0000-0000-0000-000000000001', 'Test Extension', 'some extension', true, 'test-extension', 'online');

INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
VALUES ('00000001-0000-0000-0000-000000000002', 'Test Resource', 'some-description', true, 'test-resource', 'test-resources', 'v1', 'system',
  '{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb
  '00000001-0000-0000-0000-000000000001'
);


INSERT INTO system_extension_resources (id, resource, extension_resource_definition_id) 
VALUES ( '00000001-0000-0000-0000-000000000003', '{"age": 10, "firstName": "Hello", "lastName": "World"}'::jsonb, '00000001-0000-0000-0000-000000000002');
