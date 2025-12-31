package mcp

import "errors"

var (
	// ErrInvalidTokenClaims is returned when the token claims are invalid
	ErrInvalidTokenClaims = errors.New("invalid token claims")
	// ErrTokenExpired is returned when the token is expired
	ErrTokenExpired = errors.New("token is expired")
	// ErrNoTokenFound is returned when no token is found in the request
	ErrNoTokenFound = errors.New("no token found in the request")
)
