//nolint:noctx
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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	dbm "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.hollow.sh/toolbox/ginauth"
	"go.uber.org/zap"
)

type ExtensionResourceDefinitionsTestSuite struct {
	suite.Suite

	db   *sql.DB
	conn *mockNATSConn
}

func (s *ExtensionResourceDefinitionsTestSuite) seedTestDB() error {
	testData := []string{
		// extensions
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000001', 'Test Extension 1', 'some extension', true, 'test-extension-1', 'online');`,
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000002', 'Test Extension 2', 'some extension', true, 'test-extension-2', 'online');`,
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000003', 'Test Extension 2', 'some extension', true, 'test-extension-3', 'online');`,

		// ERDs
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
		VALUES ('00000002-0000-0000-0000-000000000001', 'User Resource', 'some-description', true, 'user-resource', 'user-resources', 'v1', 'user',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000001');
		`,
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id, deleted_at) 
		VALUES ('00000002-0000-0000-0000-000000000002', 'User Resource', 'some-description', true, 'user-resource', 'user-resources', 'v1', 'user',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000001', '2023-07-12 12:00:00.000000+00');
		`,

		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
		VALUES ('00000002-0001-0000-0000-000000000001', 'User Resource', 'some-description', true, 'user-resource', 'user-resources', 'v1', 'user',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000003');
		`,
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
		VALUES ('00000002-0001-0000-0000-000000000002', 'User Resource', 'some-description', true, 'user-1-resource', 'user-1-resources', 'v1', 'user',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000003');
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

func (s *ExtensionResourceDefinitionsTestSuite) v1alpha1() *Router {
	return &Router{
		AdminGroups: []string{"governor-admin"},
		AuthMW:      &ginauth.MultiTokenMiddleware{},
		AuditMW:     ginaudit.NewJSONMiddleware("governor-api", io.Discard),
		DB:          sqlx.NewDb(s.db, "postgres"),
		EventBus:    eventbus.NewClient(eventbus.WithNATSConn(s.conn)),
		Logger:      &zap.Logger{},
	}
}

func (s *ExtensionResourceDefinitionsTestSuite) SetupSuite() {
	s.conn = &mockNATSConn{}

	gin.SetMode(gin.TestMode)

	s.db = dbtools.NewPGTestServer(s.T())

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(s.db, "migrations"); err != nil {
		panic("migration failed - could not set up test db")
	}

	if err := s.seedTestDB(); err != nil {
		panic("db setup failed - could not seed test db: " + err.Error())
	}
}

func (s *ExtensionResourceDefinitionsTestSuite) TestIsValidSlug() {
	tests := []struct {
		slug     string
		expected bool
	}{
		{"valid-slug", true},
		{"invalid_slug", false},
		{"INVALID", false},
		{"-invalid-start", false},
		{"invalid-end-", false},
		{"another-valid-slug123", true},
		{"slug with spaces", false},
		{"slug-with--double-hyphens", false},
		{"slug-with-special@chars", false},
		{"slug with white s p a c e s", false},
		{"", false},
		{"a", true},
		{"-a", false},
		{"a-", false},
	}

	for _, tt := range tests {
		s.T().Run(tt.slug, func(t *testing.T) {
			actual := isValidSlug(tt.slug)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func (s *ExtensionResourceDefinitionsTestSuite) TestCreateExtensionResourceDefinition() {
	r := s.v1alpha1()

	tests := []struct {
		name                 string
		url                  string
		payload              string
		params               gin.Params
		expectedResp         *ExtensionResourceDefinition
		expectedStatus       int
		expectedErrMsg       string
		expectedEventSubject string
		expectedEventPayload *events.Event
	}{
		{
			name:                 "ok",
			url:                  "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus:       http.StatusAccepted,
			expectedEventSubject: "events.extension.erds",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			expectedEventPayload: &events.Event{
				Action:      events.GovernorEventCreate,
				ExtensionID: "00000001-0000-0000-0000-000000000002",
			},
			payload: `{
				"name": "Test ERD 1",
				"description": "some test",
				"slug_singular": "test-1-resource",
				"slug_plural": "test-1-resources",
				"version": "v1",
				"schema": {
					"$id": "v1.test-1-resource.test-ex-1",
					"$schema": "https://json-schema.org/draft/2020-12/schema",
					"title": "Test 1 ERD",
					"type": "object",
					"required": [ "firstName", "lastName" ],
					"properties": {
						"firstName": {
							"type": "string",
							"description": "The person's first name.",
							"ui": {
								"hide": true
							}
						},
						"lastName": {
							"type": "string",
							"description": "The person's last name."
						},
						"age": {
							"description": "Age in years which must be equal to or greater than zero.",
							"type": "integer",
							"minimum": 0
						}
					}
				},
				"enabled": true,
				"scope": "system"
			}`,
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "Test ERD 1",
				Description:  "some test",
				SlugSingular: "test-1-resource",
				SlugPlural:   "test-1-resources",
				Version:      "v1",
				Scope:        "system",
				Enabled:      true,
			}},
		},
		{
			name:                 "create by extension ID",
			url:                  "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002",
			expectedStatus:       http.StatusAccepted,
			expectedEventSubject: "events.extension.erds",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			expectedEventPayload: &events.Event{
				Action:      events.GovernorEventCreate,
				ExtensionID: "00000001-0000-0000-0000-000000000002",
			},
			payload: `{
				"name": "Test ERD 1",
				"description": "some test",
				"slug_singular": "test-1-resource",
				"slug_plural": "test-1-resources",
				"version": "v2",
				"schema": {
					"$id": "v2.test-1-resource.test-ex-1",
					"$schema": "https://json-schema.org/draft/2020-12/schema",
					"title": "Test 1 ERD",
					"type": "object",
					"required": [ "firstName", "lastName" ],
					"properties": {
						"firstName": {
							"type": "string",
							"description": "The person's first name.",
							"ui": {
								"hide": true
							}
						},
						"lastName": {
							"type": "string",
							"description": "The person's last name."
						},
						"age": {
							"description": "Age in years which must be equal to or greater than zero.",
							"type": "integer",
							"minimum": 0
						}
					}
				},
				"enabled": true,
				"scope": "system"
			}`,
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "Test ERD 1",
				Description:  "some test",
				SlugSingular: "test-1-resource",
				SlugPlural:   "test-1-resources",
				Version:      "v1",
				Scope:        "system",
				Enabled:      true,
			}},
		},
		{
			name:           "bad schema",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
				"name": "Test ERD 1",
				"description": "some test",
				"slug_singular": "test-1-resource",
				"slug_plural": "test-1-resources",
				"version": "v1",
				"schema": "{",
				"enabled": true,
				"scope": "system"
			}`,
			expectedErrMsg: "ERD schema is not valid",
		},
		{
			name:           "extension not found",
			url:            "/api/v1alpha1/extensions/non-exists-extension",
			expectedStatus: http.StatusNotFound,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "non-exists-extension"},
			},
			expectedErrMsg: "extension not found",
			payload: `{
				"name": "Test ERD 1",
				"slug_singular": "test-1-resource",
				"slug_plural": "test-1-resources",
				"version": "v2",
				"schema": {},
				"enabled": true,
				"scope": "system"
			}`,
		},
		{
			name:           "missing name",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"schema": {},
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "name is required",
		},
		{
			name:           "missing slug_singular",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"schema": {},
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "ERD slugs are required",
		},
		{
			name:           "missing slug_plural",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"version": "v2",
					"schema": {},
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "ERD slugs are required",
		},
		{
			name:           "invalid slug_singular",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test 1 resource",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"schema": {},
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "one or both of ERD slugs are invalid",
		},
		{
			name:           "invalid slug_plural",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources-",
					"version": "v2",
					"schema": {},
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "one or both of ERD slugs are invalid",
		},
		{
			name:           "missing version",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources",
					"schema": {},
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "version is required",
		},
		{
			name:           "missing schema",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"enabled": true,
					"scope": "system"
			}`,
			expectedErrMsg: "schema is required",
		},
		{
			name:           "missing enabled",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"schema": {},
					"scope": "system"
			}`,
			expectedErrMsg: "enabled is required",
		},
		{
			name:           "missing scope",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"schema": {},
					"enabled": true
			}`,
			expectedErrMsg: "scope is required",
		},
		{
			name:           "bad scope",
			url:            "/api/v1alpha1/extensions/test-extension-2",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-2"},
			},
			payload: `{
					"name": "Test ERD 1",
					"slug_singular": "test-1-resource",
					"slug_plural": "test-1-resources",
					"version": "v2",
					"scope": "bad scope",
					"schema": {},
					"enabled": true
			}`,
			expectedErrMsg: "invalid ERD scope",
		},
		{
			name:                 "slug singular conflict",
			url:                  "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002",
			expectedStatus:       http.StatusBadRequest,
			expectedEventSubject: "events.extension.erds",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000002"},
			},
			payload: `{
				"name": "Test ERD 1",
				"description": "some test",
				"slug_singular": "test-1-resource",
				"slug_plural": "test-1-resources",
				"version": "v2",
				"schema": {},
				"enabled": true,
				"scope": "system"
			}`,
			expectedErrMsg: "duplicate key value violates unique constraint",
		},
		{
			name:                 "slug plural conflict",
			url:                  "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000002",
			expectedStatus:       http.StatusBadRequest,
			expectedEventSubject: "events.extension.erds",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000002"},
			},
			payload: `{
				"name": "Test ERD 1",
				"description": "some test",
				"slug_singular": "test-11-resource",
				"slug_plural": "test-1-resources",
				"version": "v2",
				"schema": {},
				"enabled": true,
				"scope": "system"
			}`,
			expectedErrMsg: "duplicate key value violates unique constraint",
		},
		{
			name:                 "duplicate slug in different extension",
			url:                  "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000003",
			expectedStatus:       http.StatusAccepted,
			expectedEventSubject: "events.extension.erds",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000003"},
			},
			payload: `{
				"name": "Test ERD 1",
				"description": "some test",
				"slug_singular": "test-1-resource",
				"slug_plural": "test-1-resources",
				"version": "v2",
				"schema": {},
				"enabled": true,
				"scope": "system"
			}`,
			expectedEventPayload: &events.Event{
				Action:      events.GovernorEventCreate,
				ExtensionID: "00000001-0000-0000-0000-000000000003",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "Test ERD 1",
				Description:  "some test",
				SlugSingular: "test-1-resource",
				SlugPlural:   "test-1-resources",
				Version:      "v1",
				Scope:        "system",
				Enabled:      true,
			}},
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
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.createExtensionResourceDefinition(c)

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

			erd := &ExtensionResourceDefinition{}
			body := w.Body.String()
			err = json.Unmarshal([]byte(body), erd)
			assert.Nil(t, err)

			assert.Equal(
				t, tt.expectedResp.Name, erd.Name,
				"Expected ERD name %s, got %s", t, tt.expectedResp.Name, erd.Name,
			)

			assert.Equal(
				t, tt.expectedResp.SlugSingular, erd.SlugSingular,
				"Expected ERD singular slug %s, got %s", t, tt.expectedResp.SlugSingular, erd.SlugSingular,
			)

			assert.Equal(
				t, tt.expectedResp.SlugPlural, erd.SlugPlural,
				"Expected ERD plural slug %s, got %s", t, tt.expectedResp.SlugPlural, erd.SlugPlural,
			)

			assert.Equal(
				t, tt.expectedResp.Description, erd.Description,
				"Expected ERD description %s, got %s", t, tt.expectedResp.Description, erd.Description,
			)

			assert.Equal(
				t, tt.expectedResp.Scope, erd.Scope,
				"Expected ERD scope %s, got %s", t, tt.expectedResp.Scope, erd.Scope,
			)

			assert.Equal(
				t, tt.expectedResp.Enabled, erd.Enabled,
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
				t, tt.expectedEventPayload.ExtensionID, event.ExtensionID,
				"Expected event extension ID to match response ID",
			)

			assert.Equal(
				t, erd.ID, event.ExtensionResourceDefinitionID,
				"Expected event ERD ID to match response ID",
			)
		})
	}
}

func (s *ExtensionResourceDefinitionsTestSuite) TestListExtensionResourceDefinitions() {
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
			name:           "list by ID",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001/erds",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "list by slug",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
			},
		},
		{
			name:           "list by ID deleted",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001/erds?deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "list by slug deleted",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds?deleted",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
			},
		},
		{
			name:           "extension not found",
			url:            "/api/v1alpha1/extensions/non-existence/erds",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension not found",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "non-existence"},
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

			r.listExtensionResourceDefinitions(c)

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

func (s *ExtensionResourceDefinitionsTestSuite) TestGetExtensionResourceDefinition() {
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
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001/erds/00000002-0000-0000-0000-000000000001",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
				gin.Param{Key: "erd-id-slug", Value: "00000002-0000-0000-0000-000000000001"},
			},
		},
		{
			name:           "get by slug ok",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:           "get deleted ok",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/00000002-0000-0000-0000-000000000002?deleted",
			expectedStatus: http.StatusOK,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "00000002-0000-0000-0000-000000000002"},
			},
		},
		{
			name:           "get deleted by slug",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1?deleted",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:           "extension not found",
			url:            "/api/v1alpha1/extensions/nonexistent-extension/erds/some-resource/v1",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "nonexistent-extension"},
				gin.Param{Key: "erd-id-slug", Value: "some-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
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

			r.getExtensionResourceDefinition(c)

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

func (s *ExtensionResourceDefinitionsTestSuite) TestUpdateExtensionResourceDefinition() {
	r := s.v1alpha1()

	tests := []struct {
		name                 string
		url                  string
		params               gin.Params
		payload              string
		expectedResp         *ExtensionResourceDefinition
		expectedStatus       int
		expectedErrMsg       string
		expectedEventSubject string
		expectedEventPayload *events.Event
	}{
		{
			name:                 "update by ID ok",
			url:                  "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000001/erds/00000002-0000-0000-0000-000000000001",
			expectedStatus:       http.StatusAccepted,
			payload:              `{ "description": "some test 1" }`,
			expectedEventSubject: "events.extension.erds",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000001"},
				gin.Param{Key: "erd-id-slug", Value: "00000002-0000-0000-0000-000000000001"},
			},
			expectedEventPayload: &events.Event{
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "User Resource",
				Description:  "some test 1",
				SlugSingular: "user-resource",
				SlugPlural:   "user-resources",
				Version:      "v1",
				Scope:        "user",
				Enabled:      true,
			}},
		},
		{
			name:                 "update by slug ok",
			url:                  "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			expectedStatus:       http.StatusAccepted,
			expectedEventSubject: "events.extension.erds",
			payload:              `{ "description": "some test 1" }`,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedEventPayload: &events.Event{
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "User Resource",
				Description:  "some test 1",
				SlugSingular: "user-resource",
				SlugPlural:   "user-resources",
				Version:      "v1",
				Scope:        "user",
				Enabled:      true,
			}},
		},
		{
			name:           "extension not found",
			url:            "/api/v1alpha1/extensions/nonexistent-extension/erds/some-resource/v1",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
			payload:        `{}`,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "nonexistent-extension"},
				gin.Param{Key: "erd-id-slug", Value: "some-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:           "attempt modify slug singular",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:        `{ "slug_singular": "user-resource-1" }`,
			expectedErrMsg: "ERD slugs are immutable",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:                 "include unmodified slug singular",
			url:                  "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:              `{ "slug_singular": "user-resource" }`,
			expectedEventSubject: "events.extension.erds",
			expectedStatus:       http.StatusAccepted,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedEventPayload: &events.Event{
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "User Resource",
				Description:  "some test 1",
				SlugSingular: "user-resource",
				SlugPlural:   "user-resources",
				Version:      "v1",
				Scope:        "user",
				Enabled:      true,
			}},
		},
		{
			name:           "attempt modify slug plural",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:        `{ "slug_plural": "user-resource-1" }`,
			expectedErrMsg: "ERD slugs are immutable",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:                 "include unmodified slug plural",
			url:                  "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:              `{ "slug_plural": "user-resources" }`,
			expectedEventSubject: "events.extension.erds",
			expectedStatus:       http.StatusAccepted,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedEventPayload: &events.Event{
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "User Resource",
				Description:  "some test 1",
				SlugSingular: "user-resource",
				SlugPlural:   "user-resources",
				Version:      "v1",
				Scope:        "user",
				Enabled:      true,
			}},
		},
		{
			name:           "attempt modify scope",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:        `{ "scope": "system" }`,
			expectedErrMsg: "ERD scope is immutable",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:                 "include unmodified scope",
			url:                  "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:              `{ "scope": "user" }`,
			expectedEventSubject: "events.extension.erds",
			expectedStatus:       http.StatusAccepted,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedEventPayload: &events.Event{
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "User Resource",
				Description:  "some test 1",
				SlugSingular: "user-resource",
				SlugPlural:   "user-resources",
				Version:      "v1",
				Scope:        "user",
				Enabled:      true,
			}},
		},
		{
			name:           "attempt modify version",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:        `{ "version": "v2" }`,
			expectedErrMsg: "ERD version is immutable",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:                 "include unmodified version",
			url:                  "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:              `{ "version": "v1" }`,
			expectedEventSubject: "events.extension.erds",
			expectedStatus:       http.StatusAccepted,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
			expectedEventPayload: &events.Event{
				Action:                        events.GovernorEventUpdate,
				ExtensionID:                   "00000001-0000-0000-0000-000000000001",
				ExtensionResourceDefinitionID: "00000002-0000-0000-0000-000000000001",
			},
			expectedResp: &ExtensionResourceDefinition{&models.ExtensionResourceDefinition{
				Name:         "User Resource",
				Description:  "some test 1",
				SlugSingular: "user-resource",
				SlugPlural:   "user-resources",
				Version:      "v1",
				Scope:        "user",
				Enabled:      true,
			}},
		},
		{
			name:           "attempt modify schema",
			url:            "/api/v1alpha1/extensions/test-extension-1/erds/user-resource/v1`",
			payload:        `{ "schema": "{}" }`,
			expectedErrMsg: "ERD schema is immutable",
			expectedStatus: http.StatusBadRequest,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-1"},
				gin.Param{Key: "erd-id-slug", Value: "user-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
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

			r.updateExtensionResourceDefinition(c)

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

			erd := &ExtensionResourceDefinition{}
			body := w.Body.String()
			err = json.Unmarshal([]byte(body), erd)
			assert.Nil(t, err)

			assert.Equal(
				t, tt.expectedResp.Name, erd.Name,
				"Expected ERD name %s, got %s", t, tt.expectedResp.Name, erd.Name,
			)

			assert.Equal(
				t, tt.expectedResp.SlugSingular, erd.SlugSingular,
				"Expected ERD singular slug %s, got %s", t, tt.expectedResp.SlugSingular, erd.SlugSingular,
			)

			assert.Equal(
				t, tt.expectedResp.SlugPlural, erd.SlugPlural,
				"Expected ERD plural slug %s, got %s", t, tt.expectedResp.SlugPlural, erd.SlugPlural,
			)

			assert.Equal(
				t, tt.expectedResp.Description, erd.Description,
				"Expected ERD description %s, got %s", t, tt.expectedResp.Description, erd.Description,
			)

			assert.Equal(
				t, tt.expectedResp.Scope, erd.Scope,
				"Expected ERD scope %s, got %s", t, tt.expectedResp.Scope, erd.Scope,
			)

			assert.Equal(
				t, tt.expectedResp.Enabled, erd.Enabled,
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
				t, tt.expectedEventPayload.ExtensionID, event.ExtensionID,
				"Expected event extension ID to match response ID",
			)

			assert.Equal(
				t, tt.expectedEventPayload.ExtensionResourceDefinitionID, event.ExtensionResourceDefinitionID,
				"Expected event ERD ID to match response ID",
			)
		})
	}
}

func (s *ExtensionResourceDefinitionsTestSuite) TestDeleteExtensionResourceDefinition() {
	r := s.v1alpha1()

	tests := []struct {
		name           string
		url            string
		params         gin.Params
		expectedStatus int
		expectedErrMsg string
	}{
		{
			name:           "delete by ID ok",
			url:            "/api/v1alpha1/extensions/00000001-0000-0000-0000-000000000003/erds/00000002-0001-0000-0000-000000000001",
			expectedStatus: http.StatusAccepted,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "00000001-0000-0000-0000-000000000003"},
				gin.Param{Key: "erd-id-slug", Value: "00000002-0001-0000-0000-000000000001"},
			},
		},
		{
			name:           "delete by slug ok",
			url:            "/api/v1alpha1/extensions/test-extension-3/erds/user-1-resource/v1`",
			expectedStatus: http.StatusAccepted,
			params: gin.Params{
				gin.Param{Key: "eid", Value: "test-extension-3"},
				gin.Param{Key: "erd-id-slug", Value: "user-1-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
		{
			name:           "extension not found",
			url:            "/api/v1alpha1/extensions/nonexistent-extension/erds/some-resource/v1",
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: "extension does not exist",
			params: gin.Params{
				gin.Param{Key: "eid", Value: "nonexistent-extension"},
				gin.Param{Key: "erd-id-slug", Value: "some-resource"},
				gin.Param{Key: "erd-version", Value: "v1"},
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			auditID := uuid.New().String()

			req, _ := http.NewRequest("DELETE", tt.url, nil)
			req = req.WithContext(context.Background())
			c.Request = req
			c.Params = tt.params
			c.Set(ginaudit.AuditIDContextKey, auditID)

			r.deleteExtensionResourceDefinition(c)

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

func TestExtensionResourceDefinitionsSuite(t *testing.T) {
	suite.Run(t, new(ExtensionResourceDefinitionsTestSuite))
}
