package v1alpha1

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

var (
	// ErrInvalidChar is returned when use input contains invalid character(s)
	ErrInvalidChar = errors.New("invalid characters in group name string")
	// ErrEmptyInput is returned when user input is empty
	ErrEmptyInput = errors.New("name or description cannot be empty")
	// ErrPublishUpdateNotificationPreferences is returned when there's an error occurred
	// while publishing update event
	ErrPublishUpdateNotificationPreferences = errors.New(
		"failed to publish notification update event, downstream changes may be delayed",
	)
	// ErrNotificationPreferencesEmptyInput is returned when there's an empty
	// update request received
	ErrNotificationPreferencesEmptyInput = errors.New("empty request is not allowed")
)

func newErrPublishUpdateNotificationPreferences(msg string) error {
	return fmt.Errorf("%w: %s", ErrPublishUpdateNotificationPreferences, msg)
}

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
