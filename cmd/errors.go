package cmd

import "errors"

// ErrMissingNATSCreds is returned when nats creds are not provided
var ErrMissingNATSCreds = errors.New("nats creds are required")
