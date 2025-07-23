package jsonschema

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	jsonschemav6 "github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	// SchemaURL is the URL for the unique constraint JSON schema extension
	SchemaURL = "https://governor/json-schemas/unique.json"
	// UniqueConstraintSchemaStr is the JSON schema string for the unique constraint JSON schema extension
	UniqueConstraintSchemaStr = `{
		"properties": {
			"unique": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`
)

// UniqueConstraintSchema is the schema struct for the unique constraint JSON schema extension
type UniqueConstraintSchema struct {
	UniqueFieldTypesMap map[string]string
	ERD                 *models.ExtensionResourceDefinition
	ResourceID          *string
	ctx                 context.Context
	db                  boil.ContextExecutor

	errHandler ValidatorErrorHandler
}

// UniqueConstraintSchema implements jsonschema.SchemaExt
var _ jsonschemav6.SchemaExt = (*UniqueConstraintSchema)(nil)

// Validate checks the uniqueness of the provided value against a database
// to ensure the unique constraint is satisfied.
func (s *UniqueConstraintSchema) Validate(ctx *jsonschemav6.ValidatorContext, v interface{}) {
	// Skip validation if no database is provided
	if s.db == nil {
		return
	}

	// Skip validation if no constraint is provided
	if len(s.UniqueFieldTypesMap) == 0 {
		return
	}

	// Try to assert the provided value as a map, skip validation otherwise
	mappedValue, ok := v.(map[string]interface{})
	if !ok {
		return
	}

	qms := []qm.QueryMod{}
	props := []string{}

	if s.ResourceID != nil {
		qms = append(qms, qm.Where("id != ?", *s.ResourceID))
	}

	for k, v := range mappedValue {
		_, exists := s.UniqueFieldTypesMap[k]
		if !exists {
			continue
		}

		// Convert value to string for JSON comparison since JSONB ->> operator returns text
		strValue := fmt.Sprintf("%v", v)
		qms = append(qms, qm.Where(`resource->>? = ?`, k, strValue))
		props = append(props, k)
	}

	var (
		exists bool
		err    error
	)

	if s.ERD.Scope == "system" {
		exists, err = s.ERD.SystemExtensionResources(qms...).Exists(s.ctx, s.db)
	} else {
		exists, err = s.ERD.UserExtensionResources(qms...).Exists(s.ctx, s.db)
	}

	if err != nil {
		s.errHandler.AddError(ctx, &ErrUniquePropertyViolation{
			Message: err.Error(),
		})

		return
	}

	if exists {
		s.errHandler.AddError(ctx, &ErrUniquePropertyViolation{
			Message: fmt.Sprintf("resource with the same [%s] already exists", strings.Join(props, ",")),
		})
	}
}

// SchemaExtension is an interface for JSON schema extensions
type SchemaExtension interface {
	// Compile compiles the JSON schema extension
	Compile() (*jsonschemav6.Vocabulary, error)
}

// UniqueConstraintCompiler is the compiler struct for the unique constraint JSON schema extension
type UniqueConstraintCompiler struct {
	ERD        *models.ExtensionResourceDefinition
	ResourceID *string
	ctx        context.Context
	db         boil.ContextExecutor
}

// UniqueConstraintCompiler implements SchemaExtension
var _ SchemaExtension = (*UniqueConstraintCompiler)(nil)

// Compile compiles the unique constraint JSON schema extension
func (uc *UniqueConstraintCompiler) Compile() (*jsonschemav6.Vocabulary, error) {
	schemaJSON, err := jsonschemav6.UnmarshalJSON(strings.NewReader(UniqueConstraintSchemaStr))
	if err != nil {
		return nil, err
	}

	c := jsonschemav6.NewCompiler()

	if err := c.AddResource(SchemaURL, schemaJSON); err != nil {
		return nil, err
	}

	schema, err := c.Compile(SchemaURL)
	if err != nil {
		return nil, err
	}

	vocab := &jsonschemav6.Vocabulary{
		URL:     SchemaURL,
		Schema:  schema,
		Compile: uc.compileUniqueConstraint,
	}

	return vocab, nil
}

func (uc *UniqueConstraintCompiler) compileUniqueConstraint(
	_ *jsonschemav6.CompilerContext, m map[string]interface{},
) (jsonschemav6.SchemaExt, error) {
	unique, ok := m["unique"]
	if !ok {
		// If "unique" is not in the map, skip processing
		return nil, nil
	}

	uniqueFields, err := assertStringSlice(unique)
	if err != nil {
		return nil, err
	}

	if len(uniqueFields) == 0 {
		// unique property is not provided, skip
		return nil, nil
	}

	required, ok := m["required"]
	if !ok {
		return nil, fmt.Errorf(
			`%w: cannot apply unique constraint when "required" is not provided`,
			ErrInvalidUniqueProperty,
		)
	}

	requiredFields, err := assertStringSlice(required)
	if err != nil {
		return nil, err
	}

	requiredMap := make(map[string]bool, len(requiredFields))
	for _, f := range requiredFields {
		requiredMap[f] = true
	}

	propertiesMap, ok := m["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(
			`%w: cannot apply unique constraint when "properties" is not provided or invalid`,
			ErrInvalidUniqueProperty,
		)
	}

	// map fieldName => fieldType
	resultUniqueFields := make(map[string]string)

	for _, fieldName := range uniqueFields {
		if !requiredMap[fieldName] {
			return nil, fmt.Errorf(
				`%w: unique property needs to be a required property, "%s" is not in "required"`,
				ErrInvalidUniqueProperty,
				fieldName,
			)
		}

		prop, ok := propertiesMap[fieldName]
		if !ok {
			return nil, fmt.Errorf(
				`%w: missing property definition for unique field "%s"`,
				ErrInvalidUniqueProperty,
				fieldName,
			)
		}

		fieldType, ok := prop.(map[string]interface{})["type"].(string)
		if !ok || !isValidType(fieldType) {
			return nil, fmt.Errorf(
				`%w: invalid type "%s" for unique field "%s"`,
				ErrInvalidUniqueProperty,
				fieldType,
				fieldName,
			)
		}

		resultUniqueFields[fieldName] = fieldType
	}

	return &UniqueConstraintSchema{
		resultUniqueFields, uc.ERD, uc.ResourceID,
		uc.ctx, uc.db,
		&V6ValidationContextErrorHandler{},
	}, nil
}

// Checks if the provided field type is valid for unique constraints
func isValidType(fieldType string) bool {
	return fieldType == "string" || fieldType == "number" || fieldType == "integer" || fieldType == "boolean"
}

// helper function to assert string slice type
func assertStringSlice(value interface{}) ([]string, error) {
	values, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf(
			`%w: unable to convert %v to string array`,
			ErrInvalidUniqueProperty,
			reflect.TypeOf(value),
		)
	}

	strs := make([]string, len(values))

	for i, v := range values {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf(
				`%w: unable to convert %v to string`,
				ErrInvalidUniqueProperty,
				reflect.TypeOf(v),
			)
		}

		strs[i] = str
	}

	return strs, nil
}
