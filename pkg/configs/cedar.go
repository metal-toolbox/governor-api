package configs

import (
	"time"
)

// DefaultCedarTimeout is the default request timeout for the cedar-agent sidecar.
const DefaultCedarTimeout = 250 * time.Millisecond

// CedarConfig holds the configuration for the optional Cedar authorization layer.
// When Enabled is false the Cedar verifier is never registered and the auth
// middleware behaves exactly as it does without this configuration.
type CedarConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// URL is the local cedar-agent sidecar base URL, e.g. http://127.0.0.1:8180
	URL string `mapstructure:"url"`
	// Timeout bounds each decision request; a timeout fails closed (deny).
	Timeout time.Duration `mapstructure:"timeout"`
}

// Validate validates the authorization configuration.
func (c *CedarConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.URL == "" {
		return ErrCedarURLRequired
	}

	return nil
}

// TimeoutOrDefault returns the configured timeout, or DefaultCedarTimeout when
// unset. A zero timeout would disable the client deadline, defeating the
// fail-closed guarantee, so callers use this instead of the raw field.
func (c CedarConfig) TimeoutOrDefault() time.Duration {
	if c.Timeout <= 0 {
		return DefaultCedarTimeout
	}

	return c.Timeout
}
