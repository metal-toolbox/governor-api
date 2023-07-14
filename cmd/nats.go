package cmd

import (
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

	nc, err := nats.Connect(viper.GetString("nats.url"), opts...)
	if err != nil {
		return nil, nil, err
	}

	return nc, nc.Close, nil
}
