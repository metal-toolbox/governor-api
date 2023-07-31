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
	testNotificationPreferencesResponse = `[
		{
			"notification_type": "defcon-1",
			"enabled": true,
			"notification_targets": [
				{
					"target": "slack",
					"enabled": true
				},
				{
					"target": "email",
					"enabled": true
				}
			]
		},
		{
			"notification_type": "notice",
			"enabled": false,
			"notification_targets": [
				{
					"target": "slack",
					"enabled": true
				},
				{
					"target": "email",
					"enabled": true
				}
			]
		},
		{
			"notification_type": "alert",
			"enabled": true,
			"notification_targets": [
				{
					"target": "slack",
					"enabled": true
				},
				{
					"target": "email",
					"enabled": true
				}
			]
		}
	]`
)

func TestClient_NotificationPreferences(t *testing.T) {
	testResp := func(r []byte) v1alpha1.UserNotificationPreferences {
		resp := v1alpha1.UserNotificationPreferences{}
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
		userID  string
		fields  fields
		want    v1alpha1.UserNotificationPreferences
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationPreferencesResponse),
					statusCode: http.StatusOK,
				},
			},
			userID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			want:   testResp([]byte(testNotificationPreferencesResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			userID:  "186c5a52-4421-4573-8bbf-78d85d3c277e",
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
			userID:  "186c5a52-4421-4573-8bbf-78d85d3c277e",
			wantErr: true,
		},
		{
			name: "missing user ID",
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
			got, err := c.NotificationPreferences(context.TODO(), "")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_UpdateNotificationPreferences(t *testing.T) {
	testResp := func(r []byte) v1alpha1.UserNotificationPreferences {
		resp := v1alpha1.UserNotificationPreferences{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	enabled := false

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		userID  string
		req     v1alpha1.UserNotificationPreferences
		want    v1alpha1.UserNotificationPreferences
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationPreferencesResponse),
					statusCode: http.StatusOK,
				},
			},
			userID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: v1alpha1.UserNotificationPreferences{
				{
					NotificationType: "alert",
					Enabled:          &enabled,
				},
			},
			want: testResp([]byte(testNotificationPreferencesResponse)),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationPreferencesResponse),
					statusCode: http.StatusAccepted,
				},
			},
			userID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: v1alpha1.UserNotificationPreferences{
				{
					NotificationType: "alert",
					Enabled:          &enabled,
				},
			},
			want: testResp([]byte(testNotificationPreferencesResponse)),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			userID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: v1alpha1.UserNotificationPreferences{
				{
					NotificationType: "alert",
					Enabled:          &enabled,
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
			userID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: v1alpha1.UserNotificationPreferences{
				{
					NotificationType: "alert",
					Enabled:          &enabled,
				},
			},
			wantErr: true,
		},
		{
			name: "missing user id",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testNotificationPreferencesResponse),
					statusCode: http.StatusOK,
				},
			},
			req: v1alpha1.UserNotificationPreferences{
				{
					NotificationType: "alert",
					Enabled:          &enabled,
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
			got, err := c.UpdateNotificationPreferences(context.TODO(), tt.userID, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
