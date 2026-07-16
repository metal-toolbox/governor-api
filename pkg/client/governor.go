package client

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	governorTimeout         = 10 * time.Second
	governorAPIVersionAlpha = "v1alpha1"
	governorAPIVersionBeta  = "v1beta1"
)

var tracer = otel.GetTracerProvider().Tracer("http-client.governor-api/v1alpha1")

// Client is a governor API client
type Client struct {
	url        string
	logger     *zap.Logger
	httpClient *http.Client
	ts         oauth2.TokenSource
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
//
// Deprecated: use WithTokenSource instead
func WithClientCredentialConfig(c *clientcredentials.Config) Option {
	return func(r *Client) {
		r.ts = c.TokenSource(context.Background())
	}
}

// WithTokenSource set the oauth token source
func WithTokenSource(ts oauth2.TokenSource) Option {
	return func(r *Client) {
		r.ts = ts
	}
}

// WithLogger sets logger
func WithLogger(l *zap.Logger) Option {
	return func(r *Client) {
		r.logger = l
	}
}

// WithHTTPClient overrides the default http client
func WithHTTPClient(c *http.Client) Option {
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

	if client.ts != nil {
		ts := oauth2.ReuseTokenSource(nil, client.ts)

		if _, err := ts.Token(); err != nil {
			return nil, err
		}

		c := *client.httpClient
		c.Transport = &oauth2.Transport{
			Source: ts,
			Base:   c.Transport,
		}

		client.httpClient = &c
	}

	return &client, nil
}

func (c *Client) newGovernorRequest(ctx context.Context, method, u string) (*http.Request, error) {
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

	return req, nil
}
