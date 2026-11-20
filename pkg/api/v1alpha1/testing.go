package v1alpha1

import (
	"github.com/nats-io/nats.go"
)

type mockNATSConn struct {
	Subject string
	Payload []byte

	// err, when set, is returned by every publish, and nothing is recorded.
	err error
}

func (m *mockNATSConn) Drain() error { return nil }
func (m *mockNATSConn) Publish(s string, p []byte) error {
	if m.err != nil {
		return m.err
	}

	m.Subject = s
	m.Payload = p

	return nil
}

func (m *mockNATSConn) PublishMsg(msg *nats.Msg) error {
	if m.err != nil {
		return m.err
	}

	m.Subject = msg.Subject
	m.Payload = msg.Data

	return nil
}
