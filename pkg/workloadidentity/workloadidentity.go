package workloadidentity

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	defaultReuseExpiry    = 30 * time.Second
	defaultRequestTimeout = 15 * time.Second
)

// SubjectTokenFn is a function that retrieves the subject token.
type SubjectTokenFn func(context.Context) (*oauth2.Token, error)

// WorkloadTokenSource implements oauth2.TokenSource.
type WorkloadTokenSource struct {
	subjectTokenFn   SubjectTokenFn
	scopes           []string
	tokenURL         string
	audience         string
	httpClient       *http.Client
	tokenReuseExpiry time.Duration
	requestTimeout   time.Duration
	subjectTokenType SubjectTokenType
	token            *oauth2.Token
	subjectToken     *oauth2.Token

	ctx    context.Context
	logger *zap.Logger
	tracer trace.Tracer
}

// Option is a functional config option for the TokenSource.
type Option func(*WorkloadTokenSource)

// NewWorkloadTokenSource returns a WorkLoadTokenSource struct
func NewWorkloadTokenSource(
	ctx context.Context,
	tokenurl string,
	opts ...Option,
) *WorkloadTokenSource {
	w := &WorkloadTokenSource{
		ctx:              ctx,
		tokenURL:         tokenurl,
		httpClient:       http.DefaultClient,
		requestTimeout:   defaultRequestTimeout,
		tokenReuseExpiry: defaultReuseExpiry,
		subjectTokenType: DefaultSubjectTokenType,
		logger:           zap.NewNop(),
		tracer:           otel.GetTracerProvider().Tracer("github.com/metal-toolbox/governor-api:workloadidentity"),
	}

	w.subjectTokenFn = w.kubeServiceAccountTokenFn(defaultKubeServiceAccountPath)

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// NewTokenSource returns a TokenSource that uses workload identity.
func NewTokenSource(
	ctx context.Context,
	tokenurl string,
	opts ...Option,
) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, NewWorkloadTokenSource(ctx, tokenurl, opts...))
}

// WithKubeSubjectToken sets the function to retrieve subject token from a kubernetes
// service account token file.
func WithKubeSubjectToken(tokenPath string, tt SubjectTokenType) Option {
	return func(w *WorkloadTokenSource) {
		w.subjectTokenFn = w.kubeServiceAccountTokenFn(tokenPath)
		w.subjectTokenType = tt
	}
}

// WithSubjectTokenFn sets the function to retrieve the subject token.
// It also sets the subject token type.
func WithSubjectTokenFn(fn SubjectTokenFn, tt SubjectTokenType) Option {
	return func(w *WorkloadTokenSource) {
		w.subjectTokenFn = fn
		w.subjectTokenType = tt
	}
}

// WithLogger sets the logger for the WorkloadTokenSource.
func WithLogger(logger *zap.Logger) Option {
	return func(w *WorkloadTokenSource) {
		w.logger = logger
	}
}

// WithTokenReuseExpiry sets the token reuse expiry for the WorkloadTokenSource.
func WithTokenReuseExpiry(d time.Duration) Option {
	return func(w *WorkloadTokenSource) {
		w.tokenReuseExpiry = d
	}
}

// WithScopes sets the scopes for the access token.
func WithScopes(scopes ...string) Option {
	return func(w *WorkloadTokenSource) {
		w.scopes = scopes
	}
}

// WithAudience sets the audience for the token exchange.
func WithAudience(aud string) Option {
	return func(w *WorkloadTokenSource) {
		w.audience = aud
	}
}

// WithHTTPClient sets a custom HTTP client for token requests.
func WithHTTPClient(client *http.Client) Option {
	return func(w *WorkloadTokenSource) {
		w.httpClient = client
	}
}

// WithRequestTimeout sets the request timeout for token requests.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(w *WorkloadTokenSource) {
		w.requestTimeout = timeout
	}
}

// WithTracer sets the tracer for the WorkloadTokenSource.
func WithTracer(tracer trace.Tracer) Option {
	return func(w *WorkloadTokenSource) {
		w.tracer = tracer
	}
}
