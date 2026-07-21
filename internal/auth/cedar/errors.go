package cedar

import "errors"

var (
	// ErrCedarRequest is returned when the cedar-agent request cannot be built or sent
	ErrCedarRequest = errors.New("error requesting cedar decision")
	// ErrCedarResponse is returned when the cedar-agent response is missing or undecodable
	ErrCedarResponse = errors.New("error decoding cedar decision")
	// ErrCedarUnexpectedStatus is returned when cedar-agent returns a non-2xx status
	ErrCedarUnexpectedStatus = errors.New("unexpected status from cedar-agent")
)
