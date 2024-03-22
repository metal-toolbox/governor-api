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
	testNotificationTargetsResponse = `[
		{
			"id": "16e44109-f49b-4ce7-911b-0a54bcc9a351",
			"name": "Email",
			"slug": "email",
			"description": "less nice",
			"default_enabled": true,
			"created_at": "2023-07-27T21:36:11.029099Z",
			"updated_at": "2023-07-27T21:36:11.029099Z",
			"deleted_at": null
		},
		{
			"id": "03e02f7f-3d36-4423-9165-cbe3deabbf04",
			"name": "MS Teams",
			"slug": "ms-teams",
			"description": "less less nice",
			"default_enabled": true,
			"created_at": "2023-07-27T21:36:25.455371Z",
			"updated_at": "2023-07-27T21:36:25.455371Z",
			"deleted_at": null
		},
		{
			"id": "71e05156-ee15-4f99-ba34-94f38ecc5438",
			"name": "Slack",
			"slug": "slack",
			"description": "nice",
			"default_enabled": true,
			"created_at": "2023-07-27T21:36:07.108261Z",
			"updated_at": "2023-07-27T21:36:07.108261Z",
			"deleted_at": null
		}
	]`

	testNotificationTargetResponse = `{
		"id": "71e05156-ee15-4f99-ba34-94f38ecc5438",
		"name": "Slack",
		"slug": "slack",
		"description": "nice",
		"default_enabled": true,
		"created_at": "2023-07-27T21:36:07.108261Z",
		"updated_at": "2023-07-27T21:36:07.108261Z",
		"deleted_at": null
	}`
)

func TestClient_NotificationTargets(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.NotificationTarget {
		resp := []*v1alpha1.NotificationTarget{}
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
		want    []*v1alpha1.NotificationTarget
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetsResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testNotificationTargetsResponse)),
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
			want:    []*v1alpha1.NotificationTarget(nil),
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
			got, err := c.NotificationTargets(context.TODO(), false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_NotificationTarget(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationTarget {
		resp := &v1alpha1.NotificationTarget{}
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
		want    *v1alpha1.NotificationTarget
		wantErr bool
		id      string
	}{
		{
			name: "example request",
			id:   "71e05156-ee15-4f99-ba34-94f38ecc5438",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testNotificationTargetResponse)),
		},
		{
			name: "non-success",
			id:   "71e05156-ee15-4f99-ba34-94f38ecc5438",
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
			id:   "71e05156-ee15-4f99-ba34-94f38ecc5438",
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
			id:   "71e05156-ee15-4f99-ba34-94f38ecc5438",
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
			got, err := c.NotificationTarget(context.TODO(), tt.id, false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_CreateNotificationTarget(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationTarget {
		resp := &v1alpha1.NotificationTarget{}
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
		req     *v1alpha1.NotificationTargetReq
		want    *v1alpha1.NotificationTarget
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.NotificationTargetReq{
				Name:           "Slack",
				Description:    "nice",
				DefaultEnabled: &enabled,
			},
			want: testResp([]byte(testNotificationTargetResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusAccepted,
				},
			},
			req: &v1alpha1.NotificationTargetReq{
				Name:           "Slack",
				Description:    "nice",
				DefaultEnabled: &enabled,
			},
			want: testResp([]byte(testNotificationTargetResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req: &v1alpha1.NotificationTargetReq{
				Name:           "Slack",
				Description:    "nice",
				DefaultEnabled: &enabled,
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
			req: &v1alpha1.NotificationTargetReq{
				Name:           "Slack",
				Description:    "nice",
				DefaultEnabled: &enabled,
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
			got, err := c.CreateNotificationTarget(context.TODO(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_UpdateNotificationTarget(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationTarget {
		resp := &v1alpha1.NotificationTarget{}
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
		req     *v1alpha1.NotificationTargetReq
		want    *v1alpha1.NotificationTarget
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusOK,
				},
			},
			id: "71e05156-ee15-4f99-ba34-94f38ecc5438",
			req: &v1alpha1.NotificationTargetReq{
				Name:        "Alert",
				Description: "alert",
			},
			want: testResp([]byte(testNotificationTargetResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusAccepted,
				},
			},
			id: "71e05156-ee15-4f99-ba34-94f38ecc5438",
			req: &v1alpha1.NotificationTargetReq{
				Name:        "Alert",
				Description: "alert",
			},
			want: testResp([]byte(testNotificationTargetResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id: "71e05156-ee15-4f99-ba34-94f38ecc5438",
			req: &v1alpha1.NotificationTargetReq{
				Name:        "Alert",
				Description: "alert",
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
			id: "71e05156-ee15-4f99-ba34-94f38ecc5438",
			req: &v1alpha1.NotificationTargetReq{
				Name:        "Alert",
				Description: "alert",
			},
			wantErr: true,
		},
		{
			name: "missing id",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.NotificationTargetReq{
				Name:        "Alert",
				Description: "alert",
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
			got, err := c.UpdateNotificationTarget(context.TODO(), tt.id, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_DeleteNotificationTarget(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationTarget {
		resp := &v1alpha1.NotificationTarget{}
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
		want    *v1alpha1.NotificationTarget
		wantErr bool
		id      string
	}{
		{
			name: "example request",
			id:   "71e05156-ee15-4f99-ba34-94f38ecc5438",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTargetResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testNotificationTargetResponse)),
		},
		{
			name: "non-success",
			id:   "71e05156-ee15-4f99-ba34-94f38ecc5438",
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
			err := c.DeleteNotificationTarget(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
