package v1alpha1

import (
	"github.com/nats-io/nats.go"
)

type mockNATSConn struct {
	Subject string
	Payload []byte
}

func (m *mockNATSConn) Drain() error { return nil }
func (m *mockNATSConn) Publish(s string, p []byte) error {
	m.Subject = s
	m.Payload = p

	return nil
}

func (m *mockNATSConn) PublishMsg(msg *nats.Msg) error {
	m.Subject = msg.Subject
	m.Payload = msg.Data

	return nil
}
