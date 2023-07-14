BEGIN;

-- Create Users
INSERT INTO users (id,external_id,name,email,avatar_url,last_login_at,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000001', 'test1@test.com','test 1','test1@test.com', '', NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO users (id,external_id,name,email,avatar_url,last_login_at,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000002', 'test2@test.com','test 2','test2@test.com', '', NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO users (id,external_id,name,email,avatar_url,last_login_at,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000003', 'test3@test.com','test 3','test3@test.com', '', NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING;

INSERT INTO users (id,name,email,status,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000004', 'pending 1', 'pending-1@test.com', 'pending', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO users (id,name,email,status,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000005', 'pending 2', 'pending-2@test.com', 'pending', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO users (id,name,email,status,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000006', 'pending 3', 'pending-3@test.com', 'pending', NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Create Groups
INSERT INTO groups (id,name,slug,description,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000007','Gophers', 'gophers', 'Group for gophers', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO groups (id,name,slug,description,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000008', 'Taco locos', 'taco-locos', 'Just for taco lovers', NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Create Applications
INSERT INTO applications (id,name,slug,kind,created_at,updated_at,approver_group_id) VALUES ('00000000-0000-0000-0000-000000000009', 'taco logs', 'taco-logs', 'splunk', NOW(), NOW(), (SELECT id FROM groups WHERE slug = 'gophers')) ON CONFLICT DO NOTHING;
INSERT INTO applications (id,name,slug,kind,created_at,updated_at,approver_group_id) VALUES ('00000000-0000-0000-0000-000000000010', 'taco ci', 'taco-ci', 'buildkite', NOW(), NOW(), (SELECT id FROM groups WHERE slug = 'gophers')) ON CONFLICT DO NOTHING;
INSERT INTO applications (id,name,slug,kind,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000011', 'taco chat', 'taco-chat', 'slack', NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Create Orgs
INSERT INTO organizations (id,name,slug,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000012', 'org 1', 'org-1', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO organizations (id,name,slug,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000013', 'org 2', 'org-2', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO organizations (id,name,slug,created_at,updated_at) VALUES ('00000000-0000-0000-0000-000000000014', 'org 3', 'org-3', NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Create Group Memberships
INSERT INTO group_memberships (group_id, user_id, is_admin, created_at, updated_at) VALUES ('00000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000001', false, NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO group_memberships (group_id, user_id, is_admin, created_at, updated_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000001', false, NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO group_memberships (group_id, user_id, is_admin, created_at, updated_at, expires_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000003', false, NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Create Group Memberbership Requests
INSERT INTO group_membership_requests (group_id, user_id, created_at, updated_at) VALUES ('00000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000002', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO group_membership_requests (group_id, user_id, created_at, updated_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000002', NOW(), NOW()) ON CONFLICT DO NOTHING;
INSERT INTO group_membership_requests (group_id, user_id, created_at, updated_at, expires_at) VALUES ('00000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000003', NOW(), NOW(), NOW()) ON CONFLICT DO NOTHING;

-- Create Group Applications
INSERT INTO group_applications (group_id, application_id, created_at, updated_at, deleted_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000009', NOW(), NOW(), NULL);

INSERT INTO group_applications (group_id, application_id, created_at, updated_at, deleted_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000010', NOW(), NOW(), NULL);

INSERT INTO group_applications (group_id, application_id, created_at, updated_at, deleted_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000011', NOW(), NOW(), NULL);


-- Create Group Application Requests
-- INSERT INTO group_application_requests (group_id, application_id, approver_group_id, requester_user_id, note, created_at, updated_at) VALUES ('00000000-0000-0000-0000-000000000007', '00000000-0000-0000-0000-000000000010', 'your governor group id', '00000000-0000-0000-0000-000000000003', 'application request', NOW(), NOW());

-- Create Group Organizations
INSERT INTO group_organizations (group_id, organization_id, created_at, updated_at) VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000012', NOW(), NOW());

COMMIT;