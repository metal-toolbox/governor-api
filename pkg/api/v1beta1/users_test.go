//nolint:noctx
package v1beta1

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aarondl/null/v8"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	dbm "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

const (
	// testIdentityIssuer is the one identity provider the test database is
	// migrated with, and the primary identity issuer of the test router.
	testIdentityIssuer = "https://idp.example.com"
	testPingIssuer     = "https://ping.example.com"

	deprecationLink = `<https://github.com/metal-toolbox/governor-api/blob/main/docs/user-external-ids.md>; rel="deprecation"`

	usersAliceID = "00000005-0000-0000-0000-000000000001"
	usersBobID   = "00000005-0000-0000-0000-000000000002"
)

type UsersTestSuite struct {
	suite.Suite

	db *sql.DB
}

func (s *UsersTestSuite) SetupSuite() {
	s.db = dbtools.NewPGTestServer(s.T())

	p, err := dbm.NewProvider(s.db, []string{testIdentityIssuer})
	s.Require().NoError(err)

	_, err = p.Up(context.Background())
	s.Require().NoError(err)

	stmts := []string{
		`INSERT INTO users (id, name, email, status, created_at, updated_at)
		VALUES ('` + usersAliceID + `', 'Alice', 'alice@example.com', 'active', NOW(), NOW())`,
		`INSERT INTO users (id, name, email, status, created_at, updated_at)
		VALUES ('` + usersBobID + `', 'Bob', 'bob@example.com', 'active', NOW(), NOW())`,
		`INSERT INTO user_external_ids (user_id, iss, external_id)
		VALUES ('` + usersAliceID + `', '` + testIdentityIssuer + `', 'okta-alice')`,
		`INSERT INTO user_external_ids (user_id, iss, external_id)
		VALUES ('` + usersAliceID + `', '` + testPingIssuer + `', 'ping-alice')`,
		`INSERT INTO user_external_ids (user_id, iss, external_id)
		VALUES ('` + usersBobID + `', '` + testPingIssuer + `', 'ping-bob')`,
	}

	for _, q := range stmts {
		_, err := s.db.Exec(q)
		s.Require().NoError(err, q)
	}
}

func (s *UsersTestSuite) router() *Router {
	return &Router{
		AuthConf: []configs.Auth{
			{AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: testIdentityIssuer}, Identity: configs.IdentityConfig{Enabled: true}},
			{AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: testPingIssuer}, Identity: configs.IdentityConfig{Enabled: true}},
		},
		DB:                    sqlx.NewDb(s.db, "postgres"),
		Logger:                zap.NewNop(),
		PrimaryIdentityIssuer: testIdentityIssuer,
	}
}

// listedUser is the part of a listed user that these tests check.
type listedUser struct {
	ExternalIDs []dbtools.ExternalID
	ExternalID  null.String
}

// listUsers runs listUsers with query and returns the status, the error
// message, the listed users by email, and the Deprecation and Link headers.
func (s *UsersTestSuite) listUsers(query string) (int, string, map[string]listedUser, []string) {
	s.T().Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1beta1/users?"+query, nil)

	s.router().listUsers(c)

	if w.Code != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}

		s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &e))

		return w.Code, e.Error, nil, nil
	}

	var resp PaginationResponse[*User]
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))

	got := map[string]listedUser{}
	for _, u := range resp.Records {
		got[u.Email] = listedUser{ExternalIDs: u.ExternalIDs, ExternalID: u.ExternalID}
	}

	return w.Code, "", got, []string{w.Header().Get("Deprecation"), w.Header().Get("Link")}
}

var (
	listedAlice = listedUser{
		ExternalIDs: []dbtools.ExternalID{{Iss: testIdentityIssuer, ID: "okta-alice"}, {Iss: testPingIssuer, ID: "ping-alice"}},
		ExternalID:  null.StringFrom("okta-alice"),
	}
	listedBob = listedUser{
		ExternalIDs: []dbtools.ExternalID{{Iss: testPingIssuer, ID: "ping-bob"}},
	}
)

func (s *UsersTestSuite) TestListUsersExternalIDs() {
	tests := []struct {
		name  string
		query string
		want  map[string]listedUser
	}{
		{
			name:  "every user has its identities",
			query: "",
			want:  map[string]listedUser{"alice@example.com": listedAlice, "bob@example.com": listedBob},
		},
		{
			name:  "deprecated external id filter matches any issuer",
			query: "external_id=ping-bob",
			want:  map[string]listedUser{"bob@example.com": listedBob},
		},
		{
			name:  "deprecated external id filter with several values",
			query: "external_id=okta-alice&external_id=ping-bob",
			want:  map[string]listedUser{"alice@example.com": listedAlice, "bob@example.com": listedBob},
		},
		{
			name:  "deprecated external id filter without a match",
			query: "external_id=unknown",
			want:  map[string]listedUser{},
		},
		{
			name:  "external id at its issuer",
			query: "external_id=ping-bob&iss=" + url.QueryEscape(testPingIssuer),
			want:  map[string]listedUser{"bob@example.com": listedBob},
		},
		{
			name:  "external id at another issuer",
			query: "external_id=ping-bob&iss=" + url.QueryEscape(testIdentityIssuer),
			want:  map[string]listedUser{},
		},
		{
			name:  "users with a record at an issuer",
			query: "iss=" + url.QueryEscape(testIdentityIssuer),
			want:  map[string]listedUser{"alice@example.com": listedAlice},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			code, msg, got, headers := s.listUsers(tt.query)
			s.Equal(http.StatusOK, code, msg)
			s.Equal(tt.want, got)
			s.Equal([]string{"true", deprecationLink}, headers)
		})
	}
}

func (s *UsersTestSuite) TestListUsersUnknownIssuer() {
	tests := []struct {
		name  string
		query string
	}{
		{name: "issuer alone", query: "iss=" + url.QueryEscape("https://x.example.com")},
		{name: "issuer with an external id", query: "external_id=x&iss=" + url.QueryEscape("https://x.example.com")},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			code, msg, _, _ := s.listUsers(tt.query)
			s.Equal(http.StatusUnprocessableEntity, code)
			s.Equal("invalid iss: not a configured issuer", msg)
		})
	}
}

func (s *UsersTestSuite) TestListUsersDatabaseError() {
	tests := []struct {
		name  string
		query string
		setup func(t *testing.T)
	}{
		{
			// the cursor is compared after the count, so only the users query fails
			name:  "users query fails",
			query: "next_cursor=not-a-uuid",
			setup: func(*testing.T) {},
		},
		{
			name:  "external ids query fails",
			setup: s.hideExternalIDsTable,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setup(s.T())

			code, msg, _, _ := s.listUsers(tt.query)
			s.Equal(http.StatusBadRequest, code)
			s.True(strings.HasPrefix(msg, "error fetching users: "), msg)
		})
	}
}

// hideExternalIDsTable renames user_external_ids until the test ends, so that
// every query of it fails.
func (s *UsersTestSuite) hideExternalIDsTable(t *testing.T) {
	t.Helper()

	_, err := s.db.Exec(`ALTER TABLE user_external_ids RENAME TO user_external_ids_hidden`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err := s.db.Exec(`ALTER TABLE user_external_ids_hidden RENAME TO user_external_ids`)
		require.NoError(t, err)
	})
}

func TestUsersSuite(t *testing.T) {
	suite.Run(t, new(UsersTestSuite))
}
