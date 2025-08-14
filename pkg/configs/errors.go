package configs

import "errors"

var (
	// ErrInvalidNATSAuthMode is returned when an invalid NATS authentication mode is provided
	ErrInvalidNATSAuthMode = errors.New("invalid NATS authentication mode")
	// ErrMissingNATSCreds is returned when nats creds are not provided
	ErrMissingNATSCreds = errors.New("nats creds are required")
)
