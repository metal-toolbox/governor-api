package cmd

import (
	"context"

	"github.com/metal-toolbox/governor-api/pkg/workloadidentity"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
		tt := v.GetString("nats.workload-identity-federation.subject-token-type")
		scopes := v.GetStringSlice("nats.workload-identity-federation.scopes")
		aud := v.GetString("nats.workload-identity-federation.audience")

		if tokenURL == "" {
			return nil, nil, ErrMissingNATSTokenURL
		}

		ts := workloadidentity.NewTokenSource(
			ctx,
			tokenURL,
			workloadidentity.WithLogger(logger.Desugar()),
			workloadidentity.WithAudience(aud),
			workloadidentity.WithScopes(scopes...),
			workloadidentity.WithKubeSubjectToken(
				kubeServiceAccount,
				workloadidentity.SubjectTokenType(tt),
			),
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

	url := v.GetString("nats.url")

	logger.Desugar().Debug(
		"creating NATS connection",
		zap.String("creds-file", v.GetString("nats.creds-file")),
		zap.String("url", url),
	)

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, nil, err
	}

	return nc, nc.Close, nil
}
