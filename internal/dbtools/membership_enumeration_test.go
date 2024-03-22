package dbtools

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	dbm "github.com/metal-toolbox/governor-api/db"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/volatiletech/null/v8"
)

var db *sql.DB

func init() {
	ts, err := testserver.NewTestServer()
	if err != nil {
		panic(err)
	}

	db, err = sql.Open("postgres", ts.PGURL().String())
	if err != nil {
		panic(err)
	}

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(db, "migrations"); err != nil {
		panic("migration failed - could not set up test db")
	}

	err = seedTestDB(db)
	if err != nil {
		panic("db setup failed - could not seed test db")
	}
}

func TestServerRunning(t *testing.T) {
	var c int

	r, err := db.Query("SELECT COUNT(*) AS c FROM users;")
	if err != nil {
		t.Fatal("could not query test database", err)
	}

	r.Next()

	scan := r.Scan(&c)

	assert.NoError(t, scan)
	assert.Equal(t, c, 5)
}

func TestGetMembershipsForUser(t *testing.T) {
	testCases := map[string][]EnumeratedMembership{
		"00000001-0000-0000-0000-000000000001": {
			{
				UserID:    "00000001-0000-0000-0000-000000000001",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   true,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
		},
		"00000001-0000-0000-0000-000000000002": {
			{
				UserID:    "00000001-0000-0000-0000-000000000002",
				GroupID:   "00000002-0000-0000-0000-000000000002",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000002",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
		},
		"00000001-0000-0000-0000-000000000003": {
			{
				UserID:    "00000001-0000-0000-0000-000000000003",
				GroupID:   "00000002-0000-0000-0000-000000000003",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000003",
				GroupID:   "00000002-0000-0000-0000-000000000002",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000003",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
		},
		"00000001-0000-0000-0000-000000000004": {
			{
				UserID:    "00000001-0000-0000-0000-000000000004",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000004",
				GroupID:   "00000002-0000-0000-0000-000000000003",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000004",
				GroupID:   "00000002-0000-0000-0000-000000000002",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
		},
		"00000001-0000-0000-0000-000000000005": {},
	}

	for user, expect := range testCases {
		t.Run(fmt.Sprintf("groups for user: %s", user), func(t *testing.T) {
			enumeratedMemberships, err := GetMembershipsForUser(context.TODO(), db, user, false)

			if assert.NoError(t, err) {
				assert.True(t, assert.ObjectsAreEqualValues(enumeratedMemberships, expect))
			}
		})
	}
}

func TestGetMembersOfGroup(t *testing.T) {
	testCases := map[string][]EnumeratedMembership{
		"00000002-0000-0000-0000-000000000003": {
			{
				UserID:    "00000001-0000-0000-0000-000000000003",
				GroupID:   "00000002-0000-0000-0000-000000000003",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000004",
				GroupID:   "00000002-0000-0000-0000-000000000003",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
		},
		"00000002-0000-0000-0000-000000000002": {
			{
				UserID:    "00000001-0000-0000-0000-000000000002",
				GroupID:   "00000002-0000-0000-0000-000000000002",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000003",
				GroupID:   "00000002-0000-0000-0000-000000000002",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000004",
				GroupID:   "00000002-0000-0000-0000-000000000002",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
		},
		"00000002-0000-0000-0000-000000000001": {
			{
				UserID:    "00000001-0000-0000-0000-000000000001",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   true,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000002",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000003",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   false,
				Direct:    false,
				ExpiresAt: null.Time{},
			},
			{
				UserID:    "00000001-0000-0000-0000-000000000004",
				GroupID:   "00000002-0000-0000-0000-000000000001",
				IsAdmin:   false,
				Direct:    true,
				ExpiresAt: null.Time{},
			},
		},
	}

	for user, expect := range testCases {
		t.Run(fmt.Sprintf("users for group: %s", user), func(t *testing.T) {
			enumeratedMemberships, err := GetMembersOfGroup(context.TODO(), db, user, false)

			if assert.NoError(t, err) {
				assert.True(t, assert.ObjectsAreEqualValues(enumeratedMemberships, expect))
			}
		})
	}
}

func TestHierarchyWouldCreateCycle(t *testing.T) {
	type testCase struct {
		parent string
		member string
	}

	testCases := map[testCase]bool{
		{parent: "00000002-0000-0000-0000-000000000001", member: "00000002-0000-0000-0000-000000000002"}: false,
		{parent: "00000002-0000-0000-0000-000000000002", member: "00000002-0000-0000-0000-000000000003"}: false,
		{parent: "00000002-0000-0000-0000-000000000003", member: "00000002-0000-0000-0000-000000000001"}: true,
		{parent: "00000002-0000-0000-0000-000000000003", member: "00000002-0000-0000-0000-000000000002"}: true,
		{parent: "00000002-0000-0000-0000-000000000003", member: "00000002-0000-0000-0000-000000000003"}: true,
		{parent: "00000002-0000-0000-0000-000000000005", member: "00000002-0000-0000-0000-000000000002"}: false, // tests that cycle detection ignores deleted groups
	}

	for test, expect := range testCases {
		t.Run(fmt.Sprintf("test for cycle: %s member of %s", test.member, test.parent), func(t *testing.T) {
			result, err := HierarchyWouldCreateCycle(context.TODO(), db, test.parent, test.member)

			assert.NoError(t, err)

			assert.Equal(t, expect, result)
		})
	}
}

// nolint:all
// Sets this up:
//                                        ┌──────┐
//                                    ┌───┤Group1│
//                                    │   └─┬─┬──┘
//                                    ▼     │ │
//                                ┌──────┐  │ ▼
//          ┌─────────────────┬───┤Group2│  │User1
//          │                 │   └───┬──┘  │
//          ▼                 ▼       │     │
// ┌────────────────┐     ┌──────┐    ▼     │
// │Group4 (Deleted)│     │Group3│   User2  │
// └───┬────────┬───┘     └┬──┬──┘          │
//     │        │          │  │             │
//     ▼        ▼          │  ▼             │
// ┌──────┐   User5        │ User3          ▼
// │Group5│                └────────────► User4
// └──────┘

func seedTestDB(db *sql.DB) error {
	testData := []string{
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000001', NULL, 'User1', 'user1@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'pending');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000002', NULL, 'User2', 'user2@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'pending');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000003', NULL, 'User3', 'user3@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'pending');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000004', NULL, 'User4', 'user4@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'pending');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000001-0000-0000-0000-000000000005', NULL, 'User5', 'user5@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'pending');`,

		`INSERT INTO "groups" ("id", "name", "slug", "description", "created_at", "updated_at", "deleted_at", "note") VALUES
		('00000002-0000-0000-0000-000000000001', 'Group1', 'group-1', 'group-1', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, '');`,
		`INSERT INTO "groups" ("id", "name", "slug", "description", "created_at", "updated_at", "deleted_at", "note") VALUES
		('00000002-0000-0000-0000-000000000002', 'Group2', 'group-2', 'group-2', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, '');`,
		`INSERT INTO "groups" ("id", "name", "slug", "description", "created_at", "updated_at", "deleted_at", "note") VALUES
		('00000002-0000-0000-0000-000000000003', 'Group3', 'group-3', 'group-3', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, '');`,
		`INSERT INTO "groups" ("id", "name", "slug", "description", "created_at", "updated_at", "deleted_at", "note") VALUES
		('00000002-0000-0000-0000-000000000004', 'Group4', 'group-4', 'group-4', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', '');`,
		`INSERT INTO "groups" ("id", "name", "slug", "description", "created_at", "updated_at", "deleted_at", "note") VALUES
		('00000002-0000-0000-0000-000000000005', 'Group5', 'group-5', 'group-5', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, '');`,

		`INSERT INTO "group_memberships" ("id", "group_id", "user_id", "is_admin", "created_at", "updated_at", "expires_at") VALUES
		('00000003-0000-0000-0000-000000000001', '00000002-0000-0000-0000-000000000001', '00000001-0000-0000-0000-000000000001', 't', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_memberships" ("id", "group_id", "user_id", "is_admin", "created_at", "updated_at", "expires_at") VALUES
		('00000003-0000-0000-0000-000000000002', '00000002-0000-0000-0000-000000000002', '00000001-0000-0000-0000-000000000002', 'f', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_memberships" ("id", "group_id", "user_id", "is_admin", "created_at", "updated_at", "expires_at") VALUES
		('00000003-0000-0000-0000-000000000003', '00000002-0000-0000-0000-000000000003', '00000001-0000-0000-0000-000000000003', 'f', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_memberships" ("id", "group_id", "user_id", "is_admin", "created_at", "updated_at", "expires_at") VALUES
		('00000003-0000-0000-0000-000000000004', '00000002-0000-0000-0000-000000000001', '00000001-0000-0000-0000-000000000004', 'f', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_memberships" ("id", "group_id", "user_id", "is_admin", "created_at", "updated_at", "expires_at") VALUES
		('00000003-0000-0000-0000-000000000005', '00000002-0000-0000-0000-000000000003', '00000001-0000-0000-0000-000000000004', 'f', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_memberships" ("id", "group_id", "user_id", "is_admin", "created_at", "updated_at", "expires_at") VALUES
		('00000003-0000-0000-0000-000000000006', '00000002-0000-0000-0000-000000000004', '00000001-0000-0000-0000-000000000005', 'f', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,

		`INSERT INTO "group_hierarchies" ("id", "parent_group_id", "member_group_id", "created_at", "updated_at", "expires_at") VALUES
		('00000004-0000-0000-0000-000000000001', '00000002-0000-0000-0000-000000000001', '00000002-0000-0000-0000-000000000002', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_hierarchies" ("id", "parent_group_id", "member_group_id", "created_at", "updated_at", "expires_at") VALUES
		('00000004-0000-0000-0000-000000000002', '00000002-0000-0000-0000-000000000002', '00000002-0000-0000-0000-000000000003', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_hierarchies" ("id", "parent_group_id", "member_group_id", "created_at", "updated_at", "expires_at") VALUES
		('00000004-0000-0000-0000-000000000003', '00000002-0000-0000-0000-000000000002', '00000002-0000-0000-0000-000000000004', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
		`INSERT INTO "group_hierarchies" ("id", "parent_group_id", "member_group_id", "created_at", "updated_at", "expires_at") VALUES
		('00000004-0000-0000-0000-000000000004', '00000002-0000-0000-0000-000000000004', '00000002-0000-0000-0000-000000000005', '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL);`,
	}
	for _, q := range testData {
		_, err := db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}
