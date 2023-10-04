package jsonschema

import "errors"

var (
	// ErrInvalidUniqueProperty is returned when the schema's unique property
	// is invalid
	ErrInvalidUniqueProperty = errors.New(`property "unique" is invalid`)

	// ErrUniqueConstraintViolation is returned when an object violates the unique
	// constrain
	ErrUniqueConstraintViolation = errors.New("unique constraint violation")
)
