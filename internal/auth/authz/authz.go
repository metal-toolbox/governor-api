// Package authz provides a Verifier that delegates authorization to an external
// Decider (in place of the usual scope check), along with the Decider interface
// that concrete backends implement.
package authz

import (
	"context"
)

// Decider decides whether a principal may perform a scoped action.
type Decider interface {
	// Enabled reports whether external authorization is active.
	Enabled() bool
	// Eval returns true if the request is permitted. It fails closed: any
	// transport, timeout, status, or decoding error returns (false, err).
	Eval(ctx context.Context, in AuthzRequest) (bool, error)
}

// AuthzRequest is a coarse principal -> scope authorization request.
type AuthzRequest struct {
	// Principal is the authenticated subject.
	Principal string
	// Scope is governor's own scope string, e.g. "create:governor:users".
	Scope string
}

// NoopDecider returns a disabled Decider whose Enabled reports false and whose
// Eval always denies. Useful when external authorization is not configured.
func NoopDecider() Decider { return noopDecider{} }

type noopDecider struct{}

func (noopDecider) Enabled() bool                                    { return false }
func (noopDecider) Eval(context.Context, AuthzRequest) (bool, error) { return false, nil }
