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
	WorkloadIdentityConfig WorkloadIdentityConfig
	IAMRuntimeConfig       IAMRuntimeConfig
	NATSConfig             NATSConfig
}

// AddFlags adds all the flags for the configuration.
func AddFlags(flags *pflag.FlagSet) {
	AddNATSFlags(flags)
	AddIAMRuntimeFlags(flags)
	AddWorkloadIdentityFlags(flags)
}

// LoadConfig loads the configuration for the application.
func LoadConfig() (*Configs, error) {
	var cfg Configs

	wif, err := LoadWorkloadIdentityConfig()
	if err != nil {
		return nil, err
	}

	cfg.WorkloadIdentityConfig = *wif

	nats, err := LoadNATSConfig()
	if err != nil {
		return nil, err
	}

	cfg.NATSConfig = *nats

	iamRuntime, err := LoadIAMRuntimeConfig()
	if err != nil {
		return nil, err
	}

	cfg.IAMRuntimeConfig = *iamRuntime

	return &cfg, nil
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

	switch cfg.NATSConfig.AuthMode {
	case AuthModeWorkloadIdentity:
		ts, err = cfg.WorkloadIdentityConfig.ToTokenSource(ctx)
	case AuthModeIAMRuntime:
		ts, err = cfg.IAMRuntimeConfig.ToTokenSource(ctx)
	}

	if err != nil {
		return nil, err
	}

	return cfg.NATSConfig.ToNATSConnection(name, ts, logger)
}
