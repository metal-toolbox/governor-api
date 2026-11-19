// Package metrics holds the governor-api Prometheus metrics that are not
// request metrics.
package metrics

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Outcome is the result of resolving a signed-in user.
type Outcome string

// The outcomes of a user resolution.
const (
	// OutcomeHit means the (issuer, external ID) pair was already recorded.
	OutcomeHit Outcome = "hit"
	// OutcomeLinked means the sign-in was linked by email to an existing user.
	OutcomeLinked Outcome = "linked"
	// OutcomeRegistered means the sign-in created a new user.
	OutcomeRegistered Outcome = "registered"
	// OutcomeRejectedConflict means the sign-in conflicts with an existing user.
	OutcomeRejectedConflict Outcome = "rejected_conflict"
	// OutcomeRejectedUnregistered means the sign-in matches no user and may not create one.
	OutcomeRejectedUnregistered Outcome = "rejected_unregistered"
	// OutcomeError means the resolution failed on a downstream error.
	OutcomeError Outcome = "error"
)

// UserResolution counts and times signed-in user resolutions by issuer and outcome.
type UserResolution struct {
	total    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewUserResolution creates the user-resolution metrics and registers them
// with reg. When reg already holds them, for example because a second API
// server was built in the same process, the registered collectors are reused.
func NewUserResolution(reg prometheus.Registerer) (*UserResolution, error) {
	labels := []string{"iss", "outcome"}

	total, err := register(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "governor_user_resolution_total",
		Help: "Signed-in user resolutions, by issuer and outcome.",
	}, labels))
	if err != nil {
		return nil, err
	}

	duration, err := register(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "governor_user_resolution_duration_seconds",
		Help:    "Time to resolve a signed-in user, by issuer and outcome.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, labels))
	if err != nil {
		return nil, err
	}

	return &UserResolution{total: total, duration: duration}, nil
}

// Observe records one resolution that took d. A nil *UserResolution records nothing.
func (m *UserResolution) Observe(iss string, outcome Outcome, d time.Duration) {
	if m == nil {
		return
	}

	m.total.WithLabelValues(iss, string(outcome)).Inc()
	m.duration.WithLabelValues(iss, string(outcome)).Observe(d.Seconds())
}

// register registers c with reg, or returns the collector reg already holds
// under the same descriptor.
func register[C prometheus.Collector](reg prometheus.Registerer, c C) (C, error) {
	err := reg.Register(c)
	if err == nil {
		return c, nil
	}

	var are prometheus.AlreadyRegisteredError
	if errors.As(err, &are) {
		if existing, ok := are.ExistingCollector.(C); ok {
			return existing, nil
		}
	}

	var zero C

	return zero, err
}
