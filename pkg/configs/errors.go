package configs

import "errors"

var (
	// ErrInvalidNATSAuthMode is returned when an invalid NATS authentication mode is provided
	ErrInvalidNATSAuthMode = errors.New("invalid NATS authentication mode")
	// ErrMissingNATSCreds is returned when nats creds are not provided
	ErrMissingNATSCreds = errors.New("nats creds are required")
	// ErrCedarURLRequired is returned when Cedar is enabled but no sidecar URL is configured
	ErrCedarURLRequired = errors.New("authz.cedar.url is required when cedar authorization is enabled")
	// ErrInvalidAuthConfig is returned when the oidc cedar configuration cannot be parsed
	ErrInvalidAuthConfig = errors.New("invalid oidc auth configuration")
)
