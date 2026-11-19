//nolint:noctx
package v1alpha1

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	"github.com/metal-toolbox/governor-api/internal/metrics"
	"github.com/metal-toolbox/governor-api/pkg/configs"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

const (
	resolveOktaUserID      = "00000003-0000-0000-0000-000000000001"
	resolveSuspendedUserID = "00000003-0000-0000-0000-000000000002"
	resolveAuditID         = "10000003-0000-0000-0000-000000000001"
	resolveGroupID         = "20000003-0000-0000-0000-000000000001"
)

// stubOIDC is an OIDC issuer that serves discovery and a userinfo endpoint
// whose response the test sets.
type stubOIDC struct {
	srv *httptest.Server

	mu       sync.Mutex
	status   int
	userInfo map[string]string
}

func newStubOIDC(t *testing.T) *stubOIDC {
	t.Helper()

	s := &stubOIDC{}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 s.srv.URL,
			"authorization_endpoint": s.srv.URL + "/authorize",
			"token_endpoint":         s.srv.URL + "/token",
			"jwks_uri":               s.srv.URL + "/jwks",
			"userinfo_endpoint":      s.srv.URL + "/userinfo",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.status != http.StatusOK {
			w.WriteHeader(s.status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.userInfo)
	})

	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)

	return s
}

// respond sets the userinfo response: status, and the claims sent with 200.
func (s *stubOIDC) respond(status int, name, email, sub string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = status
	s.userInfo = map[string]string{"name": name, "email": email, "sub": sub}
}

// token returns an access token issued by s. Its signature is never checked:
// the auth middleware is replaced in these tests.
func (s *stubOIDC) token(t *testing.T, sub string) string {
	t.Helper()

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("0123456789abcdef0123456789abcdef")}, nil)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	raw, err := jwt.Signed(signer).Claims(jwt.Claims{Issuer: s.srv.URL, Subject: sub}).Serialize()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	return raw
}

// resolveResult is what one request through the user middleware produced.
type resolveResult struct {
	Status int
	Error  string
	Email  string
}

type ResolveUserTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn
	okta *stubOIDC
	ping *stubOIDC
}

func (s *ResolveUserTestSuite) SetupSuite() {
	s.conn = &mockNATSConn{}
	s.db = dbtools.NewPGTestServer(s.T())
	s.okta = newStubOIDC(s.T())
	s.ping = newStubOIDC(s.T())

	s.Require().NoError(migrateTestDB(s.db))
}

// reset empties the tables this suite writes and seeds an Okta user, a
// suspended user and the Okta user's record.
func (s *ResolveUserTestSuite) reset() {
	s.T().Helper()

	stmts := []string{
		`DELETE FROM audit_events`,
		`DELETE FROM user_external_ids`,
		`DELETE FROM users`,
		`DELETE FROM groups`,
		`INSERT INTO groups (id, name, slug, description, created_at, updated_at)
		VALUES ('` + resolveGroupID + `', 'Resolve Group', 'resolve-group', 'group', NOW(), NOW())`,
		`INSERT INTO users (id, name, email, status, created_at, updated_at)
		VALUES ('` + resolveOktaUserID + `', 'Okta User', 'okta@example.com', 'active', NOW(), NOW())`,
		`INSERT INTO users (id, name, email, status, created_at, updated_at)
		VALUES ('` + resolveSuspendedUserID + `', 'Suspended User', 'suspended@example.com', 'suspended', NOW(), NOW())`,
	}

	for _, q := range stmts {
		_, err := s.db.Exec(q)
		s.Require().NoError(err, q)
	}

	_, err := s.db.Exec(`INSERT INTO user_external_ids (user_id, iss, external_id) VALUES ($1, $2, 'okta-sub')`,
		resolveOktaUserID, s.okta.srv.URL)
	s.Require().NoError(err)

	s.conn.Subject, s.conn.Payload = "", nil
}

// router returns a Router whose Okta provider has oktaIdentity and whose Ping
// provider has pingIdentity, with metrics recorded in reg.
func (s *ResolveUserTestSuite) router(reg *prometheus.Registry, oktaIdentity, pingIdentity configs.IdentityConfig) *Router {
	s.T().Helper()

	m, err := metrics.NewUserResolution(reg)
	s.Require().NoError(err)

	return &Router{
		AdminGroups: []string{"governor-admin"},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		AuthConf: []configs.Auth{
			{AuthConfig: ginjwt.AuthConfig{Issuer: s.okta.srv.URL}, Identity: oktaIdentity},
			{AuthConfig: ginjwt.AuthConfig{Issuer: s.ping.srv.URL}, Identity: pingIdentity},
		},
		DB:             sqlx.NewDb(s.db, "postgres"),
		EventBus:       eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:         zap.NewNop(),
		UserResolution: m,
	}
}

// claims are the verified token claims that the replaced auth middleware puts
// in the gin context.
type claims struct {
	iss, user, subject string
}

// serve sends one request for the group groupID through mw, with the given
// claims and bearer token.
func serve(mw gin.HandlerFunc, cl claims, token, groupID string) resolveResult {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/user/:id",
		func(c *gin.Context) {
			c.Set(ginaudit.AuditIDContextKey, resolveAuditID)
			c.Set("jwt.roles", []string{oidcScope})
			c.Set("jwt.user", cl.user)
			c.Set("jwt.subject", cl.subject)
			c.Set("jwt.issuer", cl.iss)
		},
		mw,
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"email": getCtxUser(c).Email})
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/user/"+groupID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	var body struct {
		Error string `json:"error"`
		Email string `json:"email"`
	}

	_ = json.Unmarshal(w.Body.Bytes(), &body)

	return resolveResult{Status: w.Code, Error: body.Error, Email: body.Email}
}

// linkedEmail returns the email of the user that holds (iss, externalID), or "".
func (s *ResolveUserTestSuite) linkedEmail(iss, externalID string) string {
	s.T().Helper()

	var email string

	err := s.db.QueryRow(`SELECT u.email FROM user_external_ids uei JOIN users u ON u.id = uei.user_id
		WHERE uei.iss = $1 AND uei.external_id = $2`, iss, externalID).Scan(&email)
	if err != nil {
		s.Require().ErrorIs(err, sql.ErrNoRows)
	}

	return email
}

// auditActions counts the audit rows by action.
func (s *ResolveUserTestSuite) auditActions() map[string]int {
	s.T().Helper()

	rows, err := s.db.Query(`SELECT action, count(*) FROM audit_events GROUP BY action`)
	s.Require().NoError(err)

	defer rows.Close()

	got := map[string]int{}

	for rows.Next() {
		var (
			action string
			n      int
		)

		s.Require().NoError(rows.Scan(&action, &n))
		got[action] = n
	}

	s.Require().NoError(rows.Err())

	return got
}

// publishedAction returns the action of the last published event, or "".
func (s *ResolveUserTestSuite) publishedAction() string {
	s.T().Helper()

	if s.conn.Payload == nil {
		return ""
	}

	var ev events.Event
	s.Require().NoError(json.Unmarshal(s.conn.Payload, &ev))

	return ev.Action
}

// outcomes returns the governor_user_resolution_total values by "iss outcome".
func outcomes(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	got := map[string]float64{}

	for _, f := range families {
		if f.GetName() != "governor_user_resolution_total" {
			continue
		}

		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}

			got[labels["iss"]+" "+labels["outcome"]] = m.GetCounter().GetValue()
		}
	}

	return got
}

func boolPtr(b bool) *bool { return &b }

func (s *ResolveUserTestSuite) TestUserAuthRequired() {
	enabled := configs.IdentityConfig{Enabled: true}
	noRegister := configs.IdentityConfig{Enabled: true, AutoRegister: boolPtr(false)}
	link := configs.IdentityConfig{Enabled: true, AutoLink: true}
	linkOnly := configs.IdentityConfig{Enabled: true, AutoLink: true, AutoRegister: boolPtr(false)}
	linkNotEnabled := configs.IdentityConfig{AutoLink: true}
	off := configs.IdentityConfig{}

	type userInfo struct {
		status           int
		name, email, sub string
	}

	tests := []struct {
		name         string
		okta, ping   configs.IdentityConfig
		issuer       func() *stubOIDC
		claims       func() claims
		userInfo     userInfo
		want         resolveResult
		wantLinkedTo string
		wantAudit    map[string]int
		wantEvent    string
		wantOutcome  func() map[string]float64
	}{
		{
			name:   "known identity",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "okta-sub", "okta@example.com"} },
			userInfo: userInfo{
				status: http.StatusInternalServerError,
			},
			want:         resolveResult{Status: http.StatusOK, Email: "okta@example.com"},
			wantLinkedTo: "okta@example.com",
			wantAudit:    map[string]int{},
			wantOutcome:  func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " hit": 1} },
		},
		{
			name:   "unknown identity registers a user",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "new-sub", "new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "New User", email: "new@example.com", sub: "new-sub",
			},
			want:         resolveResult{Status: http.StatusOK, Email: "new@example.com"},
			wantLinkedTo: "new@example.com",
			wantAudit:    map[string]int{"user.created": 1, "user.external_id.added": 1},
			wantEvent:    events.GovernorEventCreate,
			wantOutcome:  func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " registered": 1} },
		},
		{
			name:   "okta subject through an issuer without identity enabled",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "okta-sub", "okta@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Okta User", email: "okta@example.com", sub: "okta-sub",
			},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "user not registered"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " rejected_unregistered": 1} },
		},
		{
			name:   "auto-register off",
			okta:   noRegister,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "new-sub", "new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "New User", email: "new@example.com", sub: "new-sub",
			},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "user not registered"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " rejected_unregistered": 1} },
		},
		{
			name:   "email of an existing user with linking off",
			okta:   enabled,
			ping:   enabled,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-sub", "okta@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Okta User", email: "okta@example.com", sub: "ping-sub",
			},
			want:        resolveResult{Status: http.StatusConflict, Error: "a user with this email exists; linking is disabled for this issuer"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " rejected_conflict": 1} },
		},
		{
			name:   "email of a suspended user",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "suspended-sub", "suspended@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Suspended User", email: "suspended@example.com", sub: "suspended-sub",
			},
			want:        resolveResult{Status: http.StatusConflict, Error: "a user with this email exists; linking is disabled for this issuer"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " rejected_conflict": 1} },
		},
		{
			name:   "ping sign-in links the okta user with the same email",
			okta:   enabled,
			ping:   link,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-sub", "okta@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Okta User", email: "okta@example.com", sub: "ping-sub",
			},
			want:         resolveResult{Status: http.StatusOK, Email: "okta@example.com"},
			wantLinkedTo: "okta@example.com",
			wantAudit:    map[string]int{"user.external_id.added": 1},
			wantEvent:    events.GovernorEventUpdate,
			wantOutcome:  func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " linked": 1} },
		},
		{
			name:   "email match ignores case",
			okta:   enabled,
			ping:   linkOnly,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-sub", "OKTA@Example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Okta User", email: "OKTA@Example.com", sub: "ping-sub",
			},
			want:         resolveResult{Status: http.StatusOK, Email: "okta@example.com"},
			wantLinkedTo: "okta@example.com",
			wantAudit:    map[string]int{"user.external_id.added": 1},
			wantEvent:    events.GovernorEventUpdate,
			wantOutcome:  func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " linked": 1} },
		},
		{
			name:   "user already linked to another identity for the issuer",
			okta:   link,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "okta-sub-2", "okta@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Okta User", email: "okta@example.com", sub: "okta-sub-2",
			},
			want:        resolveResult{Status: http.StatusConflict, Error: "user already linked to a different identity for this issuer"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " rejected_conflict": 1} },
		},
		{
			name:   "suspended user is not linked",
			okta:   enabled,
			ping:   link,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-sub", "suspended@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Suspended User", email: "suspended@example.com", sub: "ping-sub",
			},
			want:        resolveResult{Status: http.StatusConflict, Error: "a user with this email exists; linking is disabled for this issuer"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " rejected_conflict": 1} },
		},
		{
			name:   "linking on finds no user and registers one",
			okta:   enabled,
			ping:   link,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-new", "ping-new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Ping User", email: "ping-new@example.com", sub: "ping-new",
			},
			want:         resolveResult{Status: http.StatusOK, Email: "ping-new@example.com"},
			wantLinkedTo: "ping-new@example.com",
			wantAudit:    map[string]int{"user.created": 1, "user.external_id.added": 1},
			wantEvent:    events.GovernorEventCreate,
			wantOutcome:  func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " registered": 1} },
		},
		{
			name:   "linking on and registration off finds no user",
			okta:   enabled,
			ping:   linkOnly,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-new", "ping-new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Ping User", email: "ping-new@example.com", sub: "ping-new",
			},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "user not registered"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " rejected_unregistered": 1} },
		},
		{
			name:   "linking set on a provider without identity enabled",
			okta:   enabled,
			ping:   linkNotEnabled,
			issuer: func() *stubOIDC { return s.ping },
			claims: func() claims { return claims{s.ping.srv.URL, "ping-sub", "okta@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "Okta User", email: "okta@example.com", sub: "ping-sub",
			},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "user not registered"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.ping.srv.URL + " rejected_unregistered": 1} },
		},
		{
			name:        "missing issuer",
			okta:        enabled,
			ping:        off,
			issuer:      func() *stubOIDC { return s.okta },
			claims:      func() claims { return claims{"", "okta-sub", "okta@example.com"} },
			userInfo:    userInfo{status: http.StatusInternalServerError},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "missing jwt issuer"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{} },
		},
		{
			name:        "missing user",
			okta:        enabled,
			ping:        off,
			issuer:      func() *stubOIDC { return s.okta },
			claims:      func() claims { return claims{s.okta.srv.URL, "", "okta@example.com"} },
			userInfo:    userInfo{status: http.StatusInternalServerError},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "missing jwt user"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{} },
		},
		{
			name:        "unknown issuer",
			okta:        enabled,
			ping:        off,
			issuer:      func() *stubOIDC { return s.okta },
			claims:      func() claims { return claims{"https://unknown.example.com", "okta-sub", "okta@example.com"} },
			userInfo:    userInfo{status: http.StatusInternalServerError},
			want:        resolveResult{Status: http.StatusUnauthorized, Error: "unknown issuer"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{} },
		},
		{
			name:        "userinfo error",
			okta:        enabled,
			ping:        off,
			issuer:      func() *stubOIDC { return s.okta },
			claims:      func() claims { return claims{s.okta.srv.URL, "new-sub", "new@example.com"} },
			userInfo:    userInfo{status: http.StatusBadGateway},
			want:        resolveResult{Status: http.StatusBadRequest, Error: "error retrieving userinfo"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " error": 1} },
		},
		{
			name:   "userinfo email differs from the token",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "new-sub", "new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "New User", email: "other@example.com", sub: "new-sub",
			},
			want:        resolveResult{Status: http.StatusBadRequest, Error: "invalid email from oidc userinfo"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " error": 1} },
		},
		{
			name:   "userinfo subject differs from the token",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "new-sub", "new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, name: "New User", email: "new@example.com", sub: "other-sub",
			},
			want:        resolveResult{Status: http.StatusBadRequest, Error: "invalid subject from oidc userinfo"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " error": 1} },
		},
		{
			name:   "userinfo without a name",
			okta:   enabled,
			ping:   off,
			issuer: func() *stubOIDC { return s.okta },
			claims: func() claims { return claims{s.okta.srv.URL, "new-sub", "new@example.com"} },
			userInfo: userInfo{
				status: http.StatusOK, email: "new@example.com", sub: "new-sub",
			},
			want:        resolveResult{Status: http.StatusBadRequest, Error: "invalid name from oidc userinfo"},
			wantAudit:   map[string]int{},
			wantOutcome: func() map[string]float64 { return map[string]float64{s.okta.srv.URL + " error": 1} },
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.reset()

			reg := prometheus.NewRegistry()
			r := s.router(reg, tt.okta, tt.ping)
			idp := tt.issuer()
			cl := tt.claims()

			idp.respond(tt.userInfo.status, tt.userInfo.name, tt.userInfo.email, tt.userInfo.sub)

			got := serve(r.mwUserAuthRequired(AuthRoleUser), cl, idp.token(s.T(), cl.subject), resolveGroupID)

			s.Equal(tt.want, got)
			s.Equal(tt.wantLinkedTo, s.linkedEmail(cl.iss, cl.user))
			s.Equal(tt.wantAudit, s.auditActions())
			s.Equal(tt.wantEvent, s.publishedAction())
			s.Equal(tt.wantOutcome(), outcomes(s.T(), reg))
		})
	}
}

func (s *ResolveUserTestSuite) TestGroupAuthRequiredResolvesWithoutRegistering() {
	tests := []struct {
		name string
		user string
		want resolveResult
	}{
		{
			name: "known identity",
			user: "okta-sub",
			want: resolveResult{Status: http.StatusUnauthorized, Error: "user not group member"},
		},
		{
			name: "unknown identity",
			user: "new-sub",
			want: resolveResult{Status: http.StatusUnauthorized, Error: "user not found"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.reset()

			r := s.router(prometheus.NewRegistry(), configs.IdentityConfig{Enabled: true}, configs.IdentityConfig{})
			cl := claims{s.okta.srv.URL, tt.user, "new@example.com"}

			got := serve(r.mwGroupAuthRequired(AuthRoleGroupMember), cl, s.okta.token(s.T(), cl.subject), resolveGroupID)

			s.Equal(tt.want, got)
			s.Equal(map[string]int{}, s.auditActions())
			s.Empty(s.linkedEmail(s.okta.srv.URL, "new-sub"))
		})
	}
}

// TestConcurrentFirstSignIn holds an uncommitted registration of the same
// identity while the request runs, so the request's insert waits on the email
// index and then fails with a unique violation once the other one commits.
func (s *ResolveUserTestSuite) TestConcurrentFirstSignIn() {
	s.reset()

	reg := prometheus.NewRegistry()
	r := s.router(reg, configs.IdentityConfig{Enabled: true}, configs.IdentityConfig{})
	s.okta.respond(http.StatusOK, "Racer", "racer@example.com", "racer-sub")

	ctx := context.Background()

	other, err := s.db.BeginTx(ctx, nil)
	s.Require().NoError(err)

	defer func() { _ = other.Rollback() }()

	var otherID string

	s.Require().NoError(other.QueryRowContext(ctx, `INSERT INTO users (name, email, status, created_at, updated_at)
		VALUES ('Racer', 'racer@example.com', 'active', NOW(), NOW()) RETURNING id`).Scan(&otherID))

	_, err = other.ExecContext(ctx, `INSERT INTO user_external_ids (user_id, iss, external_id) VALUES ($1, $2, 'racer-sub')`,
		otherID, s.okta.srv.URL)
	s.Require().NoError(err)

	cl := claims{s.okta.srv.URL, "racer-sub", "racer@example.com"}
	done := make(chan resolveResult, 1)

	go func() {
		done <- serve(r.mwUserAuthRequired(AuthRoleUser), cl, s.okta.token(s.T(), cl.subject), resolveGroupID)
	}()

	time.Sleep(500 * time.Millisecond)
	s.Require().NoError(other.Commit())

	got := <-done

	s.Equal(resolveResult{Status: http.StatusOK, Email: "racer@example.com"}, got)
	s.Equal(map[string]float64{s.okta.srv.URL + " hit": 1}, outcomes(s.T(), reg))
	s.Equal(map[string]int{}, s.auditActions())
}

// TestConcurrentFirstLink holds the lock on the Okta user while another
// transaction links the same Ping identity to it. Once that one commits, the
// request finds the committed record under the lock and writes nothing.
func (s *ResolveUserTestSuite) TestConcurrentFirstLink() {
	s.reset()

	reg := prometheus.NewRegistry()
	r := s.router(reg, configs.IdentityConfig{Enabled: true}, configs.IdentityConfig{Enabled: true, AutoLink: true})
	s.ping.respond(http.StatusOK, "Okta User", "okta@example.com", "ping-sub")

	ctx := context.Background()

	other, err := s.db.BeginTx(ctx, nil)
	s.Require().NoError(err)

	defer func() { _ = other.Rollback() }()

	_, err = other.ExecContext(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, resolveOktaUserID)
	s.Require().NoError(err)

	_, err = other.ExecContext(ctx, `INSERT INTO user_external_ids (user_id, iss, external_id) VALUES ($1, $2, 'ping-sub')`,
		resolveOktaUserID, s.ping.srv.URL)
	s.Require().NoError(err)

	cl := claims{s.ping.srv.URL, "ping-sub", "okta@example.com"}
	done := make(chan resolveResult, 1)

	go func() {
		done <- serve(r.mwUserAuthRequired(AuthRoleUser), cl, s.ping.token(s.T(), cl.subject), resolveGroupID)
	}()

	time.Sleep(500 * time.Millisecond)
	s.Require().NoError(other.Commit())

	got := <-done

	s.Equal(resolveResult{Status: http.StatusOK, Email: "okta@example.com"}, got)
	s.Equal(map[string]float64{s.ping.srv.URL + " hit": 1}, outcomes(s.T(), reg))
	s.Equal(map[string]int{}, s.auditActions())
	s.Empty(s.publishedAction())
}

// TestFirstSignInLockTimeout holds an uncommitted user with the same email for
// longer than the lock timeout, so the request gives up and writes nothing.
func (s *ResolveUserTestSuite) TestFirstSignInLockTimeout() {
	s.reset()

	reg := prometheus.NewRegistry()
	r := s.router(reg, configs.IdentityConfig{Enabled: true}, configs.IdentityConfig{})
	s.okta.respond(http.StatusOK, "Waiter", "waiter@example.com", "waiter-sub")

	ctx := context.Background()

	other, err := s.db.BeginTx(ctx, nil)
	s.Require().NoError(err)

	defer func() { _ = other.Rollback() }()

	_, err = other.ExecContext(ctx, `INSERT INTO users (name, email, status, created_at, updated_at)
		VALUES ('Waiter', 'waiter@example.com', 'active', NOW(), NOW())`)
	s.Require().NoError(err)

	cl := claims{s.okta.srv.URL, "waiter-sub", "waiter@example.com"}
	got := serve(r.mwUserAuthRequired(AuthRoleUser), cl, s.okta.token(s.T(), cl.subject), resolveGroupID)

	s.Equal(resolveResult{Status: http.StatusInternalServerError, Error: "error getting user: timeout"}, got)
	s.Equal(map[string]float64{s.okta.srv.URL + " error": 1}, outcomes(s.T(), reg))
	s.Empty(s.linkedEmail(s.okta.srv.URL, "waiter-sub"))
	s.Equal(map[string]int{}, s.auditActions())
}

func TestResolveUserSuite(t *testing.T) {
	suite.Run(t, new(ResolveUserTestSuite))
}
