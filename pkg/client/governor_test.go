package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockTransport implements [http.RoundTripper]
var _ http.RoundTripper = &mockTransport{}

type mockTransport struct {
	t          *testing.T
	statusCode int
	resp       []byte
	request    *http.Request
}

func (m *mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := http.Response{
		StatusCode: m.statusCode,
	}

	m.request = r
	resp.Body = io.NopCloser(bytes.NewReader(m.resp))

	return &resp, nil
}

func (m *mockTransport) Request() *http.Request {
	return m.request
}

// mockMultiTransport implements [http.RoundTripper], returning a different
// response on each successive call.
var _ http.RoundTripper = &mockMultiTransport{}

type mockMultiTransport struct {
	t           *testing.T
	statusCode  int
	resp        [][]byte
	timesCalled int
}

func (m *mockMultiTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	resp := http.Response{
		StatusCode: m.statusCode,
	}

	resp.Body = io.NopCloser(bytes.NewReader(m.resp[m.timesCalled]))

	m.timesCalled++

	return &resp, nil
}

func TestClient_newGovernorRequest(t *testing.T) {
	testReq := func(m, u string) *http.Request {
		queryURL, _ := url.Parse(u)

		req, _ := http.NewRequestWithContext(context.TODO(), m, queryURL.String(), nil)

		return req
	}

	tests := []struct {
		name    string
		url     string
		method  string
		want    *http.Request
		wantErr bool
	}{
		{
			name:   "example GET request",
			method: http.MethodGet,
			url:    "https://foo.example.com/tax",
			want:   testReq(http.MethodGet, "https://foo.example.com/tax"),
		},
		{
			name:    "example bad method",
			method:  "BREAK IT",
			url:     "https://foo.example.com/zoning",
			wantErr: true,
		},
		{
			name:    "example bad url ",
			url:     "#&^$%^*T@#%",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				logger: zap.NewNop(),
			}

			got, err := c.newGovernorRequest(context.TODO(), tt.method, tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
