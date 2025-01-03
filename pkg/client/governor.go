package client

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	governorTimeout         = 10 * time.Second
	governorAPIVersionAlpha = "v1alpha1"
	governorAPIVersionBeta  = "v1beta1"
)

// HTTPDoer implements the standard http.Client interface.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Tokener implements the token interface
type Tokener interface {
	Token(ctx context.Context) (*oauth2.Token, error)
}

// Client is a governor API client
type Client struct {
	url                    string
	clientCredentialConfig Tokener
	logger                 *zap.Logger
	token                  *oauth2.Token
	httpClient             HTTPDoer
	authMux                sync.Mutex
}

// URL returns the governor url
func (c *Client) URL() string {
	return c.url
}

// Option is a functional configuration option
type Option func(r *Client)

// WithURL sets the governor API URL
func WithURL(u string) Option {
	return func(r *Client) {
		r.url = u
	}
}

// WithClientCredentialConfig sets the oauth client credential config
func WithClientCredentialConfig(c *clientcredentials.Config) Option {
	return func(r *Client) {
		r.clientCredentialConfig = c
	}
}

// WithLogger sets logger
func WithLogger(l *zap.Logger) Option {
	return func(r *Client) {
		r.logger = l
	}
}

// WithHTTPClient overrides the default http client
func WithHTTPClient(c HTTPDoer) Option {
	return func(r *Client) {
		r.httpClient = c
	}
}

// NewClient returns a new governor client
func NewClient(opts ...Option) (*Client, error) {
	client := Client{
		logger: zap.NewNop(),
		httpClient: &http.Client{
			Timeout: governorTimeout,
		},
	}

	for _, opt := range opts {
		opt(&client)
	}

	t, err := client.auth(context.TODO())
	if err != nil {
		return nil, err
	}

	client.authMux.Lock()
	defer client.authMux.Unlock()

	client.token = t

	return &client, nil
}

func (c *Client) auth(ctx context.Context) (*oauth2.Token, error) {
	c.logger.Debug("authenticating governor client", zap.Any("clientcredentialconfig", c.clientCredentialConfig))
	return c.clientCredentialConfig.Token(ctx)
}

func (c *Client) refreshAuth(ctx context.Context) error {
	if c.token != nil && !time.Now().After(c.token.Expiry) {
		return nil
	}

	t, err := c.auth(ctx)
	if err != nil {
		return err
	}

	c.authMux.Lock()
	defer c.authMux.Unlock()

	c.token = t

	c.logger.Debug("refreshing governor client authentication")

	return nil
}

func (c *Client) newGovernorRequest(ctx context.Context, method, u string) (*http.Request, error) {
	if err := c.refreshAuth(ctx); err != nil {
		return nil, err
	}

	c.logger.Debug("parsing url", zap.String("url", u))

	queryURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("creating new http request", zap.String("url", queryURL.String()), zap.String("method", method))

	req, err := http.NewRequestWithContext(ctx, method, queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	bearer := "Bearer " + c.token.AccessToken
	req.Header.Add("Authorization", bearer)

	return req, nil
}
