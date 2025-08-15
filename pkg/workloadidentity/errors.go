package workloadidentity

import "errors"

var (
	// ErrExchangingToken is returned when there is an error exchanging tokens
	ErrExchangingToken = errors.New("error exchanging token")
	// ErrInvalidSubjectTokenType is returned when an invalid subject token type is provided
	ErrInvalidSubjectTokenType = errors.New("invalid subject token type")
)
