BEGIN;

TRUNCATE group_application_requests, group_applications, applications, application_types, audit_events, group_membership_requests, group_organizations, organizations;


-- don't delete our own admin user/group
DELETE FROM group_memberships WHERE group_id != (SELECT id FROM groups WHERE slug='governor-admins');
DELETE FROM groups WHERE slug != 'governor-admins';
DELETE FROM users WHERE id NOT IN (
    -- don't delete users in governor-admins
    SELECT user_id FROM group_memberships gm
    JOIN groups g ON g.id = gm.group_id
    WHERE g.slug = 'governor-admins'
);

COMMIT;