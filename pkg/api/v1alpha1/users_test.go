package v1alpha1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/sqlboiler/v4/types"
	"go.hollow.sh/toolbox/ginauth"
	"go.uber.org/zap"
)

type UserTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn
}

func (s *UserTestSuite) seedTestDB() error {
	testData := []string{
		`INSERT INTO users (id, name, email, status, created_at, updated_at) 
		VALUES ('00000001-0000-0000-0000-000000000001', 'Test User', 'test@example.com', 'active', NOW(), NOW());`,
		`INSERT INTO users (id, name, email, status, metadata, created_at, updated_at, deleted_at) 
		VALUES ('00000001-0000-0000-0000-000000000002', 'Deleted User', 'deleted@example.com', 'active', '{"department":"engineering"}', NOW(), NOW(), '2023-07-12 12:00:00.000000+00');`,
		`INSERT INTO users (id, name, email, status, metadata, created_at, updated_at) 
		VALUES ('00000001-0000-0000-0000-000000000003', 'Metadata User', 'metadata@example.com', 'active', '{"department":"marketing","location":"remote","details":{"level":3,"team":"growth"}}', NOW(), NOW());`,
	}

	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *UserTestSuite) v1alpha1() *Router {
	return &Router{
		AdminGroups: []string{"governor-admin"},
		AuthMW:      &ginauth.MultiTokenMiddleware{},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		DB:          sqlx.NewDb(s.db, "postgres"),
		EventBus:    eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:      zap.NewNop(),
	}
}

func (s *UserTestSuite) SetupSuite() {
	s.conn = &mockNATSConn{}

	gin.SetMode(gin.TestMode)

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
}

func (s *UserTestSuite) SetupTest() {
	if _, err := s.db.Exec("DELETE FROM audit_events"); err != nil {
		s.T().Fatalf("Failed to reset audit_events table: %v", err)
	}

	if _, err := s.db.Exec("DELETE FROM users"); err != nil {
		s.T().Fatalf("Failed to reset users table: %v", err)
	}

	if err := s.seedTestDB(); err != nil {
		s.T().Fatalf("Failed to seed test database: %v", err)
	}
}

func (s *UserTestSuite) TestCreateUser() {
	r := s.v1alpha1()

	tests := []struct {
		name           string
		url            string
		payload        string
		expectedResp   *models.User
		expectedStatus int
		expectedErrMsg string
	}{
		{
			name:           "create without metadata",
			url:            "/api/v1alpha1/users",
			expectedStatus: http.StatusAccepted,
			payload:        `{ "name": "New User", "email": "new@example.com" }`,
			expectedResp: &models.User{
				Name:     "New User",
				Email:    "new@example.com",
				Metadata: types.JSON{},
			},
		},
		{
			name:           "create with empty metadata",
			url:            "/api/v1alpha1/users",
			expectedStatus: http.StatusAccepted,
			payload:        `{ "name": "Empty Metadata User", "email": "empty-metadata@example.com", "metadata": {} }`,
			expectedResp: &models.User{
				Name:     "Empty Metadata User",
				Email:    "empty-metadata@example.com",
				Metadata: types.JSON{},
			},
		},
		{
			name:           "create with metadata",
			url:            "/api/v1alpha1/users",
			expectedStatus: http.StatusAccepted,
			payload:        `{ "name": "Metadata User", "email": "with-metadata@example.com", "metadata": {"department": "sales", "location": "office", "details": {"level": 2, "team": "enterprise"}} }`,
			expectedResp: &models.User{
				Name:     "Metadata User",
				Email:    "with-metadata@example.com",
				Metadata: types.JSON{},
			},
		},
		{
			name:           "duplicate user",
			url:            "/api/v1alpha1/users",
			payload:        `{ "name": "Test User", "email": "test@example.com" }`,
			expectedStatus: http.StatusConflict,
			expectedErrMsg: "user already exists",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("POST", tt.url, nil)
			req.Body = io.NopCloser(bytes.NewBufferString(tt.payload))
			c.Request = req
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.createUser(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}

			user := &models.User{}
			body := w.Body.String()
			err := json.Unmarshal([]byte(body), user)
			assert.Nil(t, err)

			assert.Equal(
				t, tt.expectedResp.Name, user.Name,
				"Expected user name %s, got %s", tt.expectedResp.Name, user.Name,
			)

			assert.Equal(
				t, tt.expectedResp.Email, user.Email,
				"Expected user email %s, got %s", tt.expectedResp.Email, user.Email,
			)

			// If metadata was provided in the payload, verify it exists in the response
			if strings.Contains(tt.payload, `"metadata":`) {
				assert.NotEmpty(t, user.Metadata, "Expected metadata to not be empty")
			}
		})
	}
}

func (s *UserTestSuite) TestUpdateUserMetadata() {
	r := s.v1alpha1()

	tests := []struct {
		name             string
		url              string
		params           gin.Params
		payload          string
		expectedMetadata map[string]interface{}
		expectedStatus   int
		expectedErrMsg   string
	}{
		{
			name:           "update with empty metadata",
			url:            "/api/v1alpha1/users/00000001-0000-0000-0000-000000000001",
			expectedStatus: http.StatusAccepted,
			payload:        `{ "metadata": {} }`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000001-0000-0000-0000-000000000001"},
			},
			expectedMetadata: map[string]interface{}{},
		},
		{
			name:           "update with new metadata",
			url:            "/api/v1alpha1/users/00000001-0000-0000-0000-000000000001",
			expectedStatus: http.StatusAccepted,
			payload:        `{ "metadata": {"department": "engineering", "location": "remote"} }`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000001-0000-0000-0000-000000000001"},
			},
			expectedMetadata: map[string]interface{}{
				"department": "engineering",
				"location":   "remote",
			},
		},
		{
			name:           "merge metadata",
			url:            "/api/v1alpha1/users/00000001-0000-0000-0000-000000000003",
			expectedStatus: http.StatusAccepted,
			payload:        `{ "metadata": {"projects": ["alpha", "beta"], "details": {"level": 4}} }`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000001-0000-0000-0000-000000000003"},
			},
			expectedMetadata: map[string]interface{}{
				"department": "marketing",
				"location":   "remote",
				"projects":   []interface{}{"alpha", "beta"},
				"details": map[string]interface{}{
					"level": float64(4),
					"team":  "growth",
				},
			},
		},
		{
			name:           "user not found",
			url:            "/api/v1alpha1/users/00000001-0000-0000-0000-000000000099",
			expectedStatus: http.StatusNotFound,
			payload:        `{ "metadata": {"department": "engineering"} }`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000001-0000-0000-0000-000000000099"},
			},
			expectedErrMsg: "user not found",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("PATCH", tt.url, nil)
			req.Body = io.NopCloser(bytes.NewBufferString(tt.payload))
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.updateUser(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}

			user := &models.User{}
			body := w.Body.String()
			err := json.Unmarshal([]byte(body), user)
			assert.Nil(t, err)

			// Extract and verify metadata
			var actualMetadata map[string]interface{}
			err = user.Metadata.Unmarshal(&actualMetadata)
			assert.Nil(t, err)

			assert.Equal(t, tt.expectedMetadata, actualMetadata,
				"Expected metadata %v, got %v", tt.expectedMetadata, actualMetadata)
		})
	}
}

func (s *UserTestSuite) TestListUsersWithMetadataFilter() {
	r := s.v1alpha1()

	tests := []struct {
		name           string
		url            string
		queryParams    map[string][]string
		expectedStatus int
		expectedErrMsg string
		expectedCount  int
	}{
		{
			name:           "list all users",
			url:            "/api/v1alpha1/users",
			expectedStatus: http.StatusOK,
			expectedCount:  2, // All non-deleted users
		},
		{
			name:           "include deleted users",
			url:            "/api/v1alpha1/users?deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  3, // All users
		},
		{
			name:           "filter by department=marketing",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`department=marketing`}},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only metadata@example.com
		},
		{
			name:           "filter by nested metadata field",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`details.level=3`}},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the user with details.level=3
		},
		{
			name:           "filter with no matches",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`department=finance`}},
			expectedStatus: http.StatusOK,
			expectedCount:  0, // No users in finance department
		},
		{
			name: "filter with multiple metadata fields",
			url:  "/api/v1alpha1/users",
			queryParams: map[string][]string{
				"metadata": {
					`department=marketing`,
					`details.level=3`,
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the user with both conditions
		},
		{
			name:           "filter with value contains an equal sign",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`key=base64endedWith=`}},
			expectedStatus: http.StatusOK,
			expectedCount:  0, // No results, no error
		},
		{
			name:           "invalid metadata query",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`invalidquerybadbad`}},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata query format",
		},
		{
			name: "include deleted users",
			url:  "/api/v1alpha1/users",
			queryParams: map[string][]string{
				"deleted":  {""},
				"metadata": {`department=engineering`},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the deleted user
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", tt.url, nil)

			// Add query parameters
			if len(tt.queryParams) > 0 {
				q := url.Values{}

				for k, v := range tt.queryParams {
					for _, vv := range v {
						q.Add(k, vv)
					}
				}

				req.URL.RawQuery = q.Encode()
			}

			c.Request = req
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.listUsers(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}

			// Parse response body
			body := w.Body.String()
			users := []models.User{}
			err := json.Unmarshal([]byte(body), &users)
			assert.Nil(t, err, "Expected to unmarshal response body")

			assert.Equal(t, tt.expectedCount, len(users),
				"Expected %d users, got %d", tt.expectedCount, len(users))
		})
	}
}

func TestUserSuite(t *testing.T) {
	suite.Run(t, new(UserTestSuite))
}
