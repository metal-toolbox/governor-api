package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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
		transport http.RoundTripper
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
				transport: &mockTransport{
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
				transport: &mockTransport{
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
				transport: &mockTransport{
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
				transport: &mockTransport{
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
				url:        "https://the.gov/",
				logger:     zap.NewNop(),
				httpClient: &http.Client{Transport: tt.fields.transport},
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
