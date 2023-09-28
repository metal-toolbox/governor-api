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
	testERDsResponse = `[
		{
			"id": "a82a34a5-db1f-464f-af9c-76086e79f715",
			"name": "ERD 1",
			"description": "some description",
			"enabled": true,
			"slug_singular": "erd-1",
			"slug_plural": "erd-1s",
			"version": "v1alpha1",
			"scope": "system",
			"schema": {
				"hello": "world"
			},
			"created_at": "2023-09-27T16:52:45.244555Z",
			"updated_at": "2023-09-27T16:52:45.244555Z",
			"deleted_at": null,
			"extension_id": "9ee302b2-e559-4553-a5ab-0a06484ad883"
		},
		{
			"id": "a82a34a5-db1f-464f-af9c-76086e79f715",
			"name": "ERD 2",
			"description": "some description",
			"enabled": true,
			"slug_singular": "erd-2",
			"slug_plural": "erd-2s",
			"version": "v1alpha1",
			"scope": "user",
			"schema": {
				"hello": "world"
			},
			"created_at": "2023-09-27T16:52:45.244555Z",
			"updated_at": "2023-09-27T16:52:45.244555Z",
			"deleted_at": null,
			"extension_id": "9ee302b2-e559-4553-a5ab-0a06484ad883"
		}
	]`

	testERDResponse = `{
		"id": "a82a34a5-db1f-464f-af9c-76086e79f715",
		"name": "ERD 1",
		"description": "some description",
		"enabled": true,
		"slug_singular": "erd-1",
		"slug_plural": "erd-1s",
		"version": "v1alpha1",
		"scope": "system",
		"schema": {
			"hello": "world"
		},
		"created_at": "2023-09-27T16:52:45.244555Z",
		"updated_at": "2023-09-27T16:52:45.244555Z",
		"deleted_at": null,
		"extension_id": "9ee302b2-e559-4553-a5ab-0a06484ad883"
	}`
)

func TestClient_ExtensionResourceDefinitions(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.ExtensionResourceDefinition {
		resp := []*v1alpha1.ExtensionResourceDefinition{}
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
		expected    []*v1alpha1.ExtensionResourceDefinition
		expectErr   bool
		expectedErr error
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDsResponse),
					statusCode: http.StatusOK,
				},
			},
			expected:  testResp([]byte(testERDsResponse)),
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
			expected:  []*v1alpha1.ExtensionResourceDefinition(nil),
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
			expected:    []*v1alpha1.ExtensionResourceDefinition(nil),
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
			got, err := c.ExtensionResourceDefinitions(context.TODO(), "test-extension-1", false)

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

func TestClient_ExtensionResourceDefinition(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.ExtensionResourceDefinition {
		resp := &v1alpha1.ExtensionResourceDefinition{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient *mockHTTPDoer
	}

	tests := []struct {
		name         string
		extensionID  string
		erdID        string
		erdVersion   string
		fields       fields
		expected     *v1alpha1.ExtensionResourceDefinition
		expectedErr  error
		expectedPath string
		expectErr    bool
	}{
		{
			name:        "request with slug",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			expected:     testResp([]byte(testERDResponse)),
			expectedPath: "/api/v1alpha1/extensions/test-extension-1/erds/erd-1/v1alpha1",
			expectErr:    false,
		},
		{
			name:        "request with uuid",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			expected:     testResp([]byte(testERDResponse)),
			expectedPath: "/api/v1alpha1/extensions/test-extension-1/erds/a82a34a5-db1f-464f-af9c-76086e79f715",
			expectErr:    false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
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
			name:        "missing extension id",
			extensionID: "",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingExtensionID,
		},
		{
			name:        "missing ERD id",
			extensionID: "test-extension-1",
			erdID:       "",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingERDID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
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
			got, err := c.ExtensionResourceDefinition(context.TODO(), tt.extensionID, tt.erdID, tt.erdVersion, false)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			} else if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)

			if tt.expectedPath != "" {
				assert.Equal(t, tt.expectedPath, tt.fields.httpClient.Request().URL.Path)
			}
		})
	}
}

func TestClient_CreateExtensionResourceDefinition(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.ExtensionResourceDefinition {
		resp := &v1alpha1.ExtensionResourceDefinition{}
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
		name        string
		extensionID string
		fields      fields
		req         *v1alpha1.ExtensionResourceDefinitionReq
		expected    *v1alpha1.ExtensionResourceDefinition
		expectedErr error
		expectErr   bool
	}{
		{
			name:        "example request",
			extensionID: "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Name:         "ERD 1",
				Description:  "some description",
				Enabled:      &enabled,
				SlugSingular: "erd-1",
				SlugPlural:   "erd-1s",
				Version:      "v1alpha1",
				Scope:        "system",
				Schema:       []byte(`{"hello": "world"}`),
			},
			expected:  testResp([]byte(testERDResponse)),
			expectErr: false,
		},
		{
			name:        "example request status accepted",
			extensionID: "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusAccepted,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Name:         "ERD 1",
				Description:  "some description",
				Enabled:      &enabled,
				SlugSingular: "erd-1",
				SlugPlural:   "erd-1s",
				Version:      "v1alpha1",
				Scope:        "system",
				Schema:       []byte(`{"hello": "world"}`),
			},
			expected:  testResp([]byte(testERDResponse)),
			expectErr: false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req:         &v1alpha1.ExtensionResourceDefinitionReq{},
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
		},
		{
			name:        "bad json response",
			extensionID: "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			req:       &v1alpha1.ExtensionResourceDefinitionReq{},
			expectErr: true,
		},
		{
			name:        "extension ID missing",
			extensionID: "",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			req:         &v1alpha1.ExtensionResourceDefinitionReq{},
			expectErr:   true,
			expectedErr: ErrMissingExtensionID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"extension not found: sql: no rows in result set"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req:         &v1alpha1.ExtensionResourceDefinitionReq{},
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
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
			got, err := c.CreateExtensionResourceDefinition(context.TODO(), tt.extensionID, tt.req)

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

func TestClient_UpdateExtensionResourceDefinition(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.ExtensionResourceDefinition {
		resp := &v1alpha1.ExtensionResourceDefinition{}
		if err := json.Unmarshal(r, resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	type fields struct {
		httpClient *mockHTTPDoer
	}

	tests := []struct {
		name         string
		extensionID  string
		erdID        string
		erdVersion   string
		fields       fields
		id           string
		req          *v1alpha1.ExtensionResourceDefinitionReq
		expected     *v1alpha1.ExtensionResourceDefinition
		expectedErr  error
		expectErr    bool
		expectedPath string
	}{
		{
			name:        "example request with slug",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expected:     testResp([]byte(testERDResponse)),
			expectedPath: "/api/v1alpha1/extensions/test-extension-1/erds/erd-1/v1alpha1",
			expectErr:    false,
		},
		{
			name:        "example request with id",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expected:     testResp([]byte(testERDResponse)),
			expectedPath: "/api/v1alpha1/extensions/test-extension-1/erds/a82a34a5-db1f-464f-af9c-76086e79f715",
			expectErr:    false,
		},
		{
			name:        "example request status accepted",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusAccepted,
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expected:  testResp([]byte(testERDResponse)),
			expectErr: false,
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expectErr:   true,
			expectedErr: ErrRequestNonSuccess,
		},
		{
			name:        "bad json response",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
					resp:       []byte(`{`),
				},
			},
			id: "test-extension-1",
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expectErr: true,
		},
		{
			name:       "missing extension id",
			erdID:      "erd-1",
			erdVersion: "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(""),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expectErr:   true,
			expectedErr: ErrMissingExtensionID,
		},
		{
			name:        "missing erd id",
			extensionID: "test-extension-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(""),
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expectErr:   true,
			expectedErr: ErrMissingERDID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"extension does not exist"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
			},
			expectErr:   true,
			expectedErr: v1alpha1.ErrExtensionNotFound,
		},
		{
			name:        "ERD not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(`{"error":"ERD does not exist"}`),
					statusCode: http.StatusNotFound,
				},
			},
			req: &v1alpha1.ExtensionResourceDefinitionReq{
				Description: "some test",
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
			got, err := c.UpdateExtensionResourceDefinition(context.TODO(), tt.extensionID, tt.erdID, tt.erdVersion, tt.req)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			} else if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)

			if tt.expectedPath != "" {
				assert.Equal(t, tt.expectedPath, tt.fields.httpClient.Request().URL.Path)
			}
		})
	}
}

func TestClient_DeleteExtensionResourceDefinition(t *testing.T) {
	type fields struct {
		httpClient *mockHTTPDoer
	}

	tests := []struct {
		name         string
		extensionID  string
		erdID        string
		erdVersion   string
		fields       fields
		expectedPath string
		expectedErr  error
		expectErr    bool
	}{
		{
			name:        "example request with slug",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			expectErr:    false,
			expectedPath: "/api/v1alpha1/extensions/test-extension-1/erds/erd-1/v1alpha1",
		},
		{
			name:        "example request with id",
			extensionID: "test-extension-1",
			erdID:       "a82a34a5-db1f-464f-af9c-76086e79f715",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       []byte(testERDResponse),
					statusCode: http.StatusOK,
				},
			},
			expectErr:    false,
			expectedPath: "/api/v1alpha1/extensions/test-extension-1/erds/a82a34a5-db1f-464f-af9c-76086e79f715",
		},
		{
			name:        "non-success",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
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
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingExtensionID,
		},
		{
			name:        "missing ERD id",
			extensionID: "test-extension-1",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			expectErr:   true,
			expectedErr: ErrMissingERDID,
		},
		{
			name:        "extension not found",
			extensionID: "test-extension-1",
			erdID:       "erd-1",
			erdVersion:  "v1alpha1",
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
			err := c.DeleteExtensionResourceDefinition(context.TODO(), tt.extensionID, tt.erdID, tt.erdVersion)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			} else if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.expectedPath != "" {
				assert.Equal(t, tt.expectedPath, tt.fields.httpClient.Request().URL.Path)
			}
		})
	}
}
