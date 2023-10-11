package v1alpha1

import (
	"errors"

	"github.com/gin-gonic/gin"
)

var (
	// ErrInvalidChar is returned when use input contains invalid character(s)
	ErrInvalidChar = errors.New("invalid characters in group name string")
	// ErrEmptyInput is returned when user input is empty
	ErrEmptyInput = errors.New("name or description cannot be empty")
	// ErrUnknownRequestKind is returned a request kind is unknown
	ErrUnknownRequestKind = errors.New("request kind is unrecognized")
	// ErrGetDeleteResourcedWithSlug is returned when user tries to query a deleted
	// resource with slug
	ErrGetDeleteResourcedWithSlug = errors.New("unable to get deleted resource by slug, use the id")
	// ErrExtensionNotFound is returned when an extension is not found
	ErrExtensionNotFound = errors.New("extension does not exist")
	// ErrERDNotFound is returned when an extension resource definition is not found
	ErrERDNotFound = errors.New("ERD does not exist")
	// ErrNoUserProvided is returned when no user is provided
	ErrNoUserProvided = errors.New("neither user-id nor context user where provided")
	// ErrExtensionResourceNotFound is returned when an extension resource is not found
	ErrExtensionResourceNotFound = errors.New("extension resource does not exist")
	// ErrUserNotFound is returned when a user is not found
	ErrUserNotFound = errors.New("user does not exist")
)

func sendError(c *gin.Context, code int, msg string) {
	payload := struct {
		Error string `json:"error,omitempty"`
	}{msg}

	c.AbortWithStatusJSON(code, payload)
}

func sendErrorWithDisplayMessage(c *gin.Context, code int, errorMessage, displayMessage string) {
	payload := struct {
		Error          string `json:"error,omitempty"`
		DisplayMessage string `json:"displayMessage,omitempty"`
	}{errorMessage, displayMessage}

	c.AbortWithStatusJSON(code, payload)
}
