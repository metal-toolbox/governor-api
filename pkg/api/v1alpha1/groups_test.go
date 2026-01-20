//nolint:noctx
package v1alpha1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	dbm "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.hollow.sh/toolbox/ginauth"
	"go.uber.org/zap"
)

func TestCreateGroupRequestValidator(t *testing.T) {
	tests := map[string]struct {
		input     models.Group
		expectErr error
	}{
		"Empty": {
			models.Group{
				Description: "",
				Name:        "",
			},
			ErrEmptyInput,
		},
		"InvalidChar": {
			models.Group{
				Description: "n/a",
				Name:        "john's-secret-group",
			},
			ErrInvalidChar,
		},
		"Valid": {
			models.Group{
				Description: "n/a",
				Name:        "ajZ9-( A )[0] &.",
			},
			nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, actualErr := createGroupRequestValidator(&test.input)
			assert.ErrorIs(t, actualErr, test.expectErr)
		})
	}
}

type GroupTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn
}

func (s *GroupTestSuite) seedTestDB() error {
	testData := []string{
		`INSERT INTO groups (id, name, slug, description, metadata, created_at, updated_at) 
		VALUES ('00000002-0000-0000-0000-000000000001', 'Platform Team', 'platform-team', 'Platform engineering team', '{}', NOW(), NOW());`,
		`INSERT INTO groups (id, name, slug, description, metadata, created_at, updated_at, deleted_at) 
		VALUES ('00000002-0000-0000-0000-000000000002', 'Deleted Group', 'deleted-group', 'A deleted group', '{"department":"engineering"}', NOW(), NOW(), '2023-07-12 12:00:00.000000+00');`,
		`INSERT INTO groups (id, name, slug, description, metadata, created_at, updated_at) 
		VALUES ('00000002-0000-0000-0000-000000000003', 'Marketing Team', 'marketing-team', 'Marketing team', '{"department":"marketing","location":"remote","details":{"level":3,"team":"growth"}, "annotations": {"test/hello": "world"}}', NOW(), NOW());`,
		`INSERT INTO groups (id, name, slug, description, metadata, created_at, updated_at) 
		VALUES ('00000002-0000-0000-0000-000000000004', 'Sales Team', 'sales-team', 'Sales team', '{"department":"sales","region":"emea"}', NOW(), NOW());`,
	}

	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *GroupTestSuite) v1alpha1() *Router {
	return &Router{
		AdminGroups: []string{"governor-admin"},
		AuthMW:      &ginauth.MultiTokenMiddleware{},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		DB:          sqlx.NewDb(s.db, "postgres"),
		EventBus:    eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:      zap.NewNop(),
	}
}

func (s *GroupTestSuite) SetupSuite() {
	s.conn = &mockNATSConn{}
	s.db = dbtools.NewPGTestServer(s.T())

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(s.db, "migrations"); err != nil {
		panic("migration failed - could not set up test db")
	}
}

func (s *GroupTestSuite) SetupTest() {
	if _, err := s.db.Exec("DELETE FROM audit_events"); err != nil {
		s.T().Fatalf("Failed to reset audit_events table: %v", err)
	}

	if _, err := s.db.Exec("DELETE FROM groups"); err != nil {
		s.T().Fatalf("Failed to reset groups table: %v", err)
	}

	if err := s.seedTestDB(); err != nil {
		s.T().Fatalf("Failed to seed test database: %v", err)
	}
}

func (s *GroupTestSuite) TestCreateGroupWithMetadata() {
	r := s.v1alpha1()

	tests := []struct {
		name             string
		url              string
		payload          string
		expectedStatus   int
		expectedErrMsg   string
		expectedMetadata map[string]interface{}
	}{
		{
			name:             "create group with empty metadata",
			url:              "/api/v1alpha1/groups",
			expectedStatus:   http.StatusAccepted,
			payload:          `{"name": "New Group 1", "description": "A new group", "metadata": {}}`,
			expectedMetadata: map[string]interface{}{},
		},
		{
			name:           "create group with metadata",
			url:            "/api/v1alpha1/groups",
			expectedStatus: http.StatusAccepted,
			payload:        `{"name": "New Group 2", "description": "A new group", "metadata": {"department": "engineering", "cost-center": "cc-1234"}}`,
			expectedMetadata: map[string]interface{}{
				"department":  "engineering",
				"cost-center": "cc-1234",
			},
		},
		{
			name:           "create group with nested metadata",
			url:            "/api/v1alpha1/groups",
			expectedStatus: http.StatusAccepted,
			payload:        `{"name": "New Group 3", "description": "A new group", "metadata": {"details": {"team": "platform", "level": 5}}}`,
			expectedMetadata: map[string]interface{}{
				"details": map[string]interface{}{
					"team":  "platform",
					"level": float64(5),
				},
			},
		},
		{
			name:           "create group with invalid metadata key",
			url:            "/api/v1alpha1/groups",
			expectedStatus: http.StatusBadRequest,
			payload:        `{"name": "Bad Group", "description": "A bad group", "metadata": {"invalid.key": "value"}}`,
			expectedErrMsg: "invalid metadata keys",
		},
		{
			name:             "create group without metadata",
			url:              "/api/v1alpha1/groups",
			expectedStatus:   http.StatusAccepted,
			payload:          `{"name": "No Metadata Group", "description": "A group without metadata"}`,
			expectedMetadata: map[string]interface{}{},
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

			r.createGroup(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body)

				return
			}

			group := &models.Group{}
			body := w.Body.String()
			err := json.Unmarshal([]byte(body), group)
			assert.Nil(t, err)

			var actualMetadata map[string]interface{}

			err = group.Metadata.Unmarshal(&actualMetadata)
			assert.Nil(t, err)

			assert.Equal(t, tt.expectedMetadata, actualMetadata,
				"Expected metadata %v, got %v", tt.expectedMetadata, actualMetadata)
		})
	}
}

func (s *GroupTestSuite) TestUpdateGroupMetadata() {
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
			url:            "/api/v1alpha1/groups/00000002-0000-0000-0000-000000000001",
			expectedStatus: http.StatusAccepted,
			payload:        `{"description": "Updated description", "metadata": {}}`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000002-0000-0000-0000-000000000001"},
			},
			expectedMetadata: map[string]interface{}{},
		},
		{
			name:           "update with new metadata",
			url:            "/api/v1alpha1/groups/00000002-0000-0000-0000-000000000001",
			expectedStatus: http.StatusAccepted,
			payload:        `{"description": "Updated", "metadata": {"department": "engineering", "location": "remote"}}`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000002-0000-0000-0000-000000000001"},
			},
			expectedMetadata: map[string]interface{}{
				"department": "engineering",
				"location":   "remote",
			},
		},
		{
			name:           "merge metadata",
			url:            "/api/v1alpha1/groups/00000002-0000-0000-0000-000000000003",
			expectedStatus: http.StatusAccepted,
			payload:        `{"description": "Updated", "metadata": {"projects": ["alpha", "beta"], "details": {"level": 4}}}`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000002-0000-0000-0000-000000000003"},
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
			name:           "group not found",
			url:            "/api/v1alpha1/groups/00000002-0000-0000-0000-000000000099",
			expectedStatus: http.StatusNotFound,
			payload:        `{"description": "Updated", "metadata": {"department": "engineering"}}`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000002-0000-0000-0000-000000000099"},
			},
			expectedErrMsg: "group not found",
		},
		{
			name:           "update with invalid metadata key",
			url:            "/api/v1alpha1/groups/00000002-0000-0000-0000-000000000001",
			expectedStatus: http.StatusBadRequest,
			payload:        `{"description": "Updated", "metadata": {".... . .-.. .-.. ---": ".-- --- .-. .-.. -.."}}`,
			params: gin.Params{
				gin.Param{Key: "id", Value: "00000002-0000-0000-0000-000000000001"},
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

			r.updateGroup(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body)

				return
			}

			group := &models.Group{}
			body := w.Body.String()
			err := json.Unmarshal([]byte(body), group)
			assert.Nil(t, err)

			var actualMetadata map[string]interface{}

			err = group.Metadata.Unmarshal(&actualMetadata)
			assert.Nil(t, err)

			assert.Equal(t, tt.expectedMetadata, actualMetadata,
				"Expected metadata %v, got %v", tt.expectedMetadata, actualMetadata)
		})
	}
}

func (s *GroupTestSuite) TestListGroupsWithMetadataFilter() {
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
			name:           "list all groups",
			url:            "/api/v1alpha1/groups",
			expectedStatus: http.StatusOK,
			expectedCount:  3, // All non-deleted groups
		},
		{
			name:           "include deleted groups",
			url:            "/api/v1alpha1/groups?deleted",
			queryParams:    map[string][]string{"deleted": {""}},
			expectedStatus: http.StatusOK,
			expectedCount:  4, // All groups
		},
		{
			name:           "filter by department=marketing",
			url:            "/api/v1alpha1/groups",
			queryParams:    map[string][]string{"metadata": {`department=marketing`}},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only marketing-team
		},
		{
			name:           "filter by nested metadata field",
			url:            "/api/v1alpha1/groups",
			queryParams:    map[string][]string{"metadata": {`details.level=3`}},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the group with details.level=3
		},
		{
			name:           "filter with slash in key",
			url:            "/api/v1alpha1/groups",
			queryParams:    map[string][]string{"metadata": {`annotations.test/hello=world`}},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the group with annotations.test/hello=world
		},
		{
			name:           "filter with no matches",
			url:            "/api/v1alpha1/groups",
			queryParams:    map[string][]string{"metadata": {`department=finance`}},
			expectedStatus: http.StatusOK,
			expectedCount:  0, // No groups in finance department
		},
		{
			name: "filter with multiple metadata fields",
			url:  "/api/v1alpha1/groups",
			queryParams: map[string][]string{
				"metadata": {
					`department=marketing`,
					`details.level=3`,
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the group with both conditions
		},
		{
			name: "include deleted groups with metadata filter",
			url:  "/api/v1alpha1/groups",
			queryParams: map[string][]string{
				"deleted":  {""},
				"metadata": {`department=engineering`},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  1, // Only the deleted group
		},
		{
			name:           "invalid metadata query format",
			url:            "/api/v1alpha1/groups",
			queryParams:    map[string][]string{"metadata": {`invalidquerybadbad`}},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata query format",
		},
		{
			name: "invalid metadata key",
			url:  "/api/v1alpha1/groups",
			queryParams: map[string][]string{
				"metadata": {`.... . .-.. .-.. ---=.-. .-- --- .-. .-.. -..`},
			},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata key",
		},
		{
			name:           "empty metadata filter",
			url:            "/api/v1alpha1/groups",
			queryParams:    map[string][]string{"metadata": {""}},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "invalid metadata query format",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", tt.url, nil)

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

			r.listGroups(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body)

				return
			}

			body := w.Body.String()
			groups := []models.Group{}
			err := json.Unmarshal([]byte(body), &groups)
			assert.Nil(t, err, "Expected to unmarshal response body")

			assert.Equal(t, tt.expectedCount, len(groups),
				"Expected %d groups, got %d", tt.expectedCount, len(groups))
		})
	}
}

func TestGroupTestSuite(t *testing.T) {
	suite.Run(t, new(GroupTestSuite))
}
