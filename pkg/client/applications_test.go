package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

var (
	testApplicationsResponse = []byte(`
[
    {
        "id": "9262f9ea-bdd5-4ecf-b1e8-6cb7bfbf3d61",
        "name": "Herp",
        "slug": "herp",
        "created_at": "2023-03-09T22:29:34.038843Z",
        "updated_at": "2023-03-09T22:29:34.038843Z",
        "deleted_at": null,
        "approver_group_id": null,
        "type_id": null
    },
    {
        "id": "14af17ea-6b60-4dd0-8ed4-f16d86756849",
        "name": "Derp",
        "slug": "derp",
        "kind": "splunk",
        "created_at": "2023-03-24T19:36:17.733744Z",
        "updated_at": "2023-03-24T19:38:24.823799Z",
        "deleted_at": null,
        "approver_group_id": null,
        "type_id": "bed9edd4-b44a-4dc6-ba41-902138f37bd6"
    }
]
`)

	testApplicationResponse = []byte(`
{
    "id": "14af17ea-6b60-4dd0-8ed4-f16d86756849",
    "name": "Derp",
    "slug": "derp",
    "created_at": "2023-03-24T19:36:17.733744Z",
    "updated_at": "2023-03-24T19:38:24.823799Z",
    "deleted_at": null,
    "approver_group_id": null,
    "type_id": "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
    "type": {
        "id": "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
        "name": "Gopherizer",
        "slug": "gopherizer",
        "description": "Integration with Gopherizer app",
        "logo_url": null,
        "created_at": "2023-03-24T18:57:14.531451Z",
        "updated_at": "2023-03-24T18:57:14.531451Z",
        "deleted_at": null
    }
}
`)

	testApplicationTypesResponse = []byte(`
[
	{
		"id": "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
		"name": "Gopherizer",
		"slug": "gopherizer",
		"description": "Integration with a Gopherizer app",
		"logo_url": null,
		"created_at": "2023-03-24T18:57:14.531451Z",
		"updated_at": "2023-03-24T18:57:14.531451Z",
		"deleted_at": null
	}
]
`)

	testApplicationTypeResponse = []byte(`
{
	"id": "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
	"name": "Gopherizer",
	"slug": "gopherizer",
	"description": "Integration with a Gopherizer app",
	"logo_url": null,
	"created_at": "2023-03-24T18:57:14.531451Z",
	"updated_at": "2023-03-24T18:57:14.531451Z",
	"deleted_at": null
}
`)
)

func TestClient_Applications(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.Application {
		resp := []*v1alpha1.Application{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	tests := []struct {
		name       string
		httpClient HTTPDoer
		want       []*v1alpha1.Application
		wantErr    bool
	}{
		{
			name: "example request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testApplicationsResponse,
				statusCode: http.StatusOK,
			},
			want: testResp(testApplicationsResponse),
		},
		{
			name: "example request status accepted",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testApplicationsResponse,
				statusCode: http.StatusOK,
			},
			want: testResp(testApplicationsResponse),
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			wantErr: true,
		},
		{
			name: "bad json response",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusOK,
				resp:       []byte(`{`),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}

			got, err := c.Applications(context.TODO())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_Application(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Application {
		resp := v1alpha1.Application{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return &resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		id      string
		want    *v1alpha1.Application
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testApplicationResponse,
					statusCode: http.StatusOK,
				},
			},
			id:   "14af17ea-6b60-4dd0-8ed4-f16d86756849",
			want: testResp(testApplicationResponse),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id:      "14af17ea-6b60-4dd0-8ed4-f16d86756849",
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
			id:      "14af17ea-6b60-4dd0-8ed4-f16d86756849",
			wantErr: true,
		},
		{
			name: "missing id in request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testApplicationResponse,
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

			got, err := c.Application(context.TODO(), tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_AddGroupToApplication(t *testing.T) {
	tests := []struct {
		name       string
		groupID    string
		appID      string
		httpClient HTTPDoer
		wantErr    bool
	}{
		{
			name: "example ok request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example no content request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusNoContent,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
			wantErr: true,
		},
		{
			name: "missing groupID in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			wantErr: true,
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "missing appID in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}

			err := c.AddGroupToApplication(context.TODO(), tt.groupID, tt.appID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_RemoveGroupFromApplication(t *testing.T) {
	tests := []struct {
		name       string
		groupID    string
		appID      string
		httpClient HTTPDoer
		wantErr    bool
	}{
		{
			name: "example ok request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example no content request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusNoContent,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
			wantErr: true,
		},
		{
			name: "missing groupID in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			wantErr: true,
			appID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "missing appID in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}

			err := c.RemoveGroupFromApplication(context.TODO(), tt.groupID, tt.appID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_ApplicationTypes(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.ApplicationType {
		resp := []*v1alpha1.ApplicationType{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return resp
	}

	tests := []struct {
		name       string
		httpClient HTTPDoer
		want       []*v1alpha1.ApplicationType
		wantErr    bool
	}{
		{
			name: "example request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testApplicationTypesResponse,
				statusCode: http.StatusOK,
			},
			want: testResp(testApplicationTypesResponse),
		},
		{
			name: "example request status accepted",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testApplicationTypesResponse,
				statusCode: http.StatusOK,
			},
			want: testResp(testApplicationTypesResponse),
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			wantErr: true,
		},
		{
			name: "bad json response",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusOK,
				resp:       []byte(`{`),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}

			got, err := c.ApplicationTypes(context.TODO())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_ApplicationType(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.ApplicationType {
		resp := v1alpha1.ApplicationType{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return &resp
	}

	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name    string
		fields  fields
		id      string
		want    *v1alpha1.ApplicationType
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testApplicationTypeResponse,
					statusCode: http.StatusOK,
				},
			},
			id:   "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
			want: testResp(testApplicationTypeResponse),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id:      "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
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
			id:      "bed9edd4-b44a-4dc6-ba41-902138f37bd6",
			wantErr: true,
		},
		{
			name: "missing id in request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testApplicationTypeResponse,
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

			got, err := c.ApplicationType(context.TODO(), tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
