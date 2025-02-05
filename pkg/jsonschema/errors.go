package jsonschema

import (
	"errors"

	schemav6 "github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/message"
)

var (
	// ErrInvalidUniqueProperty is returned when the schema's unique property
	// is invalid
	ErrInvalidUniqueProperty = errors.New(`property "unique" is invalid`)

	// ErrUniqueConstraintViolation is returned when an object violates the unique
	// constrain
	ErrUniqueConstraintViolation = errors.New("unique constraint violation")
)

// ErrUniquePropertyViolation is a jsonschema/v6 error that is returned when a
// unique property is violated
type ErrUniquePropertyViolation struct {
	Message string
}

// ErrUniquePropertyViolation implements the schemav6.ErrorKind interface
var _ schemav6.ErrorKind = (*ErrUniquePropertyViolation)(nil)

// KeywordPath returns the keyword path for the unique property violation
func (*ErrUniquePropertyViolation) KeywordPath() []string {
	return []string{"unique"}
}

// LocalizedString returns the localized string for the unique property violation
func (e *ErrUniquePropertyViolation) LocalizedString(p *message.Printer) string {
	return p.Sprintf("%s: %s", ErrUniqueConstraintViolation.Error(), e.Message)
}

// ValidatorErrorHandler is an interface to store validation errors
type ValidatorErrorHandler interface {
	AddError(*schemav6.ValidatorContext, schemav6.ErrorKind)
}

// V6ValidationContextErrorHandler is an implementation of the
// ValidatorErrorHandler interface for jsonschema/v6
type V6ValidationContextErrorHandler struct{}

// V6ValidationContextErrorHandler implements the ValidatorErrorHandler interface
var _ ValidatorErrorHandler = (*V6ValidationContextErrorHandler)(nil)

// AddError saves the error in the context
func (v *V6ValidationContextErrorHandler) AddError(ctx *schemav6.ValidatorContext, err schemav6.ErrorKind) {
	ctx.AddError(err)
}

// TestErrorHandler is an implementation of the ValidatorErrorHandler interface
// that is used for testing
type TestErrorHandler struct {
	Error schemav6.ErrorKind
}

// TestErrorHandler implements the ValidatorErrorHandler interface
var _ ValidatorErrorHandler = (*TestErrorHandler)(nil)

// AddError saves the error
func (t *TestErrorHandler) AddError(_ *schemav6.ValidatorContext, err schemav6.ErrorKind) {
	t.Error = err
}
