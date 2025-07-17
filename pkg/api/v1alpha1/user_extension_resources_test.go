package v1alpha1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondl/null/v8"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	dbm "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
	"go.hollow.sh/toolbox/ginauth"
	"go.uber.org/zap"
)

type UserExtensionResourceTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn
}

func (s *UserExtensionResourceTestSuite) seedTestDB() error {
	testData := []string{
		// test extension
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000001', 'Test Extension', 'some extension', true, 'test-extension', 'online');`,

		// test ERD
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
		VALUES ('00000002-0000-0000-0000-000000000001', 'User Resource', 'some-description', true, 'user-resource', 'user-resources', 'v1', 'user',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000001');
		`,
		// test system ERD
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id)
		VALUES ('00000002-0001-0000-0000-000000000001', 'System Resource', 'some-description', true, 'system-resource', 'system-resources', 'v1', 'system',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000001');
		`,

		// test users
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000003-0000-0000-0000-000000000001', NULL, 'Harold Admin', 'hadmin@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');`,
		`INSERT INTO "users" ("id", "external_id", "name", "email", "login_count", "avatar_url", "last_login_at", "created_at", "updated_at", "github_id", "github_username", "deleted_at", "status") VALUES
		('00000003-0000-0000-0000-000000000002', NULL, 'John User', 'juser@email.com', 0, NULL, NULL, '2023-07-12 12:00:00.000000+00', '2023-07-12 12:00:00.000000+00', NULL, NULL, NULL, 'active');`,

		// test extension resources each of these resource should be associated with the user above
		// harold admin's resources
		`INSERT INTO user_extension_resources (id, resource, extension_resource_definition_id, user_id)
		VALUES ('00000004-0000-0000-0000-000000000001', '{"age": 10, "firstName": "Hello", "lastName": "World"}'::jsonb, '00000002-0000-0000-0000-000000000001', '00000003-0000-0000-0000-000000000001');`,
		`INSERT INTO user_extension_resources (id, resource, extension_resource_definition_id, deleted_at, user_id)
		VALUES ('00000004-0000-0000-0000-000000000002', '{"age": 10, "firstName": "Hello", "lastName": "World"}'::jsonb, '00000002-0000-0000-0000-000000000001', '2023-07-12 12:00:00.000000+00', '00000003-0000-0000-0000-000000000001');`,
		`INSERT INTO user_extension_resources (id, resource, extension_resource_definition_id, user_id)
		VALUES ('00000004-0000-0000-0000-000000000003', '{"age": 10, "firstName": "Hello", "lastName": "Harold"}'::jsonb, '00000002-0000-0000-0000-000000000001', '00000003-0000-0000-0000-000000000001');`,
		// john user's resources
		`INSERT INTO user_extension_resources (id, resource, extension_resource_definition_id, user_id)
		VALUES ('00000004-0001-0000-0000-000000000001', '{"age": 30, "firstName": "Hello1", "lastName": "World1"}'::jsonb, '00000002-0000-0000-0000-000000000001', '00000003-0000-0000-0000-000000000002');`,
		`INSERT INTO user_extension_resources (id, resource, extension_resource_definition_id, deleted_at, user_id)
		VALUES ('00000004-0001-0000-0000-000000000002', '{"age": 30, "firstName": "Hello2", "lastName": "World2"}'::jsonb, '00000002-0000-0000-0000-000000000001', '2023-07-12 12:00:00.000000+00','00000003-0000-0000-0000-000000000002');`,
		`INSERT INTO user_extension_resources (id, resource, extension_resource_definition_id, user_id)
		VALUES ('00000004-0001-0000-0000-000000000003', '{"age": 30, "firstName": "Hello1", "lastName": "John"}'::jsonb, '00000002-0000-0000-0000-000000000001', '00000003-0000-0000-0000-000000000002');`,

		// test system extension resources
		`INSERT INTO system_extension_resources (id, resource, extension_resource_definition_id)
		VALUES ('00000004-0002-0000-0000-000000000001', '{"age": 10, "firstName": "Hello", "lastName": "World"}'::jsonb, '00000002-0001-0000-0000-000000000001');`,
	}

	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *UserExtensionResourceTestSuite) v1alpha1() *Router {
	return &Router{
		AdminGroups: []string{"governor-admin"},
		AuthMW:      &ginauth.MultiTokenMiddleware{},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		DB:          sqlx.NewDb(s.db, "postgres"),
		EventBus:    eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:      &zap.Logger{},
	}
}

func (s *UserExtensionResourceTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	s.conn = &mockNATSConn{}
	s.db = dbtools.NewPGTestServer(s.T())

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(s.db, "migrations"); err != nil {
		panic("migration failed - could not set up test db")
	}

	if err := s.seedTestDB(); err != nil {
		panic("db setup failed - could not seed test db: " + err.Error())
	}
}

func (s *UserExtensionResourceTestSuite) TestFetchUserAndERD() {
	tests := []struct {
		name   string
		params gin.Params

		contextUser *models.User

		expectedExtensionID string
		expectedERDID       string
		expectedUserID      string

		expectedFindUserErr error
		expectedFindERDErr  error
	}{
		{
			name: "fetch with user ID ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},

			expectedExtensionID: "00000001-0000-0000-0000-000000000001",
			expectedERDID:       "00000002-0000-0000-0000-000000000001",
			expectedUserID:      "00000003-0000-0000-0000-000000000001",
		},
		{
			name: "fetch with context user ok",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				ID:     "00000003-0000-0000-0000-000000000001",
				Name:   "Harold Admin",
				Email:  "hadmin@email.com",
				Status: null.NewString("active", true),
			},
			expectedExtensionID: "00000001-0000-0000-0000-000000000001",
			expectedERDID:       "00000002-0000-0000-0000-000000000001",
			expectedUserID:      "00000003-0000-0000-0000-000000000001",
		},
		{
			name: "no user provided",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedFindUserErr: ErrNoUserProvided,
		},
		{
			name: "user not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000003"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedFindUserErr: sql.ErrNoRows,
		},
		{
			name: "extension not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension-2"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedFindERDErr: ErrExtensionNotFound,
		},
		{
			name: "ERD not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v2"},
			},
			expectedFindERDErr: ErrERDNotFound,
		},
	}

	// test url is only a placeholder
	url := "/api/v1alpha1/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id"

	// run the tests
	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", url, nil)
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)
			setCtxUser(c, tt.contextUser)

			user, extension, erd, findUserErr, findERDErr := fetchUserAndERD(c, s.db)

			switch {
			case tt.expectedFindUserErr != nil:
				s.EqualError(findUserErr, tt.expectedFindUserErr.Error())
			case tt.expectedFindERDErr != nil:
				s.EqualError(findERDErr, tt.expectedFindERDErr.Error())
			default:
				s.Nil(findUserErr)
				s.Nil(findERDErr)
				s.NotNil(user)
				s.NotNil(extension)
				s.NotNil(erd)

				s.Equal(tt.expectedExtensionID, extension.ID)
				s.Equal(tt.expectedERDID, erd.ID)
				s.Equal(tt.expectedUserID, user.ID)
			}
		})
	}
}

func (s *UserExtensionResourceTestSuite) TestListUserExtensionResources() {
	r := s.v1alpha1()

	tests := []struct {
		name        string
		params      gin.Params
		contextUser *models.User
		query       string

		expectedStatus int
		expectedCount  int
		expectedErrMsg string
	}{
		{
			name: "list with user-id resources ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "list with context user resources ok",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				Name:   "John User",
				ID:     "00000003-0000-0000-0000-000000000002",
				Email:  "juser@email.com",
				Status: null.NewString("active", true),
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "list with user-id resources with deleted",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			query:          "deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  4,
		},
		{
			name: "list with context-user resources with deleted",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				Name:   "John User",
				ID:     "00000003-0000-0000-0000-000000000002",
				Status: null.NewString("active", true),
			},
			query:          "deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  4,
		},
		{
			name: "list resources with with query",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			query:          "age=10&deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  4,
		},
		{
			name: "list resource with string query",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			query:          "lastName=World",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name: "user not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000003"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "user does not exist",
		},
		{
			name: "extension not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension-2"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
		},
		{
			name: "ERD not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources-2"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "ERD does not exist",
		},
		{
			name: "attempt to list system resources",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "system-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "cannot list system resource for",
		},
	}

	// test url is only a placeholder
	url := "/api/v1alpha1/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id"

	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", fmt.Sprintf("%s?%s", url, tt.query), nil)
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)
			setCtxUser(c, tt.contextUser)

			r.listUserExtensionResources(c)

			s.Equal(tt.expectedStatus, w.Code)

			body := w.Body.String()

			if tt.expectedErrMsg != "" {
				s.Contains(
					body, tt.expectedErrMsg, "Expected error message to contain %q, got %s",
					tt.expectedErrMsg, body,
				)

				return
			}

			resp := []interface{}{}
			err := json.Unmarshal([]byte(body), &resp)

			s.Nil(err)
			s.Equal(tt.expectedCount, len(resp))
		})
	}
}

func (s *UserExtensionResourceTestSuite) TestGetUserExtensionResource() {
	r := s.v1alpha1()

	tests := []struct {
		name        string
		params      gin.Params
		contextUser *models.User
		query       string

		expectedStatus int
		expectedErrMsg string
	}{
		{
			name: "get with user-id resource ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "get with context user resource ok",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				Name:   "John User",
				ID:     "00000003-0000-0000-0000-000000000002",
				Status: null.NewString("active", true),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "get deleted resource ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000002"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			query:          "deleted",
			expectedStatus: http.StatusOK,
		},
		{
			name: "get deleted resource without deleted query param",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000002"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "user not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000003"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v2"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "user does not exist",
		},
		{
			name: "extension not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension-2"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources-2"},
				gin.Param{Key: "erd-version", Value: "v2"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
		},
		{
			name: "ERD not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources-2"},
				gin.Param{Key: "erd-version", Value: "v2"},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "resource not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000004"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension resource does not exist",
		},
		{
			name: "attempt to get system resource",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0002-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "system-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "cannot fetch system resource for",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", fmt.Sprintf("?%s", tt.query), nil)
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)
			setCtxUser(c, tt.contextUser)

			r.getUserExtensionResource(c)

			s.Equal(tt.expectedStatus, w.Code)

			body := w.Body.String()

			if tt.expectedErrMsg != "" {
				s.Contains(
					body, tt.expectedErrMsg, "Expected error message to contain %q, got %s",
					tt.expectedErrMsg, body,
				)

				return
			}

			resp := map[string]interface{}{}
			err := json.Unmarshal([]byte(body), &resp)

			s.Nil(err)
			s.Equal(tt.expectedStatus, w.Code)
		})
	}
}

func (s *UserExtensionResourceTestSuite) TestCreateUserExtensionResource() {
	r := s.v1alpha1()

	tests := []struct {
		name        string
		params      gin.Params
		contextUser *models.User
		payload     string

		expectedStatus       int
		expectedEventSubject string
		expectedEventPayload *events.Event
		expectedErrMsg       string
	}{
		{
			name: "create with user-id resource ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:              `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus:       http.StatusCreated,
			expectedEventSubject: "events.user-resources",
			expectedEventPayload: &events.Event{
				Version:                       "v1",
				Action:                        events.GovernorEventCreate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
				UserID:                        "00000003-0000-0000-0000-000000000001",
			},
		},
		{
			name: "create with context user resource ok",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				Name:   "John User",
				ID:     "00000003-0000-0000-0000-000000000002",
				Status: null.NewString("active", true),
			},
			payload:              `{"age": 10, "firstName": "Hello", "lastName": "World-2"}`,
			expectedStatus:       http.StatusCreated,
			expectedEventSubject: "events.user-resources",
			expectedEventPayload: &events.Event{
				Version:                       "v1",
				Action:                        events.GovernorEventCreate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
				UserID:                        "00000003-0000-0000-0000-000000000002",
			},
		},
		{
			name: "json schema violation (missing required field)",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello"}`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "missing property 'lastName'",
		},
		{
			name: "json schema violation (unique constrain)",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "unique constraint violation",
		},
		{
			name: "user not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000003"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "user does not exist",
		},
		{
			name: "extension not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension-2"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
		},
		{
			name: "ERD not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources-2"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "ERD does not exist",
		},
		{
			name: "invalid json payload",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "unexpected end of JSON input",
		},
		{
			name: "attempt to create system resource",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0002-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "system-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "cannot create system resource for",
		},
	}

	url := "/api/v1alpha1/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version"

	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("POST", url, nil)
			req.Body = io.NopCloser(bytes.NewBufferString(tt.payload))
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)
			setCtxUser(c, tt.contextUser)

			r.createUserExtensionResource(c)

			s.Equal(tt.expectedStatus, w.Code)

			body := w.Body.String()

			if tt.expectedErrMsg != "" {
				s.Contains(
					body, tt.expectedErrMsg, "Expected error message to contain %q, got %s",
					tt.expectedErrMsg, body,
				)

				return
			}

			ur := &UserExtensionResource{}
			err := json.Unmarshal([]byte(body), ur)
			s.Nil(err)

			event := &events.Event{}
			err = json.Unmarshal(s.conn.Payload, event)
			s.Nil(err)

			s.Nil(err, "Expected no error unmarshalling response")

			s.Equal(tt.expectedEventPayload.Version, event.Version)
			s.Equal(tt.expectedEventPayload.Action, event.Action)
			s.Equal(tt.expectedEventPayload.ExtensionID, event.ExtensionID)
			s.Equal(tt.expectedEventPayload.ExtensionResourceDefinitionID, event.ExtensionResourceDefinitionID)
			s.Equal(tt.expectedEventPayload.UserID, event.UserID)
			s.Equal(event.ExtensionResourceID, ur.ID)
		})
	}
}

// TestUpdateUserExtensionResource tests the update user extension resource endpoint
func (s *UserExtensionResourceTestSuite) TestUpdateUserExtensionResource() {
	r := s.v1alpha1()

	tests := []struct {
		name        string
		params      gin.Params
		contextUser *models.User
		payload     string

		expectedStatus       int
		expectedEventSubject string
		expectedEventPayload *events.Event
		expectedErrMsg       string
	}{
		{
			name: "update resource with user-id ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:              `{"age": 10, "firstName": "Hello", "lastName": "World-11"}`,
			expectedStatus:       http.StatusAccepted,
			expectedEventSubject: "events.user-resources",
			expectedEventPayload: &events.Event{
				Version:                       "v1",
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
				UserID:                        "00000003-0000-0000-0000-000000000002",
				ExtensionResourceID:           "00000004-0001-0000-0000-000000000001",
			},
		},
		{
			name: "update resource with context user ok",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				Name:   "John User",
				ID:     "00000003-0000-0000-0000-000000000002",
				Status: null.NewString("active", true),
			},
			payload:              `{"age": 10, "firstName": "Hello", "lastName": "World-22"}`,
			expectedStatus:       http.StatusAccepted,
			expectedEventSubject: "events.user-resources",
			expectedEventPayload: &events.Event{
				Version:                       "v1",
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
				UserID:                        "00000003-0000-0000-0000-000000000002",
				ExtensionResourceID:           "00000004-0001-0000-0000-000000000001",
			},
		},
		{
			name: "json schema violation (missing required field)",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello"}`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "missing property 'lastName'",
		},
		{
			name: "json schema violation (unique constrain)",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "unique constraint violation",
		},
		{
			name: "user not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000003"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "user does not exist",
		},
		{
			name: "extension not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension-2"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
		},
		{
			name: "ERD not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources-2"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "ERD does not exist",
		},
		{
			name: "invalid json payload",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "unexpected end of JSON input",
		},
		{
			name: "attempt to update system resource",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0002-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "system-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			payload:        `{"age": 10, "firstName": "Hello", "lastName": "World-1"}`,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "cannot update system resource for",
		},
	}

	url := "/api/v1alpha1/users/:id/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id"

	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("PATCH", url, nil)
			req.Body = io.NopCloser(bytes.NewBufferString(tt.payload))
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)
			setCtxUser(c, tt.contextUser)

			r.updateUserExtensionResource(c)

			s.Equal(tt.expectedStatus, w.Code)

			body := w.Body.String()

			if tt.expectedErrMsg != "" {
				s.Contains(
					body, tt.expectedErrMsg, "Expected error message to contain %q, got %s",
					tt.expectedErrMsg, body,
				)

				return
			}

			ur := &UserExtensionResource{}
			err := json.Unmarshal([]byte(body), ur)
			s.Nil(err)

			event := &events.Event{}
			err = json.Unmarshal(s.conn.Payload, event)
			s.Nil(err)

			s.Nil(err, "Expected no error unmarshalling response")

			s.Equal(tt.expectedEventPayload.Version, event.Version)
			s.Equal(tt.expectedEventPayload.Action, event.Action)
			s.Equal(tt.expectedEventPayload.ExtensionID, event.ExtensionID)
			s.Equal(tt.expectedEventPayload.ExtensionResourceDefinitionID, event.ExtensionResourceDefinitionID)
			s.Equal(tt.expectedEventPayload.UserID, event.UserID)
			s.Equal(event.ExtensionResourceID, ur.ID)
		})
	}
}

// TestDeleteUserExtensionResource tests the delete user extension resource endpoint
func (s *UserExtensionResourceTestSuite) TestDeleteUserExtensionResource() {
	r := s.v1alpha1()

	tests := []struct {
		name        string
		params      gin.Params
		contextUser *models.User

		expectedStatus       int
		expectedEventSubject string
		expectedEventPayload *events.Event
		expectedErrMsg       string
	}{
		{
			name: "delete resource with user-id ok",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0001-0000-0000-000000000003"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus:       http.StatusOK,
			expectedEventSubject: "events.user-resources",
			expectedEventPayload: &events.Event{
				Version:                       "v1",
				Action:                        events.GovernorEventDelete,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
				UserID:                        "00000003-0000-0000-0000-000000000002",
				ExtensionResourceID:           "00000004-0001-0000-0000-000000000003",
			},
		},
		{
			name: "delete resource with context user ok",
			params: gin.Params{
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0000-0000-0000-000000000003"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			contextUser: &models.User{
				Name:   "Harold Admin",
				ID:     "00000003-0000-0000-0000-000000000001",
				Status: null.NewString("active", true),
			},
			expectedStatus:       http.StatusOK,
			expectedEventSubject: "events.user-resources",
			expectedEventPayload: &events.Event{
				Version:                       "v1",
				Action:                        events.GovernorEventDelete,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
				UserID:                        "00000003-0000-0000-0000-000000000001",
				ExtensionResourceID:           "00000004-0000-0000-0000-000000000003",
			},
		},
		{
			name: "user not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000003"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0000-0000-0000-000000000003"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "user does not exist",
		},
		{
			name: "extension not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension-2"},
				gin.Param{Key: "resource-id", Value: "00000004-0000-0000-0000-000000000003"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
		},
		{
			name: "ERD not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0000-0000-0000-000000000003"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources-2"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "ERD does not exist",
		},
		{
			name: "resource not found",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000002"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0000-0000-0000-000000000004"},
				gin.Param{Key: "erd-slug-plural", Value: "user-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension resource does not exist",
		},
		{
			name: "attempt to delete system resource",
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000003-0000-0000-0000-000000000001"},
				gin.Param{Key: "ex-slug", Value: "test-extension"},
				gin.Param{Key: "resource-id", Value: "00000004-0002-0000-0000-000000000001"},
				gin.Param{Key: "erd-slug-plural", Value: "system-resources"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "cannot delete system resource for",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("DELETE", "", nil)
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)
			setCtxUser(c, tt.contextUser)

			r.deleteUserExtensionResource(c)

			s.Equal(tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				s.Contains(w.Body.String(), tt.expectedErrMsg)
				return
			}

			event := &events.Event{}
			err := json.Unmarshal(s.conn.Payload, event)
			s.Nil(err)

			s.Nil(err, "Expected no error unmarshalling response")

			s.Equal(tt.expectedEventPayload.Version, event.Version)
			s.Equal(tt.expectedEventPayload.Action, event.Action)
			s.Equal(tt.expectedEventPayload.ExtensionID, event.ExtensionID)
			s.Equal(tt.expectedEventPayload.ExtensionResourceDefinitionID, event.ExtensionResourceDefinitionID)
			s.Equal(tt.expectedEventPayload.UserID, event.UserID)
			s.Equal(event.ExtensionResourceID, tt.params.ByName("resource-id"))
		})
	}
}

func TestUserExtensionResourceSuite(t *testing.T) {
	suite.Run(t, new(UserExtensionResourceTestSuite))
}
