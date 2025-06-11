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
		VALUES ('00000001-0000-0000-0000-000000000003', 'Metadata User', 'metadata@example.com', 'active', '{"department":"marketing","location":"remote","details":{"level":3,"team":"growth"}, "annotations": {"test/hello": "world"}}', NOW(), NOW());`,
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
			payload: `{
		    "name": "Metadata User",
				"email": "with-metadata@example.com",
				"metadata": {
				  "department": "sales",
					"location": "office",
					"details": {"level": 2, "team": "enterprise"},
					"annotations": {
						"test/hello": "world"
					}
				}
			}`,
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
		{
			name:           "invalid metadata key",
			url:            "/api/v1alpha1/users",
			expectedStatus: http.StatusBadRequest,
			payload:        `{ "name": "Invalid Metadata User", "email": "invalid-metadata@example.com", "metadata": {".... . .-.. .-.. ---": ".-- --- .-. .-.. -.."} }`,
			expectedErrMsg: "invalid metadata keys",
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
				"annotations": map[string]interface{}{
					"test/hello": "world",
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
		{
			name:           "update with invalid metadata key",
			url:            "/api/v1alpha1/users/00000001-0000-0000-0000-000000000001",
			expectedStatus: http.StatusBadRequest,
			payload:        `{ "metadata": {".... . .-.. .-.. ---": ".-- --- .-. .-.. -.."} }`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000001-0000-0000-0000-000000000001"},
			},
			expectedErrMsg: "invalid metadata keys",
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
			name:           "filter with slash",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`annotations.test/hello=world`}},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the user with annotations.test/hello=world
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
			name: "include deleted users",
			url:  "/api/v1alpha1/users",
			queryParams: map[string][]string{
				"deleted":  {""},
				"metadata": {`department=engineering`},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the deleted user
		},
		{
			name:           "invalid metadata query",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {`invalidquerybadbad`}},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata query format",
		},
		{
			name: "invalid metadata key",
			url:  "/api/v1alpha1/users",
			queryParams: map[string][]string{
				"metadata": {`.... . .-.. .-.. ---=.-. .-- --- .-. .-.. -..`},
			},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata key",
		},
		{
			name:           "empty metadata filter",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {""}},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata query format",
		},
		{
			name:           "empty metadata key",
			url:            "/api/v1alpha1/users",
			queryParams:    map[string][]string{"metadata": {"=value"}},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata key",
		},
		{
			name: "value type mismatch 1",
			url:  "/api/v1alpha1/users",
			queryParams: map[string][]string{
				"metadata": {`details.level=stringvalue`}, // level is a number, not a string
			},
			expectedStatus: http.StatusOK,
			expectedCount:  0, // No results, no error
		},
		{
			name: "value type mismatch 2",
			url:  "/api/v1alpha1/users",
			queryParams: map[string][]string{
				"metadata": {`annotations=3`}, // annotations is an object, not a number
			},
			expectedStatus: http.StatusOK,
			expectedCount:  0, // No results, no error
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

func TestMetadataKeyPattern(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		// Valid keys
		{name: "simple alphanumeric", key: "key123", expected: true},
		{name: "single character", key: "k", expected: true},
		{name: "with underscores", key: "key_name", expected: true},
		{name: "with dashes", key: "key-name", expected: true},
		{name: "with slashes", key: "path/to/key", expected: true},
		{name: "starts with letter", key: "a123", expected: true},
		{name: "starts with number", key: "1abc", expected: true},
		{name: "mixed characters", key: "key123_name-with/path", expected: true},

		// Invalid keys
		{name: "single number", key: "7", expected: false},
		{name: "empty string", key: "", expected: false},
		{name: "with spaces", key: "key name", expected: false},
		{name: "with special chars", key: "key@name", expected: false},
		{name: "starts with underscore", key: "_key", expected: false},
		{name: "starts with dash", key: "-key", expected: false},
		{name: "starts with slash", key: "/path", expected: false},
		{name: "ends with underscore", key: "key_", expected: false},
		{name: "ends with dash", key: "key-", expected: false},
		{name: "ends with slash", key: "path/", expected: false},
		{name: "with period", key: "key.name", expected: false},
		{name: "with unicode", key: "keyλname", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := metadataKeyPattern.MatchString(tt.key)
			assert.Equal(t, tt.expected, result,
				"Key '%s': expected match=%v, got=%v", tt.key, tt.expected, result)
		})
	}
}

func TestIsValidMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		expected bool
	}{
		{
			name:     "empty metadata",
			metadata: map[string]interface{}{},
			expected: true,
		},
		{
			name: "simple valid metadata",
			metadata: map[string]interface{}{
				"department": "engineering",
				"location":   "remote",
				"level":      3,
			},
			expected: true,
		},
		{
			name: "nested valid metadata",
			metadata: map[string]interface{}{
				"details": map[string]interface{}{
					"team":     "platform",
					"level":    4,
					"location": "remote",
				},
				"projects": []string{"project1", "project2"},
			},
			expected: true,
		},
		{
			name: "metadata with array of maps",
			metadata: map[string]interface{}{
				"teams": []interface{}{
					map[string]interface{}{
						"name":     "team1",
						"projects": []string{"proj1", "proj2"},
					},
					map[string]interface{}{
						"name":     "team2",
						"projects": []string{"proj3", "proj4"},
					},
				},
			},
			expected: true,
		},
		{
			name: "complex nested valid metadata",
			metadata: map[string]interface{}{
				"organization": map[string]interface{}{
					"department": "engineering",
					"teams": map[string]interface{}{
						"platform": map[string]interface{}{
							"members": []interface{}{
								map[string]interface{}{
									"role":  "engineer",
									"level": 3,
								},
							},
						},
					},
				},
				"annotations": map[string]interface{}{
					"test/key":  "value",
					"test/key2": "value2",
				},
			},
			expected: true,
		},
		{
			name: "invalid key at root level",
			metadata: map[string]interface{}{
				"valid":       "data",
				"invalid.key": "data",
			},
			expected: false,
		},
		{
			name: "invalid key in nested map",
			metadata: map[string]interface{}{
				"valid": map[string]interface{}{
					"also-valid": "data",
					"not@valid":  "data",
				},
			},
			expected: false,
		},
		{
			name: "invalid key in array of maps",
			metadata: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"valid":      "data",
						"not_valid_": "data",
					},
				},
			},
			expected: false,
		},
		{
			name: "valid path characters",
			metadata: map[string]interface{}{
				"path/to/value":        "test",
				"path/with_underscore": "test",
				"path/with-dash":       "test",
			},
			expected: true,
		},
		{
			name: "invalid ending character",
			metadata: map[string]interface{}{
				"ends_with_": "invalid",
				"ends/with/": "invalid",
				"ends-with-": "invalid",
			},
			expected: false,
		},
		{
			name: "invalid starting character",
			metadata: map[string]interface{}{
				"_starts_with_underscore": "invalid",
				"/starts/with/slash":      "invalid",
				"-starts-with-dash":       "invalid",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidMetadata(tt.metadata)
			assert.Equal(t, tt.expected, result,
				"isValidMetadata(%v): expected %v, got %v", tt.metadata, tt.expected, result)
		})
	}
}
