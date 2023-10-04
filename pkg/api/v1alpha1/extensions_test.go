package v1alpha1

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	dbm "github.com/metal-toolbox/governor-api/db"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.hollow.sh/toolbox/ginauth"
	"go.uber.org/zap"
)

type ExtensionsTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn
}

func (s *ExtensionsTestSuite) seedTestDB() error {
	testData := []string{
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000001', 'Test Extension', 'some extension', true, 'test-extension', 'online');`,
		`INSERT INTO extensions (id, name, description, enabled, slug, status, deleted_at) 
		VALUES ('00000001-0000-0000-0000-000000000002', 'Deleted Extension', 'some deleted extension', true, 'deleted-extension', 'offline', '2023-07-12 12:00:00.000000+00');`,
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000003', 'Test Extension 3', 'some extension', true, 'test-extension-3', 'online');`,
	}

	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *ExtensionsTestSuite) v1alpha1() *Router {
	return &Router{
		AdminGroups: []string{"governor-admin"},
		AuthMW:      &ginauth.MultiTokenMiddleware{},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		DB:          sqlx.NewDb(s.db, "postgres"),
		EventBus:    eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:      &zap.Logger{},
	}
}

func (s *ExtensionsTestSuite) SetupSuite() {
	s.conn = &mockNATSConn{}

	gin.SetMode(gin.TestMode)

	ts, err := testserver.NewTestServer()
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
}

func (s *ExtensionsTestSuite) TestCreateExtension() {
	r := s.v1alpha1()

	tests := []struct {
		name                 string
		url                  string
		payload              string
		expectedResp         *Extension
		expectedStatus       int
		expectedErrMsg       string
		expectedEventSubject string
		expectedEventPayload *events.Event
	}{
		{
			name:                 "ok",
			url:                  "/api/v1alpha1/extensions",
			expectedStatus:       http.StatusAccepted,
			payload:              `{ "name": "Test Extension 1", "description": "some test", "enabled": true }`,
			expectedEventSubject: "events.extensions",
			expectedEventPayload: &events.Event{
				Action: events.GovernorEventCreate,
			},
			expectedResp: &Extension{&models.Extension{
				Name:        "Test Extension 1",
				Description: "some test",
				Slug:        "test-extension-1",
				Enabled:     true,
			}},
		},
		{
			name:                 "enabled false",
			url:                  "/api/v1alpha1/extensions",
			expectedStatus:       http.StatusAccepted,
			payload:              `{ "name": "Test Extension 2", "description": "some test", "enabled": false }`,
			expectedEventSubject: "events.extensions",
			expectedEventPayload: &events.Event{
				Action: events.GovernorEventCreate,
			},
			expectedResp: &Extension{&models.Extension{
				Name:        "Test Extension 2",
				Description: "some test",
				Slug:        "test-extension-2",
				Enabled:     false,
			}},
		},
		{
			name:                 "duplicate entry",
			url:                  "/api/v1alpha1/extensions",
			payload:              `{ "name": "Test Extension 2", "description": "some test", "enabled": true }`,
			expectedEventSubject: "events.extensions",
			expectedErrMsg:       "duplicate key value violates unique constraint",
			expectedStatus:       http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("POST", tt.url, nil)
			req = req.WithContext(context.Background())
			req.Body = io.NopCloser(bytes.NewBufferString(tt.payload))
			c.Request = req
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.createExtension(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}

			event := &events.Event{}
			err := json.Unmarshal(s.conn.Payload, event)
			assert.Nil(t, err)

			ex := &Extension{}
			body := w.Body.String()
			err = json.Unmarshal([]byte(body), ex)
			assert.Nil(t, err)

			assert.Equal(
				t, tt.expectedResp.Name, ex.Name,
				"Expected extension name %s, got %s", t, tt.expectedResp.Name, ex.Name,
			)

			assert.Equal(
				t, tt.expectedResp.Slug, ex.Slug,
				"Expected extension slug %s, got %s", t, tt.expectedResp.Slug, ex.Slug,
			)

			assert.Equal(
				t, tt.expectedResp.Description, ex.Description,
				"Expected extension description %s, got %s", t, tt.expectedResp.Description, ex.Description,
			)

			assert.Equal(
				t, tt.expectedResp.Enabled, ex.Enabled,
			)

			assert.Equal(
				t, tt.expectedEventSubject, s.conn.Subject,
				"Expected event subject %s, got %s", tt.expectedEventSubject, s.conn.Subject,
			)

			assert.Equal(
				t, tt.expectedEventPayload.Action, event.Action,
				"Expected event action %s, got %s", tt.expectedEventPayload.Action, event.Action,
			)

			assert.Equal(
				t, event.ExtensionID, ex.ID,
				"Expected event extension ID to match response ID",
			)
		})
	}
}

func (s *ExtensionsTestSuite) TestListExtensions() {
	r := s.v1alpha1()

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedErrMsg string
		expectedCount  int
	}{
		{
			name:           "ok",
			url:            "/api/v1alpha1/extensions",
			expectedStatus: http.StatusOK,
			expectedCount:  4,
		},
		{
			name:           "list deleted",
			url:            "/api/v1alpha1/extensions?deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", tt.url, nil)
			req = req.WithContext(context.Background())
			c.Request = req
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.listExtensions(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}

			body := w.Body.String()
			resp := []interface{}{}
			err := json.Unmarshal([]byte(body), &resp)

			assert.Nil(t, err, "expecting unmarshal err to be nil")
			assert.Equal(t, tt.expectedCount, len(resp))
		})
	}
}

func (s *ExtensionsTestSuite) TestGetExtension() {
	r := s.v1alpha1()

	tests := []struct {
		name           string
		url            string
		params         gin.Params
		expectedStatus int
		expectedErrMsg string
	}{
		{
			name:           "get by ID ok",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "get by slug ok",
			url:            "/api/v1alpha1/extensions/test-extension",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension"},
			},
		},
		{
			name:           "get deleted ok",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002?deleted",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000002"},
			},
		},
		{
			name:           "get deleted by slug",
			url:            "/api/v1alpha1/extensions/deleted-extension?deleted",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "deleted-extension"},
			},
		},
		{
			name:           "extension not found by ID",
			url:            "/api/v1alpha1/extensions/nonexistent-extension",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension not found",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "nonexistent-extension"},
			},
		},
		{
			name:           "extension not found by slug",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension not found",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000002"},
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", tt.url, nil)
			req = req.WithContext(context.Background())
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.getExtension(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}
		})
	}
}

func (s *ExtensionsTestSuite) TestUpdateExtension() {
	r := s.v1alpha1()

	tests := []struct {
		name                 string
		url                  string
		params               gin.Params
		payload              string
		expectedResp         *Extension
		expectedStatus       int
		expectedErrMsg       string
		expectedEventSubject string
		expectedEventPayload *events.Event
	}{
		{
			name:                 "disable extension",
			url:                  "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001",
			expectedStatus:       http.StatusAccepted,
			payload:              `{ "name": "Test Extension", "description": "some test", "enabled": false }`,
			expectedEventSubject: "events.extensions",
			expectedEventPayload: &events.Event{
				Action:      events.GovernorEventUpdate,
				ExtensionID: "00000001-0000-0000-0000-000000000001",
			},
			expectedResp: &Extension{&models.Extension{
				Name:        "Test Extension",
				Description: "some test",
				Slug:        "test-extension",
				Enabled:     false,
			}},
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:                 "update by slug",
			url:                  "/api/v1alpha1/extensions/test-extension-1",
			expectedStatus:       http.StatusAccepted,
			payload:              `{ "name": "Test Extension", "description": "some test", "enabled": true }`,
			expectedEventSubject: "events.extensions",
			expectedEventPayload: &events.Event{
				Action:      events.GovernorEventUpdate,
				ExtensionID: "00000001-0000-0000-0000-000000000001",
			},
			expectedResp: &Extension{&models.Extension{
				Name:        "Test Extension",
				Description: "some test",
				Slug:        "test-extension",
				Enabled:     true,
			}},
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "change name",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001",
			expectedStatus: http.StatusBadRequest,
			payload:        `{ "name": "Test Extension 2", "description": "some test", "enabled": false }`,
			expectedErrMsg: "modifying extension name is not allowed",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "extension not found",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002",
			expectedStatus: http.StatusNotFound,
			payload:        `{ "name": "Test Extension 2", "description": "some test", "enabled": false }`,
			expectedErrMsg: "not found",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000002"},
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("PATCH", tt.url, nil)
			req = req.WithContext(context.Background())
			req.Body = io.NopCloser(bytes.NewBufferString(tt.payload))
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.updateExtension(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}

			event := &events.Event{}
			err := json.Unmarshal(s.conn.Payload, event)
			assert.Nil(t, err)

			ex := &Extension{}
			body := w.Body.String()
			err = json.Unmarshal([]byte(body), ex)
			assert.Nil(t, err)

			assert.Equal(
				t, tt.expectedResp.Name, ex.Name,
				"Expected extension name %s, got %s", t, tt.expectedResp.Name, ex.Name,
			)

			assert.Equal(
				t, tt.expectedResp.Slug, ex.Slug,
				"Expected extension slug %s, got %s", t, tt.expectedResp.Slug, ex.Slug,
			)

			assert.Equal(
				t, tt.expectedResp.Description, ex.Description,
				"Expected extension description %s, got %s", t, tt.expectedResp.Description, ex.Description,
			)

			assert.Equal(
				t, tt.expectedResp.Enabled, ex.Enabled,
			)

			assert.Equal(
				t, tt.expectedEventPayload.Action, event.Action,
				"Expected event action %s, got %s", tt.expectedEventPayload.Action, event.Action,
			)

			assert.Equal(
				t, tt.expectedEventSubject, s.conn.Subject,
				"Expected event subject %s, got %s", tt.expectedEventSubject, s.conn.Subject,
			)

			assert.Equal(
				t, tt.expectedEventPayload.ExtensionID, event.ExtensionID,
				"Expected event extension ID %s, got %s", tt.expectedEventPayload.ExtensionID, event.ExtensionID,
			)
		})
	}
}

func (s *ExtensionsTestSuite) TestDeleteExtension() {
	r := s.v1alpha1()

	tests := []struct {
		name           string
		url            string
		params         gin.Params
		expectedStatus int
		expectedErrMsg string
		expectedCount  int
	}{
		{
			name:           "delete by ID ok",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "delete by slug ok",
			url:            "/api/v1alpha1/extensions/test-extension-3",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-3"},
			},
		},
		{
			name:           "extension not found by ID",
			url:            "/api/v1alpha1/extensions/nonexistent-extension",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension not found",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "nonexistent-extension"},
			},
		},
		{
			name:           "extension not found by slug",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension not found",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000002"},
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("GET", tt.url, nil)
			req = req.WithContext(context.Background())
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.getExtension(c)

			assert.Equal(t, tt.expectedStatus, w.Code, "Expected status %d, got %d", tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				body := w.Body.String()
				assert.Contains(
					t, body, tt.expectedErrMsg,
					"Expected error message to contain %q, got %s", tt.expectedErrMsg, body,
				)

				return
			}
		})
	}
}

func TestExtensionSuite(t *testing.T) {
	suite.Run(t, new(ExtensionsTestSuite))
}
