package jsonschema

import (
	"context"
	"fmt"
	"reflect"

	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// JSONSchemaUniqueConstraint is a JSON schema extension that provides a
// "unique" property of type array
var JSONSchemaUniqueConstraint = jsonschema.MustCompileString(
	"https://governor/json-schemas/unique.json",
	`{
		"properties": {
			"unique": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`,
)

// UniqueConstraintSchema is the schema struct for the unique constraint JSON schema extension
type UniqueConstraintSchema struct {
	UniqueFieldTypesMap map[string]string
	ERD                 *models.ExtensionResourceDefinition
	ResourceID          *string
	ctx                 context.Context
	db                  boil.ContextExecutor
}

// UniqueConstraintSchema implements jsonschema.ExtSchema
var _ jsonschema.ExtSchema = (*UniqueConstraintSchema)(nil)

// Validate checks the uniqueness of the provided value against a database
// to ensure the unique constraint is satisfied.
func (s *UniqueConstraintSchema) Validate(_ jsonschema.ValidationContext, v interface{}) error {
	// Skip validation if no database is provided
	if s.db == nil {
		return nil
	}

	// Skip validation if no constraint is provided
	if len(s.UniqueFieldTypesMap) == 0 {
		return nil
	}

	// Try to assert the provided value as a map, skip validation otherwise
	mappedValue, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}

	qms := []qm.QueryMod{}

	if s.ResourceID != nil {
		qms = append(qms, qm.Where("id != ?", *s.ResourceID))
	}

	for k, v := range mappedValue {
		_, exists := s.UniqueFieldTypesMap[k]
		if !exists {
			continue
		}

		qms = append(qms, qm.Where(`resource->>? = ?`, k, v))
	}

	var exists bool

	var err error

	if s.ERD.Scope == "system" {
		exists, err = s.ERD.SystemExtensionResources(qms...).Exists(s.ctx, s.db)
	} else {
		exists, err = s.ERD.UserExtensionResources(qms...).Exists(s.ctx, s.db)
	}

	if err != nil {
		return &jsonschema.ValidationError{
			Message: err.Error(),
		}
	}

	if exists {
		return &jsonschema.ValidationError{
			InstanceLocation: s.ERD.Name,
			KeywordLocation:  "unique",
			Message:          ErrUniqueConstraintViolation.Error(),
		}
	}

	return nil
}

// UniqueConstraintCompiler is the compiler struct for the unique constraint JSON schema extension
type UniqueConstraintCompiler struct {
	ERD        *models.ExtensionResourceDefinition
	ResourceID *string
	ctx        context.Context
	db         boil.ContextExecutor
}

// UniqueConstraintCompiler implements jsonschema.ExtCompiler
var _ jsonschema.ExtCompiler = (*UniqueConstraintCompiler)(nil)

// Compile compiles the unique constraint JSON schema extension
func (uc *UniqueConstraintCompiler) Compile(
	_ jsonschema.CompilerContext, m map[string]interface{},
) (jsonschema.ExtSchema, error) {
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

	return uc.compileUniqueConstraint(uniqueFields, requiredMap, propertiesMap)
}

func (uc *UniqueConstraintCompiler) compileUniqueConstraint(
	uniqueFields []string, requiredMap map[string]bool, propertiesMap map[string]interface{},
) (jsonschema.ExtSchema, error) {
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

	return &UniqueConstraintSchema{resultUniqueFields, uc.ERD, uc.ResourceID, uc.ctx, uc.db}, nil
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
