package eventbus

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

type mockConn struct {
	t    *testing.T
	err  error
	data []byte
}

// Publish is a mock publish function
func (m *mockConn) Publish(_ string, data []byte) error {
	if m.err != nil {
		return m.err
	}

	m.t.Logf("got data payload %s", string(data))

	if !bytes.Equal(m.data, data) {
		return errors.New("unexpected data payload") //nolint:goerr113
	}

	return nil
}

// Drain is a mock drain function
func (m *mockConn) Drain() error {
	if m.err != nil {
		return m.err
	}

	return nil
}

// PublishMsg is a mock publish message function
func (m *mockConn) PublishMsg(_ *nats.Msg) error {
	return nil
}

func Test_NewClient(t *testing.T) {
	client := NewClient()

	clientType := reflect.TypeOf(client).String()
	if clientType != "*eventbus.Client" {
		t.Errorf("expected type to be '*eventbus.Client', got %s", clientType)
	}

	if client.prefix != "events" {
		t.Errorf("expected default client prefix to be 'events', got %s", client.prefix)
	}

	client = NewClient(WithNATSPrefix("test-prefix"))
	if client.prefix != "test-prefix" {
		t.Errorf("expected client prefix to be 'test-prefix', got %s", client.prefix)
	}

	client = NewClient(WithLogger(zap.NewExample()))
	if client.logger.Core().Enabled(zap.DebugLevel) != true {
		t.Error("expected logger debug level to be 'true', got 'false'")
	}
}

func TestClient_Publish(t *testing.T) {
	tests := []struct {
		name    string
		sub     string
		event   *events.Event
		data    []byte
		err     error
		wantErr bool
	}{
		{
			name:    "empty event",
			sub:     "test",
			wantErr: true,
		},
		{
			name: "example event",
			sub:  "test",
			event: &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: "0123-abcd",
				GroupID: "phoenix",
				UserID:  "meta",
			},
			data: []byte(`{"version":"v1alpha1","action":"CREATE","audit_id":"0123-abcd","group_id":"phoenix","user_id":"meta","traceContext":{}}`),
		},
		{
			name: "example event with actor",
			sub:  "test",
			event: &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: "0123-abcd",
				GroupID: "phoenix",
				UserID:  "meta",
				ActorID: "actor",
			},
			data: []byte(`{"version":"v1alpha1","action":"CREATE","audit_id":"0123-abcd","group_id":"phoenix","user_id":"meta","actor_id":"actor","traceContext":{}}`),
		},
		{
			name: "example event empty user_id",
			sub:  "test",
			event: &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: "0123-abcd",
				GroupID: "phoenix",
				UserID:  "",
			},
			data: []byte(`{"version":"v1alpha1","action":"CREATE","audit_id":"0123-abcd","group_id":"phoenix","traceContext":{}}`),
		},
		{
			name: "example event empty group_id",
			sub:  "test",
			event: &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: "0123-abcd",
				GroupID: "",
				UserID:  "meta",
			},
			data: []byte(`{"version":"v1alpha1","action":"CREATE","audit_id":"0123-abcd","user_id":"meta","traceContext":{}}`),
		},
		{
			name: "publish error",
			sub:  "test",
			event: &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: "0123-abcd",
				GroupID: "phoenix",
				UserID:  "meta",
			},
			err:     errors.New("boom"), //nolint:goerr113
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				logger: zap.NewNop(),
				conn:   &mockConn{t, tt.err, tt.data},
				prefix: "test",
				tracer: otel.GetTracerProvider().Tracer("test"),
			}
			err := c.Publish(context.TODO(), tt.sub, tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
