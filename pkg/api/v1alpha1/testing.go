package v1alpha1

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
