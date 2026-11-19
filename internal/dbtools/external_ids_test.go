//nolint:noctx
package dbtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

const (
	extIDUser1   = "00000002-0000-0000-0000-000000000001"
	extIDUser2   = "00000002-0000-0000-0000-000000000002"
	extIDDeleted = "00000002-0000-0000-0000-000000000003"
	extIDIssuerB = "https://other-idp.example.com"
)

type ExternalIDsTestSuite struct {
	suite.Suite

	db *sql.DB
}

func (s *ExternalIDsTestSuite) SetupSuite() {
	s.db = NewPGTestServer(s.T())

	if err := migrateTestDB(s.db); err != nil {
		panic("migration failed - could not set up test db")
	}
}

func (s *ExternalIDsTestSuite) SetupTest() {
	stmts := []string{
		`DELETE FROM audit_events`,
		`DELETE FROM user_external_ids`,
		`DELETE FROM users`,
		`INSERT INTO users (id, name, email, status, created_at, updated_at)
		VALUES ('` + extIDUser1 + `', 'User One', 'one@example.com', 'active', NOW(), NOW())`,
		`INSERT INTO users (id, name, email, status, created_at, updated_at)
		VALUES ('` + extIDUser2 + `', 'User Two', 'two@example.com', 'active', NOW(), NOW())`,
		`INSERT INTO users (id, name, email, status, created_at, updated_at, deleted_at)
		VALUES ('` + extIDDeleted + `', 'Deleted', 'deleted@example.com', 'active', NOW(), NOW(), NOW())`,
		`INSERT INTO user_external_ids (user_id, iss, external_id)
		VALUES ('` + extIDUser1 + `', '` + testIdentityIssuer + `', 'sub-one')`,
		`INSERT INTO user_external_ids (user_id, iss, external_id)
		VALUES ('` + extIDDeleted + `', '` + testIdentityIssuer + `', 'sub-deleted')`,
	}

	for _, q := range stmts {
		_, err := s.db.Exec(q)
		s.Require().NoError(err, q)
	}
}

func (s *ExternalIDsTestSuite) TestFindUserByExternalID() {
	tests := []struct {
		name       string
		iss        string
		externalID string
		wantUserID string
		wantErr    error
	}{
		{
			name:       "matching issuer and external id",
			iss:        testIdentityIssuer,
			externalID: "sub-one",
			wantUserID: extIDUser1,
		},
		{
			name:       "same external id through another issuer",
			iss:        extIDIssuerB,
			externalID: "sub-one",
			wantErr:    sql.ErrNoRows,
		},
		{
			name:       "unknown external id",
			iss:        testIdentityIssuer,
			externalID: "sub-unknown",
			wantErr:    sql.ErrNoRows,
		},
		{
			name:       "soft-deleted user",
			iss:        testIdentityIssuer,
			externalID: "sub-deleted",
			wantErr:    sql.ErrNoRows,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			user, err := FindUserByExternalID(context.Background(), s.db, tt.iss, tt.externalID)

			s.ErrorIs(err, tt.wantErr)
			s.Equal(tt.wantUserID, userID(user))
		})
	}
}

// userID returns the ID of u, or "" when there is no user.
func userID(u *models.User) string {
	if u == nil {
		return ""
	}

	return u.ID
}

func (s *ExternalIDsTestSuite) TestAddUserExternalID() {
	tests := []struct {
		name       string
		userID     string
		iss        string
		externalID string
	}{
		{
			name:       "new record",
			userID:     extIDUser2,
			iss:        testIdentityIssuer,
			externalID: "sub-two",
		},
		{
			name:       "record for a second issuer",
			userID:     extIDUser1,
			iss:        extIDIssuerB,
			externalID: "sub-one-b",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			rec, err := AddUserExternalID(context.Background(), s.db, tt.userID, tt.iss, tt.externalID)
			s.Require().NoError(err)
			s.Equal(
				[]string{tt.userID, tt.iss, tt.externalID},
				[]string{rec.UserID, rec.Iss, rec.ExternalID},
			)

			user, err := FindUserByExternalID(context.Background(), s.db, tt.iss, tt.externalID)
			s.Require().NoError(err)
			s.Equal(tt.userID, user.ID)
		})
	}
}

func (s *ExternalIDsTestSuite) TestAddUserExternalIDConflict() {
	tests := []struct {
		name       string
		userID     string
		iss        string
		externalID string
	}{
		{
			name:       "identity linked to another user",
			userID:     extIDUser2,
			iss:        testIdentityIssuer,
			externalID: "sub-one",
		},
		{
			name:       "second record for the same issuer",
			userID:     extIDUser1,
			iss:        testIdentityIssuer,
			externalID: "sub-one-again",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			_, err := AddUserExternalID(context.Background(), s.db, tt.userID, tt.iss, tt.externalID)
			s.ErrorIs(err, ErrExternalIDConflict)

			var n int
			s.Require().NoError(s.db.QueryRow(`SELECT count(*) FROM user_external_ids`).Scan(&n))
			s.Equal(2, n, "a rejected record must not be written")
		})
	}
}

func (s *ExternalIDsTestSuite) TestAuditUserExternalIDAdded() {
	ctx := context.Background()

	user, err := models.FindUser(ctx, s.db, extIDUser2)
	s.Require().NoError(err)

	event, err := AuditUserExternalIDAdded(ctx, s.db, "10000002-0000-0000-0000-000000000001", user, user,
		ExternalIDPair{Iss: testIdentityIssuer, ID: "sub-two"})
	s.Require().NoError(err)

	stored, err := models.FindAuditEvent(ctx, s.db, event.ID)
	s.Require().NoError(err)

	s.Equal("user.external_id.added", stored.Action)
	s.Equal(extIDUser2, stored.SubjectUserID.String)
	s.Equal(extIDUser2, stored.ActorID.String)
	s.Equal("10000002-0000-0000-0000-000000000001", stored.ParentID.String)
	s.Equal(types.StringArray{"iss: " + testIdentityIssuer, "external_id: sub-two"}, stored.Changeset)
}

func (s *ExternalIDsTestSuite) TestAuditUserExternalIDRemoved() {
	ctx := context.Background()

	user, err := models.FindUser(ctx, s.db, extIDUser1)
	s.Require().NoError(err)

	actor, err := models.FindUser(ctx, s.db, extIDUser2)
	s.Require().NoError(err)

	event, err := AuditUserExternalIDRemoved(ctx, s.db, "10000002-0000-0000-0000-000000000002", actor, user,
		ExternalIDPair{Iss: testIdentityIssuer, ID: "sub-one"})
	s.Require().NoError(err)

	stored, err := models.FindAuditEvent(ctx, s.db, event.ID)
	s.Require().NoError(err)

	s.Equal(
		[]string{"user.external_id.removed", extIDUser1, extIDUser2, "10000002-0000-0000-0000-000000000002"},
		[]string{stored.Action, stored.SubjectUserID.String, stored.ActorID.String, stored.ParentID.String},
	)
	s.Equal(types.StringArray{"iss: " + testIdentityIssuer, "external_id: sub-one"}, stored.Changeset)
}

// storedExternalIDs returns every user_external_ids row, keyed by user ID.
func (s *ExternalIDsTestSuite) storedExternalIDs() map[string][]ExternalIDPair {
	s.T().Helper()

	rows, err := s.db.Query(`SELECT user_id, iss, external_id FROM user_external_ids ORDER BY user_id, iss`)
	s.Require().NoError(err)

	defer rows.Close()

	got := map[string][]ExternalIDPair{}

	for rows.Next() {
		var (
			userID string
			p      ExternalIDPair
		)

		s.Require().NoError(rows.Scan(&userID, &p.Iss, &p.ID))

		got[userID] = append(got[userID], p)
	}

	s.Require().NoError(rows.Err())

	return got
}

func (s *ExternalIDsTestSuite) TestReplaceUserExternalIDs() {
	tests := []struct {
		name        string
		ids         []ExternalIDPair
		wantAdded   []ExternalIDPair
		wantRemoved []ExternalIDPair
		wantStored  map[string][]ExternalIDPair
	}{
		{
			name:        "same set changes nothing",
			ids:         []ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one"}},
			wantAdded:   []ExternalIDPair{},
			wantRemoved: []ExternalIDPair{},
			wantStored: map[string][]ExternalIDPair{
				extIDUser1:   {{Iss: testIdentityIssuer, ID: "sub-one"}},
				extIDDeleted: {{Iss: testIdentityIssuer, ID: "sub-deleted"}},
			},
		},
		{
			name:        "second issuer is added",
			ids:         []ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one"}, {Iss: extIDIssuerB, ID: "sub-one-b"}},
			wantAdded:   []ExternalIDPair{{Iss: extIDIssuerB, ID: "sub-one-b"}},
			wantRemoved: []ExternalIDPair{},
			wantStored: map[string][]ExternalIDPair{
				extIDUser1:   {{Iss: testIdentityIssuer, ID: "sub-one"}, {Iss: extIDIssuerB, ID: "sub-one-b"}},
				extIDDeleted: {{Iss: testIdentityIssuer, ID: "sub-deleted"}},
			},
		},
		{
			name:        "new id for the same issuer replaces the old one",
			ids:         []ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one-new"}},
			wantAdded:   []ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one-new"}},
			wantRemoved: []ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one"}},
			wantStored: map[string][]ExternalIDPair{
				extIDUser1:   {{Iss: testIdentityIssuer, ID: "sub-one-new"}},
				extIDDeleted: {{Iss: testIdentityIssuer, ID: "sub-deleted"}},
			},
		},
		{
			name:        "empty set removes all",
			ids:         []ExternalIDPair{},
			wantAdded:   []ExternalIDPair{},
			wantRemoved: []ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one"}},
			wantStored: map[string][]ExternalIDPair{
				extIDDeleted: {{Iss: testIdentityIssuer, ID: "sub-deleted"}},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			added, removed, err := ReplaceUserExternalIDs(context.Background(), s.db, extIDUser1, tt.ids)
			s.Require().NoError(err)
			s.Equal(tt.wantAdded, added)
			s.Equal(tt.wantRemoved, removed)
			s.Equal(tt.wantStored, s.storedExternalIDs())
		})
	}
}

func (s *ExternalIDsTestSuite) TestReplaceUserExternalIDsConflict() {
	tx, err := s.db.BeginTx(context.Background(), nil)
	s.Require().NoError(err)

	defer func() { _ = tx.Rollback() }()

	_, _, err = ReplaceUserExternalIDs(context.Background(), tx, extIDUser2,
		[]ExternalIDPair{{Iss: testIdentityIssuer, ID: "sub-one"}})
	s.ErrorIs(err, ErrExternalIDConflict)
}

func (s *ExternalIDsTestSuite) TestDeleteUserExternalIDs() {
	s.Require().NoError(DeleteUserExternalIDs(context.Background(), s.db, extIDUser1))

	s.Equal(map[string][]ExternalIDPair{
		extIDDeleted: {{Iss: testIdentityIssuer, ID: "sub-deleted"}},
	}, s.storedExternalIDs())
}

func (s *ExternalIDsTestSuite) TestListUserExternalIDs() {
	_, err := s.db.Exec(`INSERT INTO user_external_ids (user_id, iss, external_id) VALUES ($1, $2, 'sub-one-b')`,
		extIDUser1, extIDIssuerB)
	s.Require().NoError(err)

	tests := []struct {
		name    string
		userIDs []string
		want    map[string][]ExternalIDPair
	}{
		{
			name:    "no users",
			userIDs: nil,
			want:    map[string][]ExternalIDPair{},
		},
		{
			name:    "users with and without records",
			userIDs: []string{extIDUser1, extIDUser2},
			want: map[string][]ExternalIDPair{
				extIDUser1: {{Iss: testIdentityIssuer, ID: "sub-one"}, {Iss: extIDIssuerB, ID: "sub-one-b"}},
				extIDUser2: {},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got, err := ListUserExternalIDs(context.Background(), s.db, tt.userIDs)
			s.Require().NoError(err)
			s.Equal(tt.want, got)
		})
	}
}

func (s *ExternalIDsTestSuite) TestWhereUserHasExternalID() {
	tests := []struct {
		name        string
		externalIDs []string
		want        []string
	}{
		{
			name:        "one external id",
			externalIDs: []string{"sub-one"},
			want:        []string{extIDUser1},
		},
		{
			name:        "external id of a deleted user",
			externalIDs: []string{"sub-deleted"},
			want:        []string{},
		},
		{
			name:        "several external ids",
			externalIDs: []string{"sub-one", "sub-unknown"},
			want:        []string{extIDUser1},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			users, err := models.Users(WhereUserHasExternalID(tt.externalIDs)).All(context.Background(), s.db)
			s.Require().NoError(err)

			got := []string{}
			for _, u := range users {
				got = append(got, u.ID)
			}

			s.Equal(tt.want, got)
		})
	}
}

func TestPrimaryExternalID(t *testing.T) {
	pairs := []ExternalIDPair{{Iss: extIDIssuerB, ID: "sub-b"}, {Iss: testIdentityIssuer, ID: "sub-a"}}

	tests := []struct {
		name    string
		pairs   []ExternalIDPair
		primary string
		want    null.String
	}{
		{name: "record of the primary issuer", pairs: pairs, primary: testIdentityIssuer, want: null.StringFrom("sub-a")},
		{name: "no record of the primary issuer", pairs: pairs[:1], primary: testIdentityIssuer, want: null.String{}},
		{name: "no primary issuer", pairs: pairs, primary: "", want: null.String{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PrimaryExternalID(tt.pairs, tt.primary))
		})
	}
}

func TestExternalIDsSuite(t *testing.T) {
	suite.Run(t, new(ExternalIDsTestSuite))
}

func TestExternalIDPairJSON(t *testing.T) {
	b, err := json.Marshal(ExternalIDPair{Iss: "https://idp.example.com", ID: "sub"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"iss":"https://idp.example.com","id":"sub"}`, string(b))
}
