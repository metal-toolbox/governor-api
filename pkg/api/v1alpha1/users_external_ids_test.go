//nolint:noctx
package v1alpha1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aarondl/null/v8"
	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	"github.com/metal-toolbox/governor-api/pkg/configs"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

const (
	usersPingIssuer    = "https://ping.example.com"
	usersTestUserID    = "00000001-0000-0000-0000-000000000001"
	usersMetadataUser  = "00000001-0000-0000-0000-000000000003"
	usersDeletedUserID = "00000001-0000-0000-0000-000000000002"
	usersUnknownUserID = "00000001-0000-0000-0000-000000000099"
	deprecationLink    = `<https://github.com/metal-toolbox/governor-api/blob/main/docs/multi-idp-user-identities-lld.md>; rel="deprecation"`
)

var (
	metadataUserPairs = []dbtools.ExternalIDPair{
		{Iss: testIdentityIssuer, ID: "okta-metadata"},
		{Iss: usersPingIssuer, ID: "ping-metadata"},
	}
	noPairs = []dbtools.ExternalIDPair{}
)

// identityRouter returns a Router with the test issuer as primary identity
// provider and a second, Ping, provider. primary overrides the primary
// identity issuer; "" means that no provider has identity.enabled.
func (s *UserTestSuite) identityRouter(primary string) *Router {
	r := s.v1alpha1()
	r.AuthConf = []configs.Auth{
		{AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: testIdentityIssuer}, Identity: configs.IdentityConfig{Enabled: primary != ""}},
		{AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: usersPingIssuer}},
	}
	r.PrimaryIdentityIssuer = primary

	return r
}

// resetExternalIDs reseeds the users and gives the metadata user an Okta and
// a Ping identity.
func (s *UserTestSuite) resetExternalIDs() {
	s.T().Helper()

	s.SetupTest()

	for _, p := range metadataUserPairs {
		_, err := s.db.Exec(`INSERT INTO user_external_ids (user_id, iss, external_id) VALUES ($1, $2, $3)`,
			usersMetadataUser, p.Iss, p.ID)
		s.Require().NoError(err)
	}

	s.conn.Subject, s.conn.Payload, s.conn.err = "", nil, nil
}

// serveUserRequest runs handler for one request and returns the recorder.
func serveUserRequest(handler gin.HandlerFunc, method, id, body string, setup ...func(*gin.Context)) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest(method, "/api/v1alpha1/users", io.NopCloser(bytes.NewBufferString(body)))
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Set(ginaudit.AuditIDContextKey, "10000001-0000-0000-0000-000000000001")

	for _, f := range setup {
		f(c)
	}

	handler(c)

	return w
}

// identityResponse is the part of a user response this file checks.
type identityResponse struct {
	Email       string
	Status      string
	ExternalIDs []dbtools.ExternalIDPair
	ExternalID  null.String
	Deprecation string
	Link        string
}

func decodeIdentity(t *testing.T, w *httptest.ResponseRecorder) identityResponse {
	t.Helper()

	var u User
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &u), w.Body.String())

	return identityResponse{
		Email:       u.Email,
		Status:      u.Status.String,
		ExternalIDs: u.ExternalIDs,
		ExternalID:  u.ExternalID,
		Deprecation: w.Header().Get("Deprecation"),
		Link:        w.Header().Get("Link"),
	}
}

// storedPairs returns the user_external_ids records of the user with email.
func (s *UserTestSuite) storedPairs(email string) []dbtools.ExternalIDPair {
	s.T().Helper()

	rows, err := s.db.Query(`SELECT uei.iss, uei.external_id FROM user_external_ids uei
		JOIN users u ON u.id = uei.user_id WHERE u.email = $1 ORDER BY uei.iss`, email)
	s.Require().NoError(err)

	defer rows.Close()

	got := []dbtools.ExternalIDPair{}

	for rows.Next() {
		var p dbtools.ExternalIDPair

		s.Require().NoError(rows.Scan(&p.Iss, &p.ID))

		got = append(got, p)
	}

	s.Require().NoError(rows.Err())

	return got
}

// auditActions counts the audit events by action.
func (s *UserTestSuite) auditActions() map[string]int {
	s.T().Helper()

	evs, err := models.AuditEvents().All(s.T().Context(), s.db)
	s.Require().NoError(err)

	got := map[string]int{}
	for _, e := range evs {
		got[e.Action]++
	}

	return got
}

// publishedAction returns the action of the last published event, or "".
func (s *UserTestSuite) publishedAction() string {
	s.T().Helper()

	if s.conn.Payload == nil {
		return ""
	}

	var ev events.Event
	s.Require().NoError(json.Unmarshal(s.conn.Payload, &ev))

	return ev.Action
}

func (s *UserTestSuite) TestCreateUserExternalIDs() {
	tests := []struct {
		name      string
		payload   string
		wantResp  identityResponse
		wantAudit map[string]int
	}{
		{
			name: "external ids at two issuers",
			payload: `{"name": "New User", "email": "new@example.com", "external_ids": [
				{"iss": "` + testIdentityIssuer + `", "id": "okta-new"}, {"iss": "` + usersPingIssuer + `", "id": "ping-new"}]}`,
			wantResp: identityResponse{
				Email:       "new@example.com",
				Status:      UserStatusActive,
				ExternalIDs: []dbtools.ExternalIDPair{{Iss: testIdentityIssuer, ID: "okta-new"}, {Iss: usersPingIssuer, ID: "ping-new"}},
				ExternalID:  null.StringFrom("okta-new"),
				Deprecation: "true",
				Link:        deprecationLink,
			},
			wantAudit: map[string]int{"user.created": 1, "user.external_id.added": 2},
		},
		{
			name:    "deprecated external id maps to the primary issuer",
			payload: `{"name": "New User", "email": "new@example.com", "external_id": "okta-new"}`,
			wantResp: identityResponse{
				Email:       "new@example.com",
				Status:      UserStatusActive,
				ExternalIDs: []dbtools.ExternalIDPair{{Iss: testIdentityIssuer, ID: "okta-new"}},
				ExternalID:  null.StringFrom("okta-new"),
				Deprecation: "true",
				Link:        deprecationLink,
			},
			wantAudit: map[string]int{"user.created": 1, "user.external_id.added": 1},
		},
		{
			name:    "a user without external ids is pending",
			payload: `{"name": "New User", "email": "new@example.com"}`,
			wantResp: identityResponse{
				Email:       "new@example.com",
				Status:      UserStatusPending,
				ExternalIDs: noPairs,
				Deprecation: "true",
				Link:        deprecationLink,
			},
			wantAudit: map[string]int{"user.created": 1},
		},
		{
			name:    "an empty external ids array is pending",
			payload: `{"name": "New User", "email": "new@example.com", "external_ids": []}`,
			wantResp: identityResponse{
				Email:       "new@example.com",
				Status:      UserStatusPending,
				ExternalIDs: noPairs,
				Deprecation: "true",
				Link:        deprecationLink,
			},
			wantAudit: map[string]int{"user.created": 1},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.resetExternalIDs()

			w := serveUserRequest(s.identityRouter(testIdentityIssuer).createUser, http.MethodPost, "", tt.payload)
			s.Require().Equal(http.StatusAccepted, w.Code, w.Body.String())

			s.Equal(tt.wantResp, decodeIdentity(s.T(), w))
			s.Equal(tt.wantResp.ExternalIDs, s.storedPairs("new@example.com"))
			s.Equal(tt.wantAudit, s.auditActions())
		})
	}
}

// externalIDsPayload returns an external_ids array with n entries.
func externalIDsPayload(n int) string {
	entries := make([]string, n)
	for i := range entries {
		entries[i] = `{"iss": "` + testIdentityIssuer + `", "id": "id-` + string(rune('a'+i)) + `"}`
	}

	return "[" + strings.Join(entries, ",") + "]"
}

func (s *UserTestSuite) TestCreateUserExternalIDsRejected() {
	tests := []struct {
		name       string
		primary    string
		externalID string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "both fields",
			primary:    testIdentityIssuer,
			externalID: `"external_id": "okta-new", "external_ids": []`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "use either external_id or external_ids, not both",
		},
		{
			name:       "deprecated external id without an identity provider",
			externalID: `"external_id": "okta-new"`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "external_id needs an oidc provider with identity.enabled; use external_ids",
		},
		{
			name:       "issuer that is not configured",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "https://x.example.com", "id": "x"}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: iss https://x.example.com is not a configured issuer",
		},
		{
			name:       "issuer that is not an http url",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "ftp://idp.example.com", "id": "x"}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: iss ftp://idp.example.com is not an absolute http or https URL",
		},
		{
			name:       "issuer longer than 512 characters",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "https://` + strings.Repeat("a", 512) + `", "id": "x"}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: iss is longer than 512 characters",
		},
		{
			name:       "empty id",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "` + testIdentityIssuer + `", "id": ""}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: id for iss " + testIdentityIssuer + " must be 1 to 256 characters",
		},
		{
			name:       "id longer than 256 characters",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "` + testIdentityIssuer + `", "id": "` + strings.Repeat("a", 257) + `"}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: id for iss " + testIdentityIssuer + " must be 1 to 256 characters",
		},
		{
			name:       "id with surrounding spaces",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "` + testIdentityIssuer + `", "id": " x "}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: id for iss " + testIdentityIssuer + " has leading or trailing spaces",
		},
		{
			name:       "deprecated external id with surrounding spaces",
			primary:    testIdentityIssuer,
			externalID: `"external_id": " x "`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: id for iss " + testIdentityIssuer + " has leading or trailing spaces",
		},
		{
			name:    "same issuer twice",
			primary: testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "` + testIdentityIssuer + `", "id": "a"},
				{"iss": "` + testIdentityIssuer + `", "id": "b"}]`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: iss " + testIdentityIssuer + " appears more than once",
		},
		{
			name:       "more than 10 entries",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": ` + externalIDsPayload(11),
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: at most 10 entries are allowed",
		},
		{
			name:       "identity of another user",
			primary:    testIdentityIssuer,
			externalID: `"external_ids": [{"iss": "` + usersPingIssuer + `", "id": "ping-metadata"}]`,
			wantStatus: http.StatusConflict,
			wantErr:    "external id already linked to another user",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.resetExternalIDs()

			payload := `{"name": "New User", "email": "new@example.com", ` + tt.externalID + `}`

			w := serveUserRequest(s.identityRouter(tt.primary).createUser, http.MethodPost, "", payload)

			s.Equal(tt.wantStatus, w.Code)
			s.JSONEq(`{"error": "`+tt.wantErr+`"}`, w.Body.String())

			exists, err := models.Users(models.UserWhere.Email.EQ("new@example.com")).Exists(s.T().Context(), s.db)
			s.Require().NoError(err)
			s.False(exists, "a rejected user must not be created")
			s.Equal(map[string]int{}, s.auditActions())
			s.Equal(metadataUserPairs, s.storedPairs("metadata@example.com"))
		})
	}
}

func (s *UserTestSuite) TestUpdateUserExternalIDs() {
	tests := []struct {
		name      string
		userID    string
		email     string
		payload   string
		wantPairs []dbtools.ExternalIDPair
		wantExtID null.String
		wantAudit map[string]int
		wantEvent string
	}{
		{
			name:      "replacing the set drops the primary identity",
			userID:    usersMetadataUser,
			email:     "metadata@example.com",
			payload:   `{"external_ids": [{"iss": "` + usersPingIssuer + `", "id": "ping-metadata"}]}`,
			wantPairs: []dbtools.ExternalIDPair{{Iss: usersPingIssuer, ID: "ping-metadata"}},
			wantAudit: map[string]int{"user.external_id.removed": 1},
			wantEvent: events.GovernorEventUpdate,
		},
		{
			name:   "the same set changes nothing",
			userID: usersMetadataUser,
			email:  "metadata@example.com",
			payload: `{"external_ids": [{"iss": "` + usersPingIssuer + `", "id": "ping-metadata"},
				{"iss": "` + testIdentityIssuer + `", "id": "okta-metadata"}]}`,
			wantPairs: metadataUserPairs,
			wantExtID: null.StringFrom("okta-metadata"),
			wantAudit: map[string]int{},
		},
		{
			name:    "deprecated external id replaces the primary identity",
			userID:  usersMetadataUser,
			email:   "metadata@example.com",
			payload: `{"external_id": "okta-new"}`,
			wantPairs: []dbtools.ExternalIDPair{
				{Iss: testIdentityIssuer, ID: "okta-new"},
				{Iss: usersPingIssuer, ID: "ping-metadata"},
			},
			wantExtID: null.StringFrom("okta-new"),
			wantAudit: map[string]int{"user.external_id.added": 1, "user.external_id.removed": 1},
			wantEvent: events.GovernorEventUpdate,
		},
		{
			name:      "deprecated external id adds the primary identity",
			userID:    usersTestUserID,
			email:     "test@example.com",
			payload:   `{"external_id": "okta-test"}`,
			wantPairs: []dbtools.ExternalIDPair{{Iss: testIdentityIssuer, ID: "okta-test"}},
			wantExtID: null.StringFrom("okta-test"),
			wantAudit: map[string]int{"user.external_id.added": 1},
			wantEvent: events.GovernorEventUpdate,
		},
		{
			name:      "an empty array removes all identities",
			userID:    usersMetadataUser,
			email:     "metadata@example.com",
			payload:   `{"external_ids": []}`,
			wantPairs: noPairs,
			wantAudit: map[string]int{"user.external_id.removed": 2},
			wantEvent: events.GovernorEventUpdate,
		},
		{
			name:      "user fields and identities together",
			userID:    usersTestUserID,
			email:     "test@example.com",
			payload:   `{"name": "Renamed", "external_ids": [{"iss": "` + usersPingIssuer + `", "id": "ping-test"}]}`,
			wantPairs: []dbtools.ExternalIDPair{{Iss: usersPingIssuer, ID: "ping-test"}},
			wantAudit: map[string]int{"user.updated": 1, "user.external_id.added": 1},
			wantEvent: events.GovernorEventUpdate,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.resetExternalIDs()

			w := serveUserRequest(s.identityRouter(testIdentityIssuer).updateUser, http.MethodPut, tt.userID, tt.payload)
			s.Require().Equal(http.StatusAccepted, w.Code, w.Body.String())

			got := decodeIdentity(s.T(), w)
			s.Equal(
				identityResponse{Email: tt.email, Status: UserStatusActive, ExternalIDs: tt.wantPairs, ExternalID: tt.wantExtID, Deprecation: "true", Link: deprecationLink},
				got,
			)
			s.Equal(tt.wantPairs, s.storedPairs(tt.email))
			s.Equal(tt.wantAudit, s.auditActions())
			s.Equal(tt.wantEvent, s.publishedAction())
		})
	}
}

func (s *UserTestSuite) TestUpdateUserExternalIDsRejected() {
	tests := []struct {
		name       string
		primary    string
		userID     string
		payload    string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "both fields",
			primary:    testIdentityIssuer,
			userID:     usersMetadataUser,
			payload:    `{"external_id": "okta-new", "external_ids": []}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "use either external_id or external_ids, not both",
		},
		{
			name:       "deprecated external id without an identity provider",
			userID:     usersMetadataUser,
			payload:    `{"external_id": "okta-new"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "external_id needs an oidc provider with identity.enabled; use external_ids",
		},
		{
			name:       "issuer that is not configured",
			primary:    testIdentityIssuer,
			userID:     usersMetadataUser,
			payload:    `{"external_ids": [{"iss": "https://x.example.com", "id": "x"}]}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "invalid external_ids: iss https://x.example.com is not a configured issuer",
		},
		{
			name:       "identity of another user",
			primary:    testIdentityIssuer,
			userID:     usersTestUserID,
			payload:    `{"name": "Renamed", "external_ids": [{"iss": "` + testIdentityIssuer + `", "id": "okta-metadata"}]}`,
			wantStatus: http.StatusConflict,
			wantErr:    "external id already linked to another user",
		},
		{
			name:       "unknown user",
			primary:    testIdentityIssuer,
			userID:     usersUnknownUserID,
			payload:    `{"external_ids": []}`,
			wantStatus: http.StatusNotFound,
			wantErr:    "user not found: sql: no rows in result set",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.resetExternalIDs()

			w := serveUserRequest(s.identityRouter(tt.primary).updateUser, http.MethodPut, tt.userID, tt.payload)

			s.Equal(tt.wantStatus, w.Code)
			s.JSONEq(`{"error": "`+tt.wantErr+`"}`, w.Body.String())
			s.Equal(metadataUserPairs, s.storedPairs("metadata@example.com"))
			s.Equal(noPairs, s.storedPairs("test@example.com"))
			s.Equal(map[string]int{}, s.auditActions())
			s.Empty(s.publishedAction())

			user, err := models.FindUser(s.T().Context(), s.db, usersTestUserID)
			s.Require().NoError(err)
			s.Equal("Test User", user.Name, "a rejected update must not change the user")
		})
	}
}

func (s *UserTestSuite) TestDeleteUserRemovesExternalIDs() {
	s.resetExternalIDs()

	w := serveUserRequest(s.identityRouter(testIdentityIssuer).deleteUser, http.MethodDelete, usersMetadataUser, "")
	s.Require().Equal(http.StatusAccepted, w.Code, w.Body.String())

	s.Equal(
		identityResponse{Email: "metadata@example.com", Status: UserStatusActive, ExternalIDs: noPairs, Deprecation: "true", Link: deprecationLink},
		decodeIdentity(s.T(), w),
	)
	s.Equal(noPairs, s.storedPairs("metadata@example.com"))
	s.Equal(map[string]int{"user.deleted": 1, "user.external_id.removed": 2}, s.auditActions())
}

func (s *UserTestSuite) TestGetUserExternalIDs() {
	tests := []struct {
		name     string
		primary  string
		userID   string
		wantResp identityResponse
	}{
		{
			name:    "user with identities",
			primary: testIdentityIssuer,
			userID:  usersMetadataUser,
			wantResp: identityResponse{
				Email: "metadata@example.com", Status: UserStatusActive, ExternalIDs: metadataUserPairs,
				ExternalID: null.StringFrom("okta-metadata"), Deprecation: "true", Link: deprecationLink,
			},
		},
		{
			name:    "primary issuer is the second provider",
			primary: usersPingIssuer,
			userID:  usersMetadataUser,
			wantResp: identityResponse{
				Email: "metadata@example.com", Status: UserStatusActive, ExternalIDs: metadataUserPairs,
				ExternalID: null.StringFrom("ping-metadata"), Deprecation: "true", Link: deprecationLink,
			},
		},
		{
			name:   "no identity provider",
			userID: usersMetadataUser,
			wantResp: identityResponse{
				Email: "metadata@example.com", Status: UserStatusActive, ExternalIDs: metadataUserPairs,
				Deprecation: "true", Link: deprecationLink,
			},
		},
		{
			name:    "user without identities",
			primary: testIdentityIssuer,
			userID:  usersTestUserID,
			wantResp: identityResponse{
				Email: "test@example.com", Status: UserStatusActive, ExternalIDs: noPairs,
				Deprecation: "true", Link: deprecationLink,
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.resetExternalIDs()

			w := serveUserRequest(s.identityRouter(tt.primary).getUser, http.MethodGet, tt.userID, "")
			s.Require().Equal(http.StatusOK, w.Code, w.Body.String())
			s.Equal(tt.wantResp, decodeIdentity(s.T(), w))
		})
	}
}

func (s *UserTestSuite) TestGetAuthenticatedUserExternalIDs() {
	s.resetExternalIDs()

	user, err := models.FindUser(s.T().Context(), s.db, usersMetadataUser)
	s.Require().NoError(err)

	admin := false

	w := serveUserRequest(s.identityRouter(testIdentityIssuer).getAuthenticatedUser, http.MethodGet, "", "", func(c *gin.Context) {
		c.Set(contextKeyUser, user)
		c.Set(contextKeyAdmin, &admin)
	})
	s.Require().Equal(http.StatusOK, w.Code, w.Body.String())

	s.Equal(identityResponse{
		Email: "metadata@example.com", Status: UserStatusActive, ExternalIDs: metadataUserPairs,
		ExternalID: null.StringFrom("okta-metadata"), Deprecation: "true", Link: deprecationLink,
	}, decodeIdentity(s.T(), w))
}

func (s *UserTestSuite) TestUpdateAuthenticatedUserExternalIDs() {
	s.resetExternalIDs()

	user, err := models.FindUser(s.T().Context(), s.db, usersMetadataUser)
	s.Require().NoError(err)

	w := serveUserRequest(s.identityRouter(testIdentityIssuer).updateAuthenticatedUser, http.MethodPut, "",
		`{"github_username": "metadata-gh"}`, func(c *gin.Context) {
			c.Set(contextKeyUser, user)
		})
	s.Require().Equal(http.StatusAccepted, w.Code, w.Body.String())

	s.Equal(identityResponse{
		Email: "metadata@example.com", Status: UserStatusActive, ExternalIDs: metadataUserPairs,
		ExternalID: null.StringFrom("okta-metadata"), Deprecation: "true", Link: deprecationLink,
	}, decodeIdentity(s.T(), w))
}

func (s *UserTestSuite) TestListUsersExternalIDs() {
	tests := []struct {
		name  string
		query string
		want  map[string][]dbtools.ExternalIDPair
	}{
		{
			name:  "every user has its identities",
			query: "",
			want: map[string][]dbtools.ExternalIDPair{
				"test@example.com":       noPairs,
				"metadata@example.com":   metadataUserPairs,
				"deleted-user@abcde.com": noPairs,
			},
		},
		{
			name:  "deprecated external id filter matches any issuer",
			query: "external_id=ping-metadata",
			want:  map[string][]dbtools.ExternalIDPair{"metadata@example.com": metadataUserPairs},
		},
		{
			name:  "deprecated external id filter without a match",
			query: "external_id=unknown",
			want:  map[string][]dbtools.ExternalIDPair{},
		},
		{
			name:  "external id at its issuer",
			query: "external_id=ping-metadata&iss=" + url.QueryEscape(usersPingIssuer),
			want:  map[string][]dbtools.ExternalIDPair{"metadata@example.com": metadataUserPairs},
		},
		{
			name:  "external id at another issuer",
			query: "external_id=ping-metadata&iss=" + url.QueryEscape(testIdentityIssuer),
			want:  map[string][]dbtools.ExternalIDPair{},
		},
		{
			name:  "users with a record at an issuer",
			query: "iss=" + url.QueryEscape(usersPingIssuer),
			want:  map[string][]dbtools.ExternalIDPair{"metadata@example.com": metadataUserPairs},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.resetExternalIDs()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1alpha1/users?"+tt.query, nil)

			s.identityRouter(testIdentityIssuer).listUsers(c)
			s.Require().Equal(http.StatusOK, w.Code, w.Body.String())

			var users []User
			s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &users))

			got := map[string][]dbtools.ExternalIDPair{}
			for _, u := range users {
				got[u.Email] = u.ExternalIDs
			}

			s.Equal(tt.want, got)
			s.Equal("true", w.Header().Get("Deprecation"))
		})
	}
}

func (s *UserTestSuite) TestListUsersUnknownIssuer() {
	tests := []struct {
		name  string
		query string
	}{
		{name: "issuer alone", query: "iss=" + url.QueryEscape("https://x.example.com")},
		{name: "issuer with an external id", query: "external_id=x&iss=" + url.QueryEscape("https://x.example.com")},
		{name: "one of two issuers", query: "iss=" + url.QueryEscape(usersPingIssuer) + "&iss=" + url.QueryEscape("https://x.example.com")},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1alpha1/users?"+tt.query, nil)

			s.identityRouter(testIdentityIssuer).listUsers(c)

			s.Equal(http.StatusUnprocessableEntity, w.Code)
			s.JSONEq(`{"error": "invalid iss: not a configured issuer"}`, w.Body.String())
		})
	}
}

func TestUserReqExternalIDsJSON(t *testing.T) {
	tests := []struct {
		name string
		req  UserReq
		want string
	}{
		{
			name: "no identities",
			req:  UserReq{Email: "a@example.com", Name: "A"},
			want: `{"email": "a@example.com", "name": "A"}`,
		},
		{
			name: "empty identities",
			req:  UserReq{Email: "a@example.com", Name: "A", ExternalIDs: &[]dbtools.ExternalIDPair{}},
			want: `{"email": "a@example.com", "name": "A", "external_ids": []}`,
		},
		{
			name: "deprecated external id",
			req:  UserReq{Email: "a@example.com", Name: "A", ExternalID: "x"},
			want: `{"email": "a@example.com", "name": "A", "external_id": "x"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.req)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(b))
		})
	}
}
