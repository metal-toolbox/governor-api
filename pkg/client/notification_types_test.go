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
	testNotificationTypesResponse = `[
		{
			"id": "21037d41-53ee-4144-b39b-6b2eb5761a30",
			"name": "Alert",
			"slug": "alert",
			"description": "alert",
			"default_enabled": true,
			"created_at": "2023-07-26T21:32:51.788685Z",
			"updated_at": "2023-07-26T21:32:51.788685Z",
			"deleted_at": null
		},
		{
			"id": "bcf7b896-31ad-4f61-81b2-534421ed3f4e",
			"name": "DEFCON-1",
			"slug": "defcon-1",
			"description": "defcon-1",
			"default_enabled": true,
			"created_at": "2023-07-26T21:33:05.637421Z",
			"updated_at": "2023-07-26T21:33:05.637421Z",
			"deleted_at": null
		},
		{
			"id": "e638b5a4-a471-43c2-9107-6c7a7deb669c",
			"name": "Notice",
			"slug": "notice",
			"description": "notice",
			"default_enabled": false,
			"created_at": "2023-07-26T21:32:40.560876Z",
			"updated_at": "2023-07-26T21:33:27.78482Z",
			"deleted_at": null
		}
	]`

	testNotificationTypeResponse = `{
		"id": "21037d41-53ee-4144-b39b-6b2eb5761a30",
		"name": "Alert",
		"slug": "alert",
		"description": "alert",
		"default_enabled": true,
		"created_at": "2023-07-26T21:32:51.788685Z",
		"updated_at": "2023-07-26T21:32:51.788685Z",
		"deleted_at": null
	}`
)

func TestClient_NotificationTypes(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.NotificationType {
		resp := []*v1alpha1.NotificationType{}
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
		want    []*v1alpha1.NotificationType
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypesResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testNotificationTypesResponse)),
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
			want:    []*v1alpha1.NotificationType(nil),
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
			got, err := c.NotificationTypes(context.TODO(), false)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_NotificationType(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationType {
		resp := &v1alpha1.NotificationType{}
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
		want    *v1alpha1.NotificationType
		wantErr bool
		id      string
	}{
		{
			name: "example request",
			id:   "21037d41-53ee-4144-b39b-6b2eb5761a30",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testNotificationTypeResponse)),
		},
		{
			name: "non-success",
			id:   "21037d41-53ee-4144-b39b-6b2eb5761a30",
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
			id:   "21037d41-53ee-4144-b39b-6b2eb5761a30",
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
			id:   "21037d41-53ee-4144-b39b-6b2eb5761a30",
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
			got, err := c.NotificationType(context.TODO(), tt.id, false)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_CreateNotificationType(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationType {
		resp := &v1alpha1.NotificationType{}
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
		req     *v1alpha1.NotificationTypeReq
		want    *v1alpha1.NotificationType
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.NotificationTypeReq{
				Name:        "Alert",
				Description: "alert",
			},
			want: testResp([]byte(testNotificationTypeResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusAccepted,
				},
			},
			req: &v1alpha1.NotificationTypeReq{
				Name:        "Alert",
				Description: "alert",
			},
			want: testResp([]byte(testNotificationTypeResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req: &v1alpha1.NotificationTypeReq{
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
			req: &v1alpha1.NotificationTypeReq{
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
			got, err := c.CreateNotificationType(context.TODO(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_UpdateNotificationType(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationType {
		resp := &v1alpha1.NotificationType{}
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
		req     *v1alpha1.NotificationTypeReq
		want    *v1alpha1.NotificationType
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusOK,
				},
			},
			id: "21037d41-53ee-4144-b39b-6b2eb5761a30",
			req: &v1alpha1.NotificationTypeReq{
				Name:        "Alert",
				Description: "alert",
			},
			want: testResp([]byte(testNotificationTypeResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusAccepted,
				},
			},
			id: "21037d41-53ee-4144-b39b-6b2eb5761a30",
			req: &v1alpha1.NotificationTypeReq{
				Name:        "Alert",
				Description: "alert",
			},
			want: testResp([]byte(testNotificationTypeResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id: "21037d41-53ee-4144-b39b-6b2eb5761a30",
			req: &v1alpha1.NotificationTypeReq{
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
			id: "21037d41-53ee-4144-b39b-6b2eb5761a30",
			req: &v1alpha1.NotificationTypeReq{
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
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.NotificationTypeReq{
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
			got, err := c.UpdateNotificationType(context.TODO(), tt.id, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_DeleteNotificationType(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.NotificationType {
		resp := &v1alpha1.NotificationType{}
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
		want    *v1alpha1.NotificationType
		wantErr bool
		id      string
	}{
		{
			name: "example request",
			id:   "21037d41-53ee-4144-b39b-6b2eb5761a30",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationTypeResponse),
					statusCode: http.StatusOK,
				},
			},
			want: testResp([]byte(testNotificationTypeResponse)),
		},
		{
			name: "non-success",
			id:   "21037d41-53ee-4144-b39b-6b2eb5761a30",
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
			err := c.DeleteNotificationType(context.TODO(), tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
