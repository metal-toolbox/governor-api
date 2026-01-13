package mcp

import (
	"context"
	"errors"
	"net/http"

	"go.hollow.sh/toolbox/ginjwt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

const (
	// DefaultMetadataBaseURL is the default base URL for the MCP server metadata.
	DefaultMetadataBaseURL = "http://localhost:3001"
	// WellKnownOAuthProtectedResourcePath is the path for the OAuth protected resource metadata.
	WellKnownOAuthProtectedResourcePath = "/.well-known/oauth-protected-resource"
)

// GovernorMCPServer represents the MCP server for Governor.
type GovernorMCPServer struct {
	httpserver      *http.Server
	authConfigs     []ginjwt.AuthConfig
	metadataBaseURL string

	httpclient *http.Client
	govURL     string

	logger *zap.Logger
	tracer trace.Tracer
}

// Option defines a functional option for configuring the GovernorMCPServer.
type Option func(*GovernorMCPServer)

// WithLogger sets the logger for the GovernorMCPServer.
func WithLogger(logger *zap.Logger) Option {
	return func(s *GovernorMCPServer) {
		s.logger = logger.With(zap.String("component", "mcp-server"))
	}
}

// WithTracer sets the tracer for the GovernorMCPServer.
func WithTracer(tracer trace.Tracer) Option {
	return func(s *GovernorMCPServer) {
		s.tracer = tracer
	}
}

// WithAuthConfigs sets the authentication configurations for the GovernorMCPServer.
func WithAuthConfigs(authConfigs []ginjwt.AuthConfig) Option {
	return func(s *GovernorMCPServer) {
		s.authConfigs = authConfigs
	}
}

// WithMetadataBaseURL sets the metadata base URL for the GovernorMCPServer.
func WithMetadataBaseURL(url string) Option {
	return func(s *GovernorMCPServer) {
		s.metadataBaseURL = url
	}
}

// WithHTTPClient sets the HTTP client for the GovernorMCPServer, the mcp server
// uses this client to make outbound requests to governor api.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(s *GovernorMCPServer) {
		s.httpclient = httpClient
	}
}

// NewGovernorMCPServer creates a new instance of GovernorMCPServer with the provided options.
func NewGovernorMCPServer(httpserver *http.Server, govURL string, opts ...Option) *GovernorMCPServer {
	s := &GovernorMCPServer{
		govURL:          govURL,
		httpserver:      httpserver,
		httpclient:      &http.Client{},
		logger:          zap.NewNop(),
		tracer:          noop.NewTracerProvider().Tracer("governor-mcp-server"),
		metadataBaseURL: DefaultMetadataBaseURL,
	}

	for _, opt := range opts {
		opt(s)
	}

	v1alpha1Handler := s.v1alpha1()

	mux := http.NewServeMux()
	// Wrap with tracing first, then auth middleware to ensure
	// each HTTP request creates a fresh span that propagates to tool calls
	mux.Handle(
		"/v1alpha1",
		otelhttp.NewHandler(s.authMiddleware()(v1alpha1Handler), "mcp/v1alpha1"),
	)

	s.authMetadataHandler(mux)

	s.httpserver.Handler = mux

	return s
}

// Start starts the MCP server.
func (s *GovernorMCPServer) Start() error {
	s.logger.Info("starting MCP server")

	if err := s.httpserver.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the MCP server.
func (s *GovernorMCPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down MCP server")

	if err := s.httpserver.Close(); err != nil {
		return err
	}

	if err := s.httpserver.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}
