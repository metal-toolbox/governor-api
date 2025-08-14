package configs

import (
	"context"
	"time"

	"github.com/metal-toolbox/iam-runtime-contrib/iamruntime"
	"github.com/metal-toolbox/iam-runtime/pkg/iam/runtime/identity"
	"google.golang.org/grpc"

	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

// DefaultIAMRuntimeTimeoutSeconds is the default timeout for the IAM runtime
const DefaultIAMRuntimeTimeoutSeconds = 15

// AddIAMRuntimeFlags adds iam-runtime flags to the given FlagSet
func AddIAMRuntimeFlags(flags *pflag.FlagSet) {
	flags.String("iam-runtime-socket", "unix:///tmp/runtime.sock", "IAM runtime socket path")
	viperBindFlag("iam-runtime.socket", flags.Lookup("iam-runtime-socket"))
	flags.Duration("iam-runtime-timeout", DefaultIAMRuntimeTimeoutSeconds*time.Second, "IAM runtime timeout")
	viperBindFlag("iam-runtime.timeout", flags.Lookup("iam-runtime-timeout"))
}

// IAMRuntimeConfig holds the configuration for the IAM runtime.
type IAMRuntimeConfig struct {
	Socket  string        `mapstructure:"socket"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type iamruntimeTokenSource struct {
	client  iamruntime.HealthyRuntime
	ctx     context.Context
	timeout time.Duration
}

func (ts *iamruntimeTokenSource) Token() (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(ts.ctx, ts.timeout)
	defer cancel()

	iamCreds, err := ts.client.GetAccessToken(ctx, &identity.GetAccessTokenRequest{})
	if err != nil {
		return nil, err
	}

	return &oauth2.Token{AccessToken: iamCreds.Token}, nil
}

// iamruntimeTokenSource implements [oauth2.TokenSource]
var _ oauth2.TokenSource = (*iamruntimeTokenSource)(nil)

// ToTokenSource creates a new oauth2.TokenSource from the IAM runtime config.
func (c *IAMRuntimeConfig) ToTokenSource(ctx context.Context, dialOpts ...grpc.DialOption) (oauth2.TokenSource, error) {
	rt, err := iamruntime.NewClient(c.Socket, dialOpts...)
	if err != nil {
		return nil, err
	}

	return &iamruntimeTokenSource{
		client:  rt,
		ctx:     ctx,
		timeout: c.Timeout,
	}, nil
}

// Validate validates the IAM runtime configuration.
func (c *IAMRuntimeConfig) Validate() error {
	return nil
}
