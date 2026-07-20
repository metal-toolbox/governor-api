package cedar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/metal-toolbox/governor-api/internal/auth/authz"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// isAuthorizedPath is the cedar-agent decision endpoint.
	isAuthorizedPath = "/v1/is_authorized"
	// decisionAllow is the cedar-agent decision value that authorizes a request.
	decisionAllow = "Allow"
	// resourcePlaceholder is the fixed resource entity every request carries.
	// Cedar's request format requires a resource, but no policy here constrains on
	// it (the resource is already encoded in the action/scope).
	resourcePlaceholder = `Resource::"na"`
)

// cedarRequest is the cedar-agent /v1/is_authorized request body.
type cedarRequest struct {
	Principal string `json:"principal"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
}

// cedarResponse is the relevant portion of the cedar-agent response body.
type cedarResponse struct {
	Decision string `json:"decision"`
}

// Option configures a sidecarDecider.
type Option func(*CedarDecider)

// WithLogger sets the logger.
func WithLogger(l *zap.Logger) Option {
	return func(d *CedarDecider) { d.logger = l }
}

// WithTracer sets the tracer.
func WithTracer(t trace.Tracer) Option {
	return func(d *CedarDecider) { d.tracer = t }
}

// WithHTTPClient sets a custom HTTP client (its Timeout is otherwise derived
// from the configured request timeout).
func WithHTTPClient(c *http.Client) Option {
	return func(d *CedarDecider) { d.client = c }
}

// WithAuditWriter sets the writer that authorization-decision audit records are
// written to. When unset, decisions are logged through the regular logger.
func WithAuditWriter(w io.Writer) Option {
	return func(d *CedarDecider) { d.auditWriter = w }
}

// CedarDecider is an HTTP client to the local cedar-agent sidecar.
type CedarDecider struct {
	url     string
	timeout time.Duration
	client  *http.Client
	logger  *zap.Logger
	tracer  trace.Tracer

	auditWriter io.Writer
	auditLogger *zap.Logger
}

// CedarDecider implements [authz.Decider]
var _ authz.Decider = (*CedarDecider)(nil)

// NewDecider returns a Decider backed by the local cedar-agent sidecar at url.
func NewDecider(url string, timeout time.Duration, opts ...Option) authz.Decider {
	d := &CedarDecider{
		url:     url,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
		logger:  zap.NewNop(),
		tracer:  otel.GetTracerProvider().Tracer("github.com/metal-toolbox/governor-api/internal/auth/cedar"),

		auditWriter: nil,
	}

	for _, opt := range opts {
		opt(d)
	}

	httpclient := *d.client
	httpclient.Transport = otelhttp.NewTransport(httpclient.Transport)
	d.client = &httpclient

	d.auditLogger = newAuditLogger(d.logger, d.auditWriter)

	return d
}

// newAuditLogger returns a logger that inherits base's level, name, and
// contextual fields but writes JSON records to w. When w is nil it falls back
// to base, so audit records are never silently dropped.
func newAuditLogger(base *zap.Logger, w io.Writer) *zap.Logger {
	if w == nil {
		return base
	}

	return base.WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(w),
			base.Level(),
		)
	}))
}

// Enabled always returns true for a sidecar-backed decider.
func (d *CedarDecider) Enabled() bool { return true }

// Eval POSTs the decision request to the cedar-agent sidecar and reports
// whether the decision is Allow. It fails closed on any error.
func (d *CedarDecider) Eval(ctx context.Context, in authz.AuthzRequest) (bool, error) {
	ctx, span := d.tracer.Start(ctx, "authz.Decider.Eval")
	defer span.End()

	span.SetAttributes(
		attribute.String("authz.principal", in.Principal),
		attribute.String("authz.scope", in.Scope),
	)

	body := cedarRequest{
		Principal: fmt.Sprintf(`Workload::%q`, in.Principal),
		Action:    fmt.Sprintf(`Action::%q`, in.Scope),
		Resource:  resourcePlaceholder,
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return d.fail(span, fmt.Errorf("%w: %w", ErrCedarRequest, err))
	}

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+isAuthorizedPath, bytes.NewReader(buf))
	if err != nil {
		return d.fail(span, fmt.Errorf("%w: %w", ErrCedarRequest, err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return d.fail(span, fmt.Errorf("%w: %w", ErrCedarRequest, err))
	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return d.fail(span, fmt.Errorf("%w: %s", ErrCedarUnexpectedStatus, resp.Status))
	}

	var decision cedarResponse
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		return d.fail(span, fmt.Errorf("%w: %w", ErrCedarResponse, err))
	}

	allow := decision.Decision == decisionAllow

	d.auditLogger.Info("cedar authorization decision",
		zap.String("principal", in.Principal),
		zap.String("scope", in.Scope),
		zap.String("decision", decision.Decision),
		zap.Bool("allow", allow),
	)

	span.SetAttributes(attribute.Bool("authz.allow", allow))

	return allow, nil
}

// fail records the error on the span and returns a fail-closed result.
func (d *CedarDecider) fail(span trace.Span, err error) (bool, error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	d.logger.Warn("cedar authorization failed closed", zap.Error(err))

	return false, err
}
