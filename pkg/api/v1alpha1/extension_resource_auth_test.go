package v1alpha1

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	dbm "github.com/metal-toolbox/governor-api/db"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
	"go.hollow.sh/toolbox/ginauth"
	"go.uber.org/zap"
)

type ExtensionResourcesGroupAuthTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn

	v1alpha1 *Router

	haroladAdmin *models.User
	johnUser     *models.User
}

func (s *ExtensionResourcesGroupAuthTestSuite) seedTestDB() error {
	testData := []string{
		// extensions
		`INSERT INTO extensions (id, name, description, enabled, slug, status, created_at, updated_at)
		VALUES ('00000001-0000-0000-0000-000000000001', 'Test Extension 1', 'some extension', true, 'test-extension-1', 'online', now(), now());`,

		// groups
		`INSERT INTO groups (id, name, slug, description, note, created_at, updated_at)
		VALUES ('00000002-0000-0000-0000-000000000001', 'Governor Admin', 'governor-admin', 'governor-admin', 'some note', now(), now());`,
		`INSERT INTO groups (id, name, slug, description, note, created_at, updated_at)
		VALUES ('00000002-0000-0000-0000-000000000002', 'Ext Admin', 'ext-admin', 'ext-admin', 'some note', now(), now());`,

		// test users
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000003-0000-0000-0000-000000000001', NULL, 'Harold Admin', 'hadmin@email.com', 0, NULL, NULL, now(), now(), NULL, NULL, NULL, 'active');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000003-0000-0000-0000-000000000002', NULL, 'John User', 'juser@email.com', 0, NULL, NULL, now(), now(), NULL, NULL, NULL, 'active');`,

		// group members
		// 		harold-admin -> governor-admin
		`INSERT INTO "group_memberships" (user_id, group_id, created_at, updated_at)
		VALUES ('00000003-0000-0000-0000-000000000001', '00000002-0000-0000-0000-000000000001', now(), now());`,
		// 		john-user -> ext-admin
		`INSERT INTO "group_memberships" (user_id, group_id, created_at, updated_at)
		VALUES ('00000003-0000-0000-0000-000000000002', '00000002-0000-0000-0000-000000000002', now(), now());`,

		// ERDs
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
		VALUES ('00000004-0000-0000-0000-000000000001', 'Some Resource', 'some-description', true, 'some-resource', 'some-resources', 'v1', 'system',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000001');
		`,

		// ERs
		`
		INSERT INTO system_extension_resources (id, extension_resource_definition_id, resource)
		VALUES ('00000005-0000-0000-0000-000000000001', '00000004-0000-0000-0000-000000000001', '{"firstName": "a", "lastName": "b"}'::jsonb);
		`,
		`
		INSERT INTO system_extension_resources (id, extension_resource_definition_id, resource)
		VALUES ('00000005-0000-0000-0000-000000000002', '00000004-0000-0000-0000-000000000001', '{"firstName": "a", "lastName": "b"}'::jsonb);
		`,
		`
		INSERT INTO system_extension_resources (id, extension_resource_definition_id, resource)
		VALUES ('00000005-0000-0000-0000-000000000003', '00000004-0000-0000-0000-000000000001', '{"firstName": "a", "lastName": "b"}'::jsonb);
		`,
	}

	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

// custom routes for test, it skips actual jwt validation
func extResAuthTestRoutes(rg *gin.RouterGroup, r *Router) {
	rg.POST(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("CreateSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.mwSystemExtensionResourceGroupAuth,
		r.mwExtensionResourcesEnabledCheck,
		r.createSystemExtensionResource,
	)

	rg.GET(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("ListSystemExtensionResources"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.listSystemExtensionResources,
	)

	rg.GET(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("GetSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.getSystemExtensionResource,
	)

	rg.PATCH(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("UpdateSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.mwSystemExtensionResourceGroupAuth,
		r.mwExtensionResourcesEnabledCheck,
		r.updateSystemExtensionResource,
	)

	rg.DELETE(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("DeleteSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.mwSystemExtensionResourceGroupAuth,
		r.mwExtensionResourcesEnabledCheck,
		r.deleteSystemExtensionResource,
	)

	// user extension resources
	rg.POST(
		"/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("CreateUserExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:users")),
		r.mwExtensionResourcesEnabledCheck,
		r.createUserExtensionResource,
	)

	rg.POST(
		"/user/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("CreateAuthenticatedUserExtensionResource"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwExtensionResourcesEnabledCheck,
		r.createUserExtensionResource,
	)

	rg.GET(
		"/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("ListUserExtensionResources"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:users")),
		r.listUserExtensionResources,
	)

	rg.GET(
		"/user/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("ListAuthenticatedUserExtensionResources"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.listUserExtensionResources,
	)

	rg.GET(
		"/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("GetUserExtensionResource"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:users")),
		r.getUserExtensionResource,
	)

	rg.GET(
		"/user/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("GetAuthenticatedUserExtensionResources"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.getUserExtensionResource,
	)

	rg.PATCH(
		"/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("UpdateUserExtensionResource"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:users")),
		r.mwExtensionResourcesEnabledCheck,
		r.updateUserExtensionResource,
	)

	rg.PATCH(
		"/user/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("UpdateAuthenticatedUserExtensionResources"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwExtensionResourcesEnabledCheck,
		r.updateUserExtensionResource,
	)

	rg.DELETE(
		"/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("DeleteUserExtensionResource"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:users")),
		r.mwExtensionResourcesEnabledCheck,
		r.deleteUserExtensionResource,
	)

	rg.DELETE(
		"/user/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("DeleteAuthenticatedUserExtensionResources"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwExtensionResourcesEnabledCheck,
		r.deleteUserExtensionResource,
	)
}

func (s *ExtensionResourcesGroupAuthTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	s.conn = &mockNATSConn{}

	ts, err := dbtools.NewCRDBTestServer()
	if err != nil {
		panic(err)
	}

	s.db, err = sql.Open("postgres", ts.PGURL().String())
	if err != nil {
		panic(err)
	}

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(s.db, "migrations"); err != nil {
		panic("migration failed - could not set up test db")
	}

	if err := s.seedTestDB(); err != nil {
		panic("db setup failed - could not seed test db: " + err.Error())
	}

	s.johnUser = &models.User{
		ID:    "00000003-0000-0000-0000-000000000002",
		Name:  "John User",
		Email: "juser@email.com",
	}

	s.haroladAdmin = &models.User{
		ID:    "00000003-0000-0000-0000-000000000001",
		Name:  "Harold Admin",
		Email: "hadmin@email.com",
	}

	s.v1alpha1 = &Router{
		AdminGroups: []string{"governor-admin"},
		AuthMW:      &ginauth.MultiTokenMiddleware{},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		DB:          sqlx.NewDb(s.db, "postgres"),
		EventBus:    eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:      zap.NewNop(),
	}
}

func (s *ExtensionResourcesGroupAuthTestSuite) mwForgeUser(u *models.User, isAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		setCtxUser(c, u)
		setCtxAdmin(c, &isAdmin)
		c.Set("jwt.roles", []string{oidcScope})
	}
}

func (s *ExtensionResourcesGroupAuthTestSuite) updateERD(ctx context.Context, payload string) error {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	auditID := uuid.New().String()
	params := gin.Params{
		gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
		gin.Param{Key: "erd-id-slug", Value: "00000004-0000-0000-0000-000000000001"},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		"/api/v1alpha1/extensions/test-extension-1/erds/some-resources/v1",
		io.NopCloser(bytes.NewBufferString(payload)),
	)
	if err != nil {
		return err
	}

	c.Request = req
	c.Params = params
	c.Set(ginaudit.AuditIDContextKey, auditID)

	s.v1alpha1.updateExtensionResourceDefinition(c)

	if w.Code != http.StatusAccepted {
		return fmt.Errorf("expected %d, got %d: resp: %s", http.StatusAccepted, w.Code, w.Body.String()) // nolint:err113
	}

	return nil
}

func (s *ExtensionResourcesGroupAuthTestSuite) TestGetResources() {
	tt := []struct {
		name       string
		resourceID string
		user       *models.User
		admin      bool
		respcode   int
	}{
		{
			name:       "admin-get-resources",
			respcode:   http.StatusOK,
			resourceID: "00000005-0000-0000-0000-000000000001",
			admin:      true,
			user:       s.haroladAdmin,
		},
		{
			name:       "non-admin-get-resources",
			respcode:   http.StatusOK,
			resourceID: "00000005-0000-0000-0000-000000000001",
			admin:      false,
			user:       s.johnUser,
		},
	}

	s.T().Parallel()

	for _, tc := range tt {
		r := gin.New()
		rg := r.Group("/api/v1alpha1")
		rg.Use(s.mwForgeUser(tc.user, tc.admin))
		extResAuthTestRoutes(rg, s.v1alpha1)

		s.T().Run(tc.name, func(_ *testing.T) {
			w := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				fmt.Sprintf(
					"/api/v1alpha1/extension-resources/test-extension-1/some-resources/v1/%s",
					tc.resourceID,
				),
				nil,
			)
			s.Assert().NoError(err)

			r.ServeHTTP(w, req)
			s.Assert().Equal(tc.respcode, w.Code, fmt.Sprintf("expected %d, got %d", tc.respcode, w.Code))
		})
	}
}

func (s *ExtensionResourcesGroupAuthTestSuite) TestListResources() {
	tt := []struct {
		name     string
		user     *models.User
		admin    bool
		respcode int
	}{
		{
			name:     "admin-list-resources",
			respcode: http.StatusOK,
			admin:    true,
			user:     s.haroladAdmin,
		},
		{
			name:     "non-admin-list-resources",
			respcode: http.StatusOK,
			admin:    false,
			user:     s.johnUser,
		},
	}

	s.T().Parallel()

	for _, tc := range tt {
		r := gin.New()
		rg := r.Group("/api/v1alpha1")
		rg.Use(s.mwForgeUser(tc.user, tc.admin))
		extResAuthTestRoutes(rg, s.v1alpha1)

		s.T().Run(tc.name, func(_ *testing.T) {
			w := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/api/v1alpha1/extension-resources/test-extension-1/some-resources/v1",
				nil,
			)
			s.Assert().NoError(err)

			r.ServeHTTP(w, req)
			s.Assert().Equal(tc.respcode, w.Code, fmt.Sprintf("expected %d, got %d", tc.respcode, w.Code))
		})
	}
}

func (s *ExtensionResourcesGroupAuthTestSuite) TestCreateResource() {
	tt := []struct {
		name     string
		user     *models.User
		admin    bool
		respcode int
		before   func(context.Context) error
		after    func(context.Context) error
	}{
		{
			name:     "admin-create-resource",
			respcode: http.StatusCreated,
			admin:    true,
			user:     s.haroladAdmin,
		},
		{
			name:     "non-admin-create-resource",
			respcode: http.StatusForbidden,
			admin:    false,
			user:     s.johnUser,
		},
		{
			name:     "admin-group-member-create-resource",
			respcode: http.StatusCreated,
			admin:    false,
			user:     s.johnUser,
			before: func(ctx context.Context) error {
				return s.updateERD(ctx, `{ "admin_group": "00000002-0000-0000-0000-000000000002" }`)
			},
			after: func(ctx context.Context) error {
				return s.updateERD(ctx, `{ "admin_group": "" }`)
			},
		},
	}

	payload := `{"firstName": "a", "lastName": "b"}`

	for _, tc := range tt {
		ctx := context.Background()

		if tc.before != nil {
			err := tc.before(ctx)
			s.Assert().NoError(err)
		}

		r := gin.New()
		rg := r.Group("/api/v1alpha1")
		rg.Use(s.mwForgeUser(tc.user, tc.admin))
		extResAuthTestRoutes(rg, s.v1alpha1)

		s.T().Run(tc.name, func(_ *testing.T) {
			w := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				"/api/v1alpha1/extension-resources/test-extension-1/some-resources/v1",
				io.NopCloser(bytes.NewBufferString(payload)),
			)
			s.Assert().NoError(err)

			r.ServeHTTP(w, req)
			s.Assert().Equal(tc.respcode, w.Code, fmt.Sprintf("expected %d, got %d: resp: %s", tc.respcode, w.Code, w.Body.String()))
		})

		if tc.after != nil {
			err := tc.after(ctx)
			s.Assert().NoError(err)
		}
	}
}

func (s *ExtensionResourcesGroupAuthTestSuite) TestUpdateResource() {
	tt := []struct {
		name     string
		user     *models.User
		admin    bool
		respcode int
		before   func(context.Context) error
		after    func(context.Context) error
	}{
		{
			name:     "admin-update-resource",
			respcode: http.StatusAccepted,
			admin:    true,
			user:     s.haroladAdmin,
		},
		{
			name:     "non-admin-update-resource",
			respcode: http.StatusForbidden,
			admin:    false,
			user:     s.johnUser,
		},
		{
			name:     "admin-group-member-update-resource",
			respcode: http.StatusAccepted,
			admin:    false,
			user:     s.johnUser,
			before: func(ctx context.Context) error {
				return s.updateERD(ctx, `{ "admin_group": "00000002-0000-0000-0000-000000000002" }`)
			},
			after: func(ctx context.Context) error {
				return s.updateERD(ctx, `{ "admin_group": "" }`)
			},
		},
	}

	payload := `{"firstName": "a", "lastName": "b"}`

	for _, tc := range tt {
		ctx := context.Background()

		if tc.before != nil {
			err := tc.before(ctx)
			s.Assert().NoError(err)
		}

		r := gin.New()
		rg := r.Group("/api/v1alpha1")
		rg.Use(s.mwForgeUser(tc.user, tc.admin))
		extResAuthTestRoutes(rg, s.v1alpha1)

		s.T().Run(tc.name, func(_ *testing.T) {
			w := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPatch,
				"/api/v1alpha1/extension-resources/test-extension-1/some-resources/v1/00000005-0000-0000-0000-000000000001",
				io.NopCloser(bytes.NewBufferString(payload)),
			)
			s.Assert().NoError(err)

			r.ServeHTTP(w, req)
			s.Assert().Equal(tc.respcode, w.Code, fmt.Sprintf("expected %d, got %d: resp: %s", tc.respcode, w.Code, w.Body.String()))
		})

		if tc.after != nil {
			err := tc.after(ctx)
			s.Assert().NoError(err)
		}
	}
}

func (s *ExtensionResourcesGroupAuthTestSuite) TestDeleteResource() {
	tt := []struct {
		name       string
		user       *models.User
		admin      bool
		respcode   int
		resourceID string
		before     func(context.Context) error
		after      func(context.Context) error
	}{
		{
			name:       "admin-delete-resource",
			resourceID: "00000005-0000-0000-0000-000000000002",
			respcode:   http.StatusAccepted,
			admin:      true,
			user:       s.haroladAdmin,
		},
		{
			name:       "non-admin-delete-resource",
			resourceID: "00000005-0000-0000-0000-000000000003",
			respcode:   http.StatusForbidden,
			admin:      false,
			user:       s.johnUser,
		},
		{
			name:       "admin-group-member-delete-resource",
			respcode:   http.StatusAccepted,
			admin:      false,
			user:       s.johnUser,
			resourceID: "00000005-0000-0000-0000-000000000003",
			before: func(ctx context.Context) error {
				return s.updateERD(ctx, `{ "admin_group": "00000002-0000-0000-0000-000000000002" }`)
			},
			after: func(ctx context.Context) error {
				return s.updateERD(ctx, `{ "admin_group": "" }`)
			},
		},
	}

	for _, tc := range tt {
		ctx := context.Background()

		if tc.before != nil {
			err := tc.before(ctx)
			s.Assert().NoError(err)
		}

		r := gin.New()
		rg := r.Group("/api/v1alpha1")
		rg.Use(s.mwForgeUser(tc.user, tc.admin))
		extResAuthTestRoutes(rg, s.v1alpha1)

		s.T().Run(tc.name, func(_ *testing.T) {
			w := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodDelete,
				fmt.Sprintf(
					"/api/v1alpha1/extension-resources/test-extension-1/some-resources/v1/%s",
					tc.resourceID,
				),
				nil,
			)
			s.Assert().NoError(err)

			r.ServeHTTP(w, req)
			s.Assert().Equal(tc.respcode, w.Code, fmt.Sprintf("expected %d, got %d: resp: %s", tc.respcode, w.Code, w.Body.String()))
		})

		if tc.after != nil {
			err := tc.after(ctx)
			s.Assert().NoError(err)
		}
	}
}

func TestExtensionResourcesGroupAuthTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionResourcesGroupAuthTestSuite))
}
