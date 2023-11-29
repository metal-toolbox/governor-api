package eventbus

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/google/uuid"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/nats-io/nats.go"
)

const (
	defaultSubject = "events"
	natsTracerName = "github.com/metal-toolbox/governor-api:nats"
)

type conn interface {
	Publish(subject string, data []byte) error
	PublishMsg(m *nats.Msg) error
	Drain() error
}

// Client is an event bus client with some configuration
type Client struct {
	conn   conn
	logger *zap.Logger
	prefix string
	tracer trace.Tracer
}

// Option is a functional configuration option for governor eventing
type Option func(c *Client)

// NewClient configures and establishes a new event bus client connection
func NewClient(opts ...Option) *Client {
	client := Client{
		logger: zap.NewNop(),
		prefix: defaultSubject,
		tracer: otel.GetTracerProvider().Tracer(natsTracerName),
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

// WithNATSConn sets the nats connection
func WithNATSConn(nc conn) Option {
	return func(c *Client) {
		c.conn = nc
	}
}

// WithNATSPrefix sets the nats subscription prefix
func WithNATSPrefix(p string) Option {
	return func(c *Client) {
		c.prefix = p
	}
}

// WithLogger sets the client logger
func WithLogger(l *zap.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// Shutdown drains the event bus and closes the connections
func (c *Client) Shutdown() error {
	return c.conn.Drain()
}

// Publish an event on the event bus
func (c *Client) Publish(ctx context.Context, sub string, event *events.Event) error {
	if event == nil {
		return ErrEmptyEvent
	}

	subject := c.prefix + "." + sub

	c.logger.Info("publishing event to the event bus", zap.String("subject", subject), zap.Any("action", event.Action))

	ctx, span := c.tracer.Start(ctx, "events.nats.PublishEvent", trace.WithAttributes(
		attribute.String("events.action", event.Action),
		attribute.String("event.subject", subject),
		attribute.String("event.actor_id", event.ActorID),
	))

	defer span.End()

	// Propagate trace context into the message for the subscriber
	var mapCarrier propagation.MapCarrier = make(map[string]string)

	otel.GetTextMapPropagator().Inject(ctx, mapCarrier)

	event.TraceContext = mapCarrier

	payload, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	var headers nats.Header = event.Headers

	if headers == nil {
		headers = nats.Header{}
	}

	if v := headers.Get(events.GovernorEventCorrelationIDHeader); v == "" {
		correlationsID := uuid.New().String()
		headers.Set(events.GovernorEventCorrelationIDHeader, correlationsID)
	}

	span.SetAttributes(attribute.String("event.correlation_id", headers.Get(events.GovernorEventCorrelationIDHeader)))

	msg := &nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  headers,
	}

	return c.conn.PublishMsg(msg)
}
