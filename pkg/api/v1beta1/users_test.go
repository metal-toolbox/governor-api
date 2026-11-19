//nolint:noctx
package v1beta1

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/null/v8"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
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
	ExternalIDs []dbtools.ExternalIDPair
	ExternalID  null.String
}

// listUsers runs listUsers with query and returns the status, the error
// message, and the listed users by email.
func (s *UsersTestSuite) listUsers(query string) (int, string, map[string]listedUser, string) {
	s.T().Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1beta1/users?"+query, nil)

	s.router().listUsers(c)

	if w.Code != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}

		s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &e))

		return w.Code, e.Error, nil, ""
	}

	var resp PaginationResponse[*User]
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))

	got := map[string]listedUser{}
	for _, u := range resp.Records {
		got[u.Email] = listedUser{ExternalIDs: u.ExternalIDs, ExternalID: u.ExternalID}
	}

	return w.Code, "", got, w.Header().Get("Deprecation")
}

var (
	listedAlice = listedUser{
		ExternalIDs: []dbtools.ExternalIDPair{{Iss: testIdentityIssuer, ID: "okta-alice"}, {Iss: testPingIssuer, ID: "ping-alice"}},
		ExternalID:  null.StringFrom("okta-alice"),
	}
	listedBob = listedUser{
		ExternalIDs: []dbtools.ExternalIDPair{{Iss: testPingIssuer, ID: "ping-bob"}},
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
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			code, msg, got, deprecation := s.listUsers(tt.query)
			s.Equal(http.StatusOK, code, msg)
			s.Equal(tt.want, got)
			s.Equal("true", deprecation)
		})
	}
}

func TestUsersSuite(t *testing.T) {
	suite.Run(t, new(UsersTestSuite))
}
