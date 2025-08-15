package configs

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// NATSAuthMode represents the authentication modes for NATS
type NATSAuthMode string

const (
	// AuthModeCredsFileOnly is the auth mode for using a credentials file only
	AuthModeCredsFileOnly NATSAuthMode = "creds-file-only"
	// AuthModeWorkloadIdentity is the auth mode for using workload identity
	AuthModeWorkloadIdentity NATSAuthMode = "workload-identity"
	// AuthModeIAMRuntime is the auth mode for using IAM runtime
	AuthModeIAMRuntime NATSAuthMode = "iam-runtime"
)

// AddNATSFlags adds NATS flags to the given FlagSet
func AddNATSFlags(flags *pflag.FlagSet) {
	flags.String("nats-url", "nats://127.0.0.1:4222", "NATS server connection url")
	viperBindFlag("nats.url", flags.Lookup("nats-url"))
	flags.String("nats-creds-file", "", "Path to the file containing the NATS credentials file")
	viperBindFlag("nats.creds-file", flags.Lookup("nats-creds-file"))
	flags.String("nats-subject-prefix", "governor.events", "prefix for NATS subjects")
	viperBindFlag("nats.subject-prefix", flags.Lookup("nats-subject-prefix"))
	flags.String("nats-auth-mode", string(AuthModeCredsFileOnly), "NATS authentication mode")
	viperBindFlag("nats.auth-mode", flags.Lookup("nats-auth-mode"))
}

// NATSConfig holds the configuration for NATS
type NATSConfig struct {
	URL           string       `mapstructure:"url"`
	CredsFile     string       `mapstructure:"creds-file"`
	SubjectPrefix string       `mapstructure:"subject-prefix"`
	AuthMode      NATSAuthMode `mapstructure:"auth-mode"`
}

// ToNATSConnection creates a NATS connection based on a config
func (c *NATSConfig) ToNATSConnection(name string, opts ...Opt) (*nats.Conn, error) {
	if c.CredsFile == "" {
		return nil, fmt.Errorf("%w: NATS credentials file is required", ErrMissingNATSCreds)
	}

	o := newOptionals()

	for _, opt := range opts {
		opt(o)
	}

	logger := o.logger
	ts := o.ts

	natsopts := []nats.Option{
		nats.Name(name),
		nats.UserCredentials(c.CredsFile),
	}

	if c.AuthMode != AuthModeCredsFileOnly {
		if ts == nil {
			return nil, fmt.Errorf("%w: token source is required", ErrMissingNATSCreds)
		}

		natsopts = append(natsopts, nats.UserInfoHandler(func() (string, string) {
			token, err := ts.Token()
			if err != nil {
				logger.Error("failed to get an access token from workload identity", zap.Error(err))
				return name, ""
			}

			return name, token.AccessToken
		}))
	}

	return nats.Connect(c.URL, natsopts...)
}

// Validate validates the NATS configuration.
func (c *NATSConfig) Validate() error {
	switch c.AuthMode {
	case AuthModeCredsFileOnly, AuthModeWorkloadIdentity, AuthModeIAMRuntime:
	default:
		return fmt.Errorf("%w: %s", ErrInvalidNATSAuthMode, c.AuthMode)
	}

	return nil
}
