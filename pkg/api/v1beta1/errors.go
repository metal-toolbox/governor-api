package v1beta1

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

// ErrInvalidQueryParameterValue is returned when the query parameter value is invalid
var ErrInvalidQueryParameterValue = errors.New("InvalidQueryParameterValue")

// ErrInvalidFunctionParameter is returned when a function parameter fails an assertion
var ErrInvalidFunctionParameter = errors.New("InvalidFunctionParameter")

func sendError(c *gin.Context, code int, msg string) {
	payload := struct {
		Error string `json:"error,omitempty"`
	}{msg}

	c.AbortWithStatusJSON(code, payload)
}

func invalidQueryParameterValue(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidQueryParameterValue, msg)
}

func invalidFunctionParameter(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidQueryParameterValue, msg)
}
