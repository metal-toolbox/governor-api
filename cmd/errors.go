package cmd

import "errors"

var (
	// ErrMissingNATSCreds is returned when nats creds are not provided
	ErrMissingNATSCreds = errors.New("nats creds are required")
	// ErrMissingNATSTokenURL is returned when workload identity federation config is incomplete
	ErrMissingNATSTokenURL = errors.New("workload identity federation token URL is required")
	// ErrNoOIDCConfig is returned when no enabled oidc provider is configured
	ErrNoOIDCConfig = errors.New("no oidc auth configs found")
	// ErrUnsupportedDBDriver is returned for a db.driver other than postgres or crdb
	ErrUnsupportedDBDriver = errors.New("unsupported database driver")
)
