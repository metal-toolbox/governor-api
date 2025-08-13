package cmd

import "errors"

var (
	// ErrMissingNATSCreds is returned when nats creds are not provided
	ErrMissingNATSCreds = errors.New("nats creds are required")
	// ErrMissingNATSTokenURL is returned when workload identity federation config is incomplete
	ErrMissingNATSTokenURL = errors.New("workload identity federation token URL is required")
)
