package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testIss = "https://idp.example.com"

func TestUserResolutionObserve(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	m, err := NewUserResolution(reg)
	require.NoError(t, err)

	m.Observe(testIss, OutcomeHit, 3*time.Millisecond)
	m.Observe(testIss, OutcomeHit, 30*time.Millisecond)
	m.Observe(testIss, OutcomeRegistered, time.Second)

	want := `
# HELP governor_user_resolution_total Signed-in user resolutions, by issuer and outcome.
# TYPE governor_user_resolution_total counter
governor_user_resolution_total{iss="https://idp.example.com",outcome="hit"} 2
governor_user_resolution_total{iss="https://idp.example.com",outcome="registered"} 1
`
	assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(want), "governor_user_resolution_total"))
}

func TestUserResolutionObserveDuration(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	m, err := NewUserResolution(reg)
	require.NoError(t, err)

	m.Observe(testIss, OutcomeRegistered, time.Second)

	want := `
# HELP governor_user_resolution_duration_seconds Time to resolve a signed-in user, by issuer and outcome.
# TYPE governor_user_resolution_duration_seconds histogram
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.001"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.005"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.01"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.025"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.05"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.1"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.25"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="0.5"} 0
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="1"} 1
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="2.5"} 1
governor_user_resolution_duration_seconds_bucket{iss="https://idp.example.com",outcome="registered",le="+Inf"} 1
governor_user_resolution_duration_seconds_sum{iss="https://idp.example.com",outcome="registered"} 1
governor_user_resolution_duration_seconds_count{iss="https://idp.example.com",outcome="registered"} 1
`
	assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(want), "governor_user_resolution_duration_seconds"))
}

func TestNewUserResolutionReusesRegisteredCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()

	first, err := NewUserResolution(reg)
	require.NoError(t, err)

	second, err := NewUserResolution(reg)
	require.NoError(t, err)

	second.Observe(testIss, OutcomeError, time.Millisecond)

	assert.InDelta(t, 1, testutil.ToFloat64(first.total.WithLabelValues(testIss, string(OutcomeError))), 0)
}

func TestNilUserResolutionObserve(t *testing.T) {
	var m *UserResolution

	assert.NotPanics(t, func() { m.Observe(testIss, OutcomeHit, time.Millisecond) })
}
