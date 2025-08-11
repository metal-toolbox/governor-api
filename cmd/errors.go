package cmd

import "errors"

var (
	// ErrMissingNATSCreds is returned when nats creds are not provided
	ErrMissingNATSCreds = errors.New("nats creds are required")
	// ErrMissingWorkloadIdentityConfig is returned when workload identity federation config is incomplete
	ErrMissingWorkloadIdentityConfig = errors.New("workload identity federation token URL is required")
)
