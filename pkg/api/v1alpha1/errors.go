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
