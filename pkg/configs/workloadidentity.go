package configs

import (
	"context"

	"github.com/metal-toolbox/governor-api/pkg/workloadidentity"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

// AddWorkloadIdentityFlags adds workload identity federation flags to the given FlagSet.
func AddWorkloadIdentityFlags(flags *pflag.FlagSet) {
	flags.String("workload-identity-federation-token-url", "", "workload identity federation token URL")
	viperBindFlag("workload-identity-federation.token-url", flags.Lookup("workload-identity-federation-token-url"))
	flags.String("workload-identity-federation-kube-service-account", "", "Kubernetes service account token file path")
	viperBindFlag("workload-identity-federation.kube-service-account", flags.Lookup("workload-identity-federation-kube-service-account"))
	flags.StringSlice("workload-identity-federation-scopes", []string{}, "workload identity federation scopes")
	viperBindFlag("workload-identity-federation.scopes", flags.Lookup("workload-identity-federation-scopes"))
	flags.String("workload-identity-federation-audience", "", "workload identity federation audience")
	viperBindFlag("workload-identity-federation.audience", flags.Lookup("workload-identity-federation-audience"))
	flags.String("workload-identity-federation-subject-token-type", string(workloadidentity.DefaultSubjectTokenType), "workload identity federation subject token type")
	viperBindFlag("workload-identity-federation.subject-token-type", flags.Lookup("workload-identity-federation-subject-token-type"))
}

// WorkloadIdentityConfig holds the configuration for workload identity federation.
type WorkloadIdentityConfig struct {
	TokenURL           string   `mapstructure:"token-url"`
	KubeServiceAccount string   `mapstructure:"kube-service-account"`
	Scopes             []string `mapstructure:"scopes"`
	Audience           string   `mapstructure:"audience"`
	SubjectTokenType   string   `mapstructure:"subject-token-type"`
}

// ToTokenSource creates a token source from a config
func (c *WorkloadIdentityConfig) ToTokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	stt, err := workloadidentity.NewSubjectTokenTypeFromString(c.SubjectTokenType)
	if err != nil {
		return nil, err
	}

	return workloadidentity.NewTokenSource(
		ctx, c.TokenURL,
		workloadidentity.WithAudience(c.Audience),
		workloadidentity.WithScopes(c.Scopes...),
		workloadidentity.WithKubeSubjectToken(c.KubeServiceAccount, stt),
	), nil
}

// LoadWorkloadIdentityConfig loads the configuration for workload identity federation.
func LoadWorkloadIdentityConfig() (*WorkloadIdentityConfig, error) {
	var cfg WorkloadIdentityConfig

	err := viper.UnmarshalKey("workload-identity-federation", &cfg)
	if err != nil {
		return nil, err
	}

	// Set default subject token type if empty
	if cfg.SubjectTokenType == "" {
		cfg.SubjectTokenType = string(workloadidentity.DefaultSubjectTokenType)
	}

	return &cfg, nil
}
