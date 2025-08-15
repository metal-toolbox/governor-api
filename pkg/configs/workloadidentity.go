package configs

import (
	"context"

	"github.com/metal-toolbox/governor-api/pkg/workloadidentity"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

// AddWorkloadIdentityFlags adds workload identity federation flags to the given FlagSet.
func AddWorkloadIdentityFlags(flags *pflag.FlagSet) {
	flags.String("workload-identity-token-url", "", "workload identity federation token URL")
	viperBindFlag("workload-identity.token-url", flags.Lookup("workload-identity-token-url"))
	flags.String("workload-identity-kube-service-account", "", "Kubernetes service account token file path")
	viperBindFlag("workload-identity.kube-service-account", flags.Lookup("workload-identity-kube-service-account"))
	flags.StringSlice("workload-identity-scopes", []string{}, "workload identity federation scopes")
	viperBindFlag("workload-identity.scopes", flags.Lookup("workload-identity-scopes"))
	flags.String("workload-identity-audience", "", "workload identity federation audience")
	viperBindFlag("workload-identity.audience", flags.Lookup("workload-identity-audience"))
	flags.String("workload-identity-subject-token-type", string(workloadidentity.DefaultSubjectTokenType), "workload identity federation subject token type")
	viperBindFlag("workload-identity.subject-token-type", flags.Lookup("workload-identity-subject-token-type"))
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

// Validate validates the workload identity configuration.
func (c *WorkloadIdentityConfig) Validate() error {
	_, err := workloadidentity.NewSubjectTokenTypeFromString(c.SubjectTokenType)
	if err != nil {
		return err
	}

	return nil
}
