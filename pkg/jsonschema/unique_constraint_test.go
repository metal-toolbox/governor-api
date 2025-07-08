package jsonschema

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	dbm "github.com/metal-toolbox/governor-api/db/crdb"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/pressly/goose/v3"
	jsonschemav6 "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type UniqueConstrainTestSuite struct {
	suite.Suite

	db *sql.DB
}

func (s *UniqueConstrainTestSuite) seedTestDB() error {
	testData := []string{
		`INSERT INTO extensions (id, name, description, enabled, slug, status) 
		VALUES ('00000001-0000-0000-0000-000000000001', 'Test Extension', 'some extension', true, 'test-extension', 'online');`,
		`
		INSERT INTO extension_resource_definitions (id, name, description, enabled, slug_singular, slug_plural, version, scope, schema, extension_id) 
		VALUES ('00000001-0000-0000-0000-000000000002', 'Test Resource', 'some-description', true, 'test-resource', 'test-resources', 'v1', 'system',
		'{"$id": "v1.person.test-ex-1","$schema": "https://json-schema.org/draft/2020-12/schema","title": "Person","type": "object","unique": ["firstName", "lastName"],"required": ["firstName", "lastName"],"properties": {"firstName": {"type": "string","description": "The person''s first name.","ui": {"hide": true}},"lastName": {"type": "string","description": "The person''s last name."},"age": {"description": "Age in years which must be equal to or greater than zero.","type": "integer","minimum": 0}}}'::jsonb,
		'00000001-0000-0000-0000-000000000001');
		`,
		`INSERT INTO system_extension_resources (id, resource, extension_resource_definition_id) 
		VALUES ('00000001-0000-0000-0000-000000000003', '{"age": 10, "firstName": "Hello", "lastName": "World"}'::jsonb, '00000001-0000-0000-0000-000000000002');`,
	}

	for _, q := range testData {
		_, err := s.db.Query(q)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *UniqueConstrainTestSuite) SetupSuite() {
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
}

func (s *UniqueConstrainTestSuite) TestCompile() {
	tests := []struct {
		name        string
		inputMap    map[string]interface{}
		expectedErr string
	}{
		{
			name:        "no unique key",
			inputMap:    map[string]interface{}{},
			expectedErr: "",
		},
		{
			name: "invalid unique field type",
			inputMap: map[string]interface{}{
				"unique": 1234,
			},
			expectedErr: "unable to convert",
		},
		{
			name: "unique exists but empty",
			inputMap: map[string]interface{}{
				"unique": []interface{}{},
			},
			expectedErr: "",
		},
		{
			name: "unique exists but required missing",
			inputMap: map[string]interface{}{
				"unique": []interface{}{"a"},
			},
			expectedErr: `cannot apply unique constraint when "required" is not provided`,
		},
		{
			name: "unique exists but required invalid",
			inputMap: map[string]interface{}{
				"unique":   []interface{}{"a"},
				"required": 1234,
			},
			expectedErr: "unable to convert",
		},
		{
			name: "missing properties",
			inputMap: map[string]interface{}{
				"unique":   []interface{}{"a"},
				"required": []interface{}{"a"},
			},
			expectedErr: "cannot apply unique constraint when \"properties\" is not provided or invalid",
		},

		{
			name: "invalid properties type",
			inputMap: map[string]interface{}{
				"unique":     []interface{}{"a"},
				"required":   []interface{}{"a"},
				"properties": "invalidType",
			},
			expectedErr: "cannot apply unique constraint when \"properties\" is not provided or invalid",
		},
		{
			name: "valid unique, required, and properties",
			inputMap: map[string]interface{}{
				"unique":   []interface{}{"a"},
				"required": []interface{}{"a"},
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "unique field not in properties",
			inputMap: map[string]interface{}{
				"unique":   []interface{}{"b"},
				"required": []interface{}{"b"},
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expectedErr: "missing property definition for unique field",
		},
		{
			name: "unique property not in required",
			inputMap: map[string]interface{}{
				"unique":   []interface{}{"a"},
				"required": []interface{}{"b"},
				"properties": map[string]interface{}{
					"a": map[string]interface{}{"type": "string"},
					"b": map[string]interface{}{"type": "string"},
				},
			},
			expectedErr: "unique property needs to be a required property",
		},
	}

	for _, tt := range tests {
		s.Suite.T().Run(tt.name, func(t *testing.T) {
			uc := &UniqueConstraintCompiler{
				ctx: context.Background(),
				db:  nil,
			}

			v, err := uc.Compile()
			assert.Nil(t, err)

			_, err = v.Compile(&jsonschemav6.CompilerContext{}, tt.inputMap)

			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func (s *UniqueConstrainTestSuite) TestValidate() {
	resourceID := "00000001-0000-0000-0000-000000000003"

	tests := []struct {
		name         string
		db           boil.ContextExecutor
		resourceID   *string
		value        interface{}
		uniqueFields map[string]string
		expectedErr  string
		existsReturn bool
		existsErr    error
	}{
		{
			name:         "no DB provided",
			db:           nil,
			value:        map[string]interface{}{"firstName": "test1", "lastName": "test11"},
			uniqueFields: map[string]string{"firstName": "string", "lastName": "string"},
			expectedErr:  "",
		},
		{
			name:         "value not a map",
			db:           s.db,
			value:        "not-a-map",
			uniqueFields: map[string]string{"firstName": "string", "lastName": "string"},
			expectedErr:  "",
		},
		{
			name:         "value matches uniqueFields (string)",
			db:           s.db,
			value:        map[string]interface{}{"firstName": "Hello", "lastName": "World"},
			uniqueFields: map[string]string{"firstName": "string", "lastName": "string"},
			expectedErr:  "unique constraint violation",
		},
		{
			name:         "allow self updates (string)",
			db:           s.db,
			resourceID:   &resourceID,
			value:        map[string]interface{}{"firstName": "Hello", "lastName": "World"},
			uniqueFields: map[string]string{"firstName": "string", "lastName": "string"},
		},
		{
			name:         "empty unique fields",
			db:           s.db,
			value:        map[string]interface{}{"firstName": "Hello", "lastName": "World", "age": 10},
			uniqueFields: map[string]string{},
		},
		{
			name:         "value matches uniqueFields (int)",
			db:           s.db,
			value:        map[string]interface{}{"firstName": "Hello", "age": 10},
			uniqueFields: map[string]string{"firstName": "string", "age": "int"},
			expectedErr:  "unique constraint violation",
		},
		{
			name:         "allow self updates (int)",
			db:           s.db,
			resourceID:   &resourceID,
			value:        map[string]interface{}{"firstName": "Hello", "age": 10},
			uniqueFields: map[string]string{"firstName": "string", "age": "int"},
		},
	}

	erd, err := models.
		ExtensionResourceDefinitions(qm.Where("id = ?", "00000001-0000-0000-0000-000000000002")).
		One(context.Background(), s.db)
	if err != nil {
		panic(err)
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			errHandler := &TestErrorHandler{}

			schema := &UniqueConstraintSchema{
				UniqueFieldTypesMap: tt.uniqueFields,
				ERD:                 erd,
				ctx:                 context.Background(),
				db:                  tt.db,
				ResourceID:          tt.resourceID,

				errHandler: errHandler,
			}

			schema.Validate(&jsonschemav6.ValidatorContext{}, tt.value)

			if tt.expectedErr == "" {
				assert.Nil(t, errHandler.Error)
			} else {
				assert.NotNil(t, errHandler.Error)

				p := message.NewPrinter(language.English)
				errmsg := errHandler.Error.LocalizedString(p)

				assert.Contains(t, errmsg, tt.expectedErr)
			}
		})
	}
}

func TestAssertStringSlice(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    []string
		expectedErr string
	}{
		{
			name:        "valid string slice",
			input:       []interface{}{"foo", "bar", "baz"},
			expected:    []string{"foo", "bar", "baz"},
			expectedErr: "",
		},
		{
			name:        "invalid type",
			input:       "not a slice",
			expected:    nil,
			expectedErr: "to string array",
		},
		{
			name:        "invalid element type",
			input:       []interface{}{"foo", 42, "baz"},
			expected:    nil,
			expectedErr: "to string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := assertStringSlice(tt.input)

			if tt.expectedErr == "" {
				assert.Nil(t, err)

				if !reflect.DeepEqual(actual, tt.expected) {
					t.Errorf("assertStringSlice() = %v, expected %v", actual, tt.expected)
				}
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)

				return
			}
		})
	}
}

func TestUniqueConstraintSuite(t *testing.T) {
	suite.Run(t, new(UniqueConstrainTestSuite))
}
