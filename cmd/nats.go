package cmd

import (
	"context"

	"github.com/metal-toolbox/governor-api/pkg/workloadidentity"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

func newNATSConnection(ctx context.Context, v *viper.Viper) (*nats.Conn, func(), error) {
	opts := []nats.Option{
		nats.Name(appName),
	}

	if credsFile := v.GetString("nats.creds-file"); credsFile != "" {
		opts = append(opts, nats.UserCredentials(credsFile))
	} else {
		return nil, nil, ErrMissingNATSCreds
	}

	if v.GetBool("nats.workload-identity-federation.enabled") {
		tokenURL := v.GetString("nats.workload-identity-federation.token-url")
		kubeServiceAccount := v.GetString("nats.workload-identity-federation.kube-service-account")
		scopes := v.GetStringSlice("nats.workload-identity-federation.scopes")

		if tokenURL == "" {
			return nil, nil, ErrMissingWorkloadIdentityConfig
		}

		ts := workloadidentity.NewTokenSource(
			ctx,
			tokenURL,
			workloadidentity.WithLogger(logger.Desugar()),
			workloadidentity.WithKubeSubjectToken(kubeServiceAccount),
			workloadidentity.WithScopes(scopes...),
		)

		opts = append(opts, nats.UserInfoHandler(func() (string, string) {
			token, err := ts.Token()
			if err != nil {
				logger.Errorw("failed to get an access token from workload identity", "error", err)
				return appName, ""
			}

			return appName, token.AccessToken
		}))
	}

	nc, err := nats.Connect(v.GetString("nats.url"), opts...)
	if err != nil {
		return nil, nil, err
	}

	return nc, nc.Close, nil
}
