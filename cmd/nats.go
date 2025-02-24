package cmd

import (
	"context"

	"github.com/metal-toolbox/iam-runtime-contrib/iamruntime"
	"github.com/metal-toolbox/iam-runtime/pkg/iam/runtime/identity"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

func newNATSConnection(v *viper.Viper) (*nats.Conn, func(), error) {
	opts := []nats.Option{
		nats.Name(appName),
	}

	if credsFile := v.GetString("nats.creds-file"); credsFile != "" {
		opts = append(opts, nats.UserCredentials(credsFile))
	} else {
		return nil, nil, ErrMissingNATSCreds
	}

	if viper.GetBool("nats.use-runtime-access-token") {
		rt, err := iamruntime.NewClient(viper.GetString("iam-runtime.socket"))
		if err != nil {
			return nil, nil, err
		}

		timeout := viper.GetDuration("iam-runtime.timeout")

		opts = append(opts, nats.UserInfoHandler(func() (string, string) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			iamCreds, err := rt.GetAccessToken(ctx, &identity.GetAccessTokenRequest{})
			if err != nil {
				logger.Errorw("failed to get an access token from the iam-runtime", "error", err)
				return appName, ""
			}

			return appName, iamCreds.Token
		}))
	}

	nc, err := nats.Connect(viper.GetString("nats.url"), opts...)
	if err != nil {
		return nil, nil, err
	}

	return nc, nc.Close, nil
}
