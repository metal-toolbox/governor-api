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

type optionals struct {
	logger *zap.Logger
	ts     oauth2.TokenSource
}

func newOptionals() *optionals {
	return &optionals{logger: zap.NewNop()}
}

// Opt is a functional option type for configuring optional parameters.
type Opt func(*optionals)

// WithLogger sets the logger in the options.
func WithLogger(l *zap.Logger) Opt {
	return func(o *optionals) {
		o.logger = l
	}
}

// WithTokenSource sets the token source in the options.
func WithTokenSource(ts oauth2.TokenSource) Opt {
	return func(o *optionals) {
		o.ts = ts
	}
}

// NATSConn is a shorthand that checks the NATS auth mode and creates a NATS connection
// based on all the configs available
func (cfg *Configs) NATSConn(ctx context.Context, name string, opts ...Opt) (*nats.Conn, error) {
	var (
		ts  oauth2.TokenSource
		err error
	)

	switch cfg.NATS.AuthMode {
	case AuthModeWorkloadIdentity:
		ts, err = cfg.WorkloadIdentity.ToTokenSource(ctx)
	case AuthModeIAMRuntime:
		ts, err = cfg.IAMRuntime.ToTokenSource(ctx)
	}

	if err != nil {
		return nil, err
	}

	if ts != nil {
		opts = append(opts, WithTokenSource(ts))
	}

	return cfg.NATS.ToNATSConnection(name, opts...)
}
