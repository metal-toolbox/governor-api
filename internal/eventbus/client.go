package eventbus

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

const (
	defaultSubject = "events"
)

type conn interface {
	Publish(subject string, data []byte) error
	Drain() error
}

// Client is an event bus client with some configuration
type Client struct {
	conn   conn
	logger *zap.Logger
	prefix string
}

// Option is a functional configuration option for governor eventing
type Option func(c *Client)

// NewClient configures and establishes a new event bus client connection
func NewClient(opts ...Option) *Client {
	client := Client{
		logger: zap.NewNop(),
		prefix: defaultSubject,
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

// WithNATSConn sets the nats connection
func WithNATSConn(nc *nats.Conn) Option {
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

	c.logger.Debug("publishing event on subject", zap.String("subject", subject), zap.Any("event", event))

	j, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return c.conn.Publish(subject, j)
}
