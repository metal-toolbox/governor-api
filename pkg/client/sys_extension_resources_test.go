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
	testExtensionResourcesResponse = `[
		{
			"id": "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			"resource": {
				"age": 10,
				"firstName": "test",
				"lastName": "2"
			},
			"created_at": "2023-10-03T15:00:31.723117Z",
			"updated_at": "2023-10-03T15:00:31.723117Z",
			"deleted_at": null,
			"extension_resource_definition_id": "151f7af1-8211-471e-bf71-60e15d767243"
		},
		{
			"id": "7ebb627b-707e-4d6c-b155-6245f4e74cd2",
			"resource": {
				"age": 10,
				"firstName": "test",
				"lastName": "3"
			},
			"created_at": "2023-10-03T15:00:36.123365Z",
			"updated_at": "2023-10-03T15:00:36.123365Z",
			"deleted_at": null,
			"extension_resource_definition_id": "151f7af1-8211-471e-bf71-60e15d767243"
		},
		{
			"id": "87ae035e-54c4-4bb6-af74-e4eb71b26a70",
			"resource": {
				"age": 10,
				"firstName": "test",
				"lastName": "1"
			},
			"created_at": "2023-10-03T14:59:44.491402Z",
			"updated_at": "2023-10-03T15:00:12.323262Z",
			"deleted_at": null,
			"extension_resource_definition_id": "151f7af1-8211-471e-bf71-60e15d767243"
		}
	]`

	testExtensionResourceResponse = `{
		"id": "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
		"resource": {
			"age": 10,
			"firstName": "test",
			"lastName": "2"
		},
		"created_at": "2023-10-03T15:00:31.723117Z",
		"updated_at": "2023-10-03T15:00:31.723117Z",
		"deleted_at": null,
		"extension_resource_definition_id": "151f7af1-8211-471e-bf71-60e15d767243"
	}`
)

var testExtensionResourcePayload = &struct {
	Age       int    `json:"age"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}{
	Age:       10,
	FirstName: "test",
	LastName:  "2",
}

func TestClient_SystemExtensionResources(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.SystemExtensionResource {
		resp := []*v1alpha1.SystemExtensionResource{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name        string
		fields      fields
		expected    []*v1alpha1.SystemExtensionResource
		expectErr   bool
		expectedErr error
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourcesResponse),
					statusCode: http.StatusOK,
				},
			},
			expected:  testResp([]byte(testExtensionResourcesResponse)),
			expectErr: false,
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
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
			expectErr: true,
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
			expected:  []*v1alpha1.SystemExtensionResource(nil),
			expectErr: false,
		},
		{
			name: "extension not found",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"extension does not exist: sql: no rows in result set"}`),
				},
			},
			expected:    []*v1alpha1.SystemExtensionResource(nil),
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.SystemExtensionResources(
				context.TODO(), "test-extension-1", "some-resources", "v1", false, nil,
			)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			} else if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestClient_SystemExtensionResource(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.SystemExtensionResource {
		resp := &v1alpha1.SystemExtensionResource{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient *mockHTTPDoer
	}

	tests := []struct {
		name        string
		extensionID string
		erdID       string
		erdVersion  string
		resourceID  string
		fields      fields
		expected    *v1alpha1.SystemExtensionResource
		expectedErr error
		expectErr   bool
	}{
		{
			name:        "request with slug",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusOK,
				},
			},
			expected:  testResp([]byte(testExtensionResourceResponse)),
			expectErr: false,
		},
		{
			name:        "request with uuid",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusOK,
				},
			},
			expected:  testResp([]byte(testExtensionResourceResponse)),
			expectErr: false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
		},
		{
			name:        "bad json response",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			expectErr: true,
		},
		{
			name:        "missing extension slug",
			extensionID: "",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingExtensionIDOrSlug,
		},
		{
			name:        "missing ERD slug",
			extensionID: "test-extension-1",
			erdID:       "",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingERDIDOrSlug,
		},
		{
			name:        "missing resource id",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingResourceID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"extension does not exist"}`),
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
		{
			name:        "ERD not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"ERD does not exist"}`),
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrERDNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.SystemExtensionResource(
				context.TODO(), tt.extensionID, tt.erdID, tt.erdVersion,
				tt.resourceID, false,
			)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			} else if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestClient_DeleteSystemExtensionResource(t *testing.T) {
	type fields struct {
		httpClient *mockHTTPDoer
	}

	tests := []struct {
		name        string
		extensionID string
		erdID       string
		erdVersion  string
		resourceID  string
		fields      fields
		expectedErr error
		expectErr   bool
	}{
		{
			name:        "example request",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusOK,
				},
			},
			expectErr: false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
		},
		{
			name:       "missing extension id",
			erdID:      "erd-1",
			erdVersion: "v1alpha1",
			resourceID: "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingExtensionIDOrSlug,
		},
		{
			name:        "missing ERD id",
			extensionID: "test-extension-1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingERDIDOrSlug,
		},
		{
			name:        "missing resource id",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingResourceID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"extension does not exist"}`),
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
		{
			name:        "ERD not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"ERD does not exist"}`),
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrERDNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			err := c.DeleteSystemExtensionResource(context.TODO(), tt.extensionID, tt.erdID, tt.erdVersion, tt.resourceID)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			} else if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
