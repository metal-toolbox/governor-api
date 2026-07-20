package cedar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/metal-toolbox/auditevent"
	"github.com/metal-toolbox/governor-api/internal/auth/authz"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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
	// auditEventType identifies Cedar authorization decisions in the audit log.
	auditEventType = "CedarAuthorization"
	// auditEventSourceValue names the decision source in audit events.
	auditEventSourceValue = "cedar-agent"
	// defaultComponent is the audit event component used when none is configured.
	defaultComponent = "governor-api"
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

// AuditEventWriter writes structured audit events. It is satisfied by
// *auditevent.EventWriter (the same writer governor's HTTP audit middleware
// uses), declared as a narrow interface here so the decider can be tested
// without a real audit sink and so callers can inject any compatible writer.
type AuditEventWriter interface {
	Write(*auditevent.AuditEvent) error
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

// WithAuditWriter sets the writer that authorization-decision audit events are
// written to, as auditevent.AuditEvent records — the same schema and, normally,
// the same underlying log used by governor's HTTP audit middleware. When unset,
// decisions are not audited beyond the regular logger.
func WithAuditWriter(w AuditEventWriter) Option {
	return func(d *CedarDecider) { d.auditWriter = w }
}

// WithComponent overrides the audit event component field (defaults to
// "governor-api", the component name governor's other audit events use).
func WithComponent(component string) Option {
	return func(d *CedarDecider) { d.component = component }
}

// CedarDecider is an HTTP client to the local cedar-agent sidecar.
type CedarDecider struct {
	url     string
	timeout time.Duration
	client  *http.Client
	logger  *zap.Logger
	tracer  trace.Tracer

	component   string
	auditWriter AuditEventWriter
}

// CedarDecider implements [authz.Decider]
var _ authz.Decider = (*CedarDecider)(nil)

// NewDecider returns a Decider backed by the local cedar-agent sidecar at url.
func NewDecider(url string, timeout time.Duration, opts ...Option) authz.Decider {
	d := &CedarDecider{
		url:       url,
		timeout:   timeout,
		client:    &http.Client{Timeout: timeout},
		logger:    zap.NewNop(),
		tracer:    otel.GetTracerProvider().Tracer("github.com/metal-toolbox/governor-api/internal/auth/cedar"),
		component: defaultComponent,
	}

	for _, opt := range opts {
		opt(d)
	}

	httpclient := *d.client
	httpclient.Transport = otelhttp.NewTransport(httpclient.Transport)
	d.client = &httpclient

	return d
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
		return d.fail(span, in, fmt.Errorf("%w: %w", ErrCedarRequest, err))
	}

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+isAuthorizedPath, bytes.NewReader(buf))
	if err != nil {
		return d.fail(span, in, fmt.Errorf("%w: %w", ErrCedarRequest, err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return d.fail(span, in, fmt.Errorf("%w: %w", ErrCedarRequest, err))
	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return d.fail(span, in, fmt.Errorf("%w: %s", ErrCedarUnexpectedStatus, resp.Status))
	}

	var decision cedarResponse
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		return d.fail(span, in, fmt.Errorf("%w: %w", ErrCedarResponse, err))
	}

	allow := decision.Decision == decisionAllow

	outcome := auditevent.OutcomeDenied
	if allow {
		outcome = auditevent.OutcomeSucceeded
	}

	d.audit(in, outcome, decision.Decision)

	span.SetAttributes(attribute.Bool("authz.allow", allow))

	return allow, nil
}

// fail records the error on the span, audits the failure, and returns a
// fail-closed result.
func (d *CedarDecider) fail(span trace.Span, in authz.AuthzRequest, err error) (bool, error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	d.logger.Warn("cedar authorization failed closed", zap.Error(err))
	d.audit(in, auditevent.OutcomeFailed, err.Error())

	return false, err
}

// audit writes a structured audit event for a decision or failure, when an
// audit writer is configured. Write failures are logged but never affect the
// authorization outcome that was already decided.
func (d *CedarDecider) audit(in authz.AuthzRequest, outcome, detail string) {
	if d.auditWriter == nil {
		return
	}

	event := auditevent.NewAuditEvent(
		auditEventType,
		auditevent.EventSource{Type: "internal", Value: auditEventSourceValue},
		outcome,
		map[string]string{"principal": in.Principal},
		d.component,
	).WithTarget(map[string]string{"scope": in.Scope, "detail": detail})

	if err := d.auditWriter.Write(event); err != nil {
		d.logger.Warn("failed to write cedar authorization audit event", zap.Error(err))
	}
}
