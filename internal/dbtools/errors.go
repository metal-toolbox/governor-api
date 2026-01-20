package dbtools

import "errors"

var (
	// ErrUnknownRequestKind is returned a request kind is unknown
	ErrUnknownRequestKind = errors.New("request kind is unrecognized")
	// ErrInvalidMetadataQueryFormat is returned when a metadata query string is not in key=value format
	ErrInvalidMetadataQueryFormat = errors.New("invalid metadata query format")
	// ErrInvalidMetadataKey is returned when a metadata key doesn't match the required pattern
	ErrInvalidMetadataKey = errors.New("invalid metadata key")
)
