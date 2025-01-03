package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type mockHTTPDoer struct {
	t          *testing.T
	statusCode int
	resp       []byte
	request    *http.Request
}

type mockTokener struct {
	t     *testing.T
	err   error
	token *oauth2.Token
}

func (m *mockHTTPDoer) Do(r *http.Request) (*http.Response, error) {
	resp := http.Response{
		StatusCode: m.statusCode,
	}

	m.request = r
	resp.Body = io.NopCloser(bytes.NewReader(m.resp))

	return &resp, nil
}

func (m *mockHTTPDoer) Request() *http.Request {
	return m.request
}

func (m *mockTokener) Token(_ context.Context) (*oauth2.Token, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.token != nil {
		return m.token, nil
	}

	return &oauth2.Token{Expiry: time.Now().Add(5 * time.Second)}, nil
}

type mockHTTPMultiDoer struct {
	t           *testing.T
	statusCode  int
	resp        [][]byte
	timesCalled int
}

func (m *mockHTTPMultiDoer) Do(_ *http.Request) (*http.Response, error) {
	resp := http.Response{
		StatusCode: m.statusCode,
	}

	resp.Body = io.NopCloser(bytes.NewReader(m.resp[m.timesCalled]))

	m.timesCalled++

	return &resp, nil
}

func TestClient_newGovernorRequest(t *testing.T) {
	testReq := func(m, u, t string) *http.Request {
		queryURL, _ := url.Parse(u)

		req, _ := http.NewRequestWithContext(context.TODO(), m, queryURL.String(), nil)
		req.Header.Add("Authorization", "Bearer "+t)

		return req
	}

	type fields struct {
		url   string
		token *oauth2.Token
	}

	tests := []struct {
		name    string
		fields  fields
		method  string
		url     string
		want    *http.Request
		wantErr bool
	}{
		{
			name: "example GET request",
			fields: fields{
				token: &oauth2.Token{
					AccessToken: "topSekret!!!!!11111",
					Expiry:      time.Now().Add(5 * time.Second),
				},
			},
			method: http.MethodGet,
			url:    "https://foo.example.com/tax",
			want:   testReq(http.MethodGet, "https://foo.example.com/tax", "topSekret!!!!!11111"),
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
				url:                    tt.fields.url,
				logger:                 zap.NewNop(),
				clientCredentialConfig: &mockTokener{t: t},
				token:                  tt.fields.token,
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
