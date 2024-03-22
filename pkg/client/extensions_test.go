package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	testExtensionsResponse = `[
		{
			"id": "35b9861f-83b5-49df-95b0-321cfe5c1532",
			"name": "Test Extension 1",
			"slug": "test-extension-1",
			"description": "some test",
			"enabled": true,
			"status": "online",
			"created_at": "2023-09-26T20:04:19.190374Z",
			"updated_at": "2023-09-26T20:04:19.190374Z",
			"deleted_at": null
		},
		{
			"id": "e311a55e-d77f-4289-ba69-e2cbea09e3a3",
			"name": "Test Extension 2",
			"slug": "test-extension-2",
			"description": "some test",
			"enabled": true,
			"status": "online",
			"created_at": "2023-09-26T20:04:19.190374Z",
			"updated_at": "2023-09-26T20:04:19.190374Z",
			"deleted_at": null
		}
	]`

	testExtensionResponse = `{
		"id": "35b9861f-83b5-49df-95b0-321cfe5c1532",
		"name": "Test Extension 1",
		"slug": "test-extension-1",
		"description": "some test",
		"enabled": true,
		"status": "online",
		"created_at": "2023-09-26T20:04:19.190374Z",
		"updated_at": "2023-09-26T20:04:19.190374Z",
		"deleted_at": null
	}`
)

func TestClient_Extensions(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.Extension {
		resp := []*v1alpha1.Extension{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		want    []*v1alpha1.Extension
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionsResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testExtensionsResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			wantErr: true,
		},
		{
			name: "bad json response",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			wantErr: true,
		},
		{
			name: "null response",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`null`),
				},
			},
			want:    []*v1alpha1.Extension(nil),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.Extensions(context.TODO(), false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_Extension(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Extension {
		resp := &v1alpha1.Extension{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		want    *v1alpha1.Extension
		wantErr bool
		id      string
	}{
		{
			name: "example request",
			id:   "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testExtensionResponse)),
		},
		{
			name: "non-success",
			id:   "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			wantErr: true,
		},
		{
			name: "bad json response",
			id:   "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			wantErr: true,
		},
		{
			name: "missing id",
			id:   "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.Extension(context.TODO(), tt.id, false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_CreateExtension(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Extension {
		resp := &v1alpha1.Extension{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	enabled := true

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		req     *v1alpha1.ExtensionReq
		want    *v1alpha1.Extension
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionReq{
				Name:        "Test Extension 1",
				Description: "some test",
				Enabled:     &enabled,
			},
			want: testResp([]byte(testExtensionResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusAccepted,
				},
			},
			req: &v1alpha1.ExtensionReq{
				Name:        "Test Extension 1",
				Description: "some test",
				Enabled:     &enabled,
			},
			want: testResp([]byte(testExtensionResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req: &v1alpha1.ExtensionReq{
				Name:        "Test Extension 1",
				Description: "some test",
				Enabled:     &enabled,
			},
			wantErr: true,
		},
		{
			name: "bad json response",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			req: &v1alpha1.ExtensionReq{
				Name:        "Test Extension 1",
				Description: "some test",
				Enabled:     &enabled,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.CreateExtension(context.TODO(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_UpdateExtension(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Extension {
		resp := &v1alpha1.Extension{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		id      string
		req     *v1alpha1.ExtensionReq
		want    *v1alpha1.Extension
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusOK,
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionReq{
				Description: "some test",
			},
			want: testResp([]byte(testExtensionResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusAccepted,
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionReq{
				Description: "some test",
			},
			want: testResp([]byte(testExtensionResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionReq{
				Description: "some test",
			},
			wantErr: true,
		},
		{
			name: "bad json response",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionReq{
				Description: "some test",
			},
			wantErr: true,
		},
		{
			name: "missing id",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionReq{
				Description: "some test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.UpdateExtension(context.TODO(), tt.id, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_DeleteExtension(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Extension {
		resp := &v1alpha1.Extension{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		want    *v1alpha1.Extension
		wantErr bool
		id      string
	}{
		{
			name: "example request",
			id:   "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testExtensionResponse)),
		},
		{
			name: "non-success",
			id:   "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			wantErr: true,
		},
		{
			name: "missing id",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			err := c.DeleteExtension(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
