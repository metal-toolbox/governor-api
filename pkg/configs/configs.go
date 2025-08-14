package configs

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// viperBindFlag provides a wrapper around the viper bindings that handles error checks
func viperBindFlag(name string, flag *pflag.Flag) {
	err := viper.BindPFlag(name, flag)
	if err != nil {
		panic(err)
	}
}

// Configs holds the configuration for the application.
type Configs struct {
	WorkloadIdentity WorkloadIdentityConfig `mapstructure:"workload-identity"`
	IAMRuntime       IAMRuntimeConfig       `mapstructure:"iam-runtime"`
	NATS             NATSConfig             `mapstructure:"nats"`
}

// AddFlags adds all the flags for the configuration.
func AddFlags(flags *pflag.FlagSet) {
	AddNATSFlags(flags)
	AddIAMRuntimeFlags(flags)
	AddWorkloadIdentityFlags(flags)
}

// Validate validates all the configs
func (cfg *Configs) Validate() error {
	if err := cfg.WorkloadIdentity.Validate(); err != nil {
		return err
	}

	if err := cfg.IAMRuntime.Validate(); err != nil {
		return err
	}

	if err := cfg.NATS.Validate(); err != nil {
		return err
	}

	return nil
}

// NATSConn is a shorthand that checks the NATS auth mode and creates a NATS connection
// based on all the configs available
func (cfg *Configs) NATSConn(ctx context.Context, name string, logger *zap.Logger) (*nats.Conn, error) {
	var (
		ts  oauth2.TokenSource
		err error
	)

	if logger == nil {
		logger = zap.NewNop()
	}

	switch cfg.NATS.AuthMode {
	case AuthModeWorkloadIdentity:
		ts, err = cfg.WorkloadIdentity.ToTokenSource(ctx)
	case AuthModeIAMRuntime:
		ts, err = cfg.IAMRuntime.ToTokenSource(ctx)
	}

	if err != nil {
		return nil, err
	}

	return cfg.NATS.ToNATSConnection(name, ts, logger)
}
