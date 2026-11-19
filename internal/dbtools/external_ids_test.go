//nolint:noctx
package dbtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

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

func TestExternalIDsSuite(t *testing.T) {
	suite.Run(t, new(ExternalIDsTestSuite))
}

func TestExternalIDPairJSON(t *testing.T) {
	b, err := json.Marshal(ExternalIDPair{Iss: "https://idp.example.com", ID: "sub"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"iss":"https://idp.example.com","id":"sub"}`, string(b))
}
