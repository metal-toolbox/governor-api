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

func TestClient_UserExtensionResources(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.UserExtensionResource {
		resp := []*v1alpha1.UserExtensionResource{}
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
		expected    []*v1alpha1.UserExtensionResource
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
			expected:  []*v1alpha1.UserExtensionResource(nil),
			expectErr: false,
		},
		{
			name: "extension not found",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"extension not found: sql: no rows in result set"}`),
				},
			},
			expected:    []*v1alpha1.UserExtensionResource(nil),
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
		{
			name: "user not found",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"user does not exist: sql: no rows in result set"}`),
				},
			},
			expected:    []*v1alpha1.UserExtensionResource(nil),
			expectErr:   true,
			expectedErr: v1alpha1.ErrUserNotFound,
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
			got, err := c.UserExtensionResources(
				context.TODO(), "user-1", "test-extension-1", "some-resources", "v1", false, nil,
			)

			if tt.expectedErr != nil {
				assert.ErrorAs(t, err, &tt.expectedErr)
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

func TestClient_UserExtensionResource(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.UserExtensionResource {
		resp := &v1alpha1.UserExtensionResource{}
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
		userID      string
		fields      fields
		expected    *v1alpha1.UserExtensionResource
		expectedErr error
		expectErr   bool
	}{
		{
			name:        "request with slug",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			name:        "missing user id",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			userID:      "",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingUserID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
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
		{
			name:        "resource not found",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"extension resource does not exist"}`),
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionResourceNotFound,
		},
		{
			name:        "user not found",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
					resp:       []byte(`{"error":"user does not exist"}`),
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrUserNotFound,
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
			got, err := c.UserExtensionResource(
				context.TODO(), tt.userID, tt.extensionID, tt.erdID, tt.erdVersion,
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

func TestClient_CreateUserExtensionResource(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.UserExtensionResource {
		resp := &v1alpha1.UserExtensionResource{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name        string
		extensionID string
		erdID       string
		userID      string
		erdVersion  string
		fields      fields
		req         interface{}
		expected    *v1alpha1.UserExtensionResource
		expectedErr error
		expectErr   bool
	}{
		{
			name:        "example request",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusCreated,
				},
			},
			req:       testExtensionResourcePayload,
			expected:  testResp([]byte(testExtensionResourceResponse)),
			expectErr: false,
		},
		{
			name:        "example request status accepted",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusAccepted,
				},
			},
			req:       testExtensionResourcePayload,
			expected:  testResp([]byte(testExtensionResourceResponse)),
			expectErr: false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
		},
		{
			name:        "bad json response",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusCreated,
					resp:       []byte(`{`),
				},
			},
			req:       testExtensionResourcePayload,
			expectErr: true,
		},
		{
			name:        "extension ID missing",
			extensionID: "",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusCreated,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingExtensionIDOrSlug,
		},
		{
			name:        "ERD ID missing",
			extensionID: "test-extension-1",
			erdID:       "",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusCreated,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingERDIDOrSlug,
		},
		{
			name:        "user ID missing",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusCreated,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingUserID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"extension does not exist: sql: no rows in result set"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
		{
			name:        "user not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "f7c1c9f5-6c7d-4d6e-8d7e-9c7a3a9b5f0d",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"user does not exist: sql: no rows in result set"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: v1alpha1.ErrUserNotFound,
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
			got, err := c.CreateUserExtensionResource(
				context.TODO(), tt.userID, tt.extensionID, tt.erdID, tt.erdVersion, tt.req,
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

func TestClient_UpdateUserExtensionResource(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.UserExtensionResource {
		resp := &v1alpha1.UserExtensionResource{}
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
		userID      string
		fields      fields
		resourceID  string
		req         interface{}
		expected    *v1alpha1.UserExtensionResource
		expectedErr error
		expectErr   bool
	}{
		{
			name:        "example request",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusOK,
				},
			},
			req:       testExtensionResourcePayload,
			expected:  testResp([]byte(testExtensionResourceResponse)),
			expectErr: false,
		},
		{
			name:        "example request status accepted",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testExtensionResourceResponse),
					statusCode: http.StatusAccepted,
				},
			},
			req:       testExtensionResourcePayload,
			expected:  testResp([]byte(testExtensionResourceResponse)),
			expectErr: false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
		},
		{
			name:        "bad json response",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			req:       testExtensionResourcePayload,
			expectErr: true,
		},
		{
			name:       "missing extension id",
			erdID:      "erd-1",
			erdVersion: "v1alpha1",
			userID:     "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID: "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(""),
					statusCode: http.StatusOK,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingExtensionIDOrSlug,
		},
		{
			name:        "missing erd id",
			extensionID: "test-extension-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(""),
					statusCode: http.StatusOK,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingERDIDOrSlug,
		},
		{
			name:        "missing resource id",
			erdID:       "erd-1",
			extensionID: "test-extension-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(""),
					statusCode: http.StatusOK,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingResourceID,
		},
		{
			name: "missing user id",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(""),
					statusCode: http.StatusOK,
				},
			},
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: ErrMissingUserID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"extension does not exist"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
		{
			name:        "ERD not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"ERD does not exist"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req:         testExtensionResourcePayload,
			expectErr:   true,
			expectedErr: v1alpha1.ErrERDNotFound,
		},
		{
			name:        "user not found",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			erdVersion:  "v1alpha1",
			userID:      "e8f5f7c8-6d4a-4c7d-9a5b-9c9c8d7e6f5a",
			resourceID:  "673ccd3a-1381-4e68-bc90-04e5f6745b9c",
			req:         testExtensionResourcePayload,
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"user does not exist"}`),
					statusCode: http.StatusNotFound,
				},
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrUserNotFound,
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
			got, err := c.UpdateUserExtensionResource(
				context.TODO(), tt.userID, tt.extensionID, tt.erdID, tt.erdVersion,
				tt.resourceID, tt.req,
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

func TestClient_DeleteUserExtensionResource(t *testing.T) {
	type fields struct {
		httpClient *mockHTTPDoer
	}

	tests := []struct {
		name        string
		userID      string
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
			userID:      "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			userID:      "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			name:        "missing user id",
			extensionID: "test-extension-1",
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
			expectedErr: ErrMissingUserID,
		},
		{
			name:       "missing extension id",
			userID:     "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			userID:      "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			userID:      "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			userID:      "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			userID:      "d4f8e98c-7a1e-4b5c-9a9c-4e4d9e6f3c3f",
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
			err := c.DeleteUserExtensionResource(context.TODO(), tt.userID, tt.extensionID, tt.erdID, tt.erdVersion, tt.resourceID)

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
