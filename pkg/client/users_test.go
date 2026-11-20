package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/aarondl/null/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/pkg/api/types"
	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/metal-toolbox/governor-api/pkg/api/v1beta1"
)

func TestClient_Users(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.User {
		resp := []*v1alpha1.User{}
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
		fields  fields
		want    []*v1alpha1.User
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUsersResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testUsersResponse),
		},
		{
			name: "non-success",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
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
			got, err := c.Users(context.TODO(), false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_UsersV2(t *testing.T) {
	testResp := func(r []byte) []*v1beta1.User {
		resp := []*v1beta1.User{}
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
		fields  fields
		want    []*v1beta1.User
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				transport: &mockMultiTransport{
					t:          t,
					resp:       testUsersResponseV2,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testUsersResponse),
		},
		{
			name: "non-success",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
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
			got, err := c.UsersV2(context.TODO(), map[string][]string{})

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_User(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.User {
		resp := v1alpha1.User{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return &resp
	}

	type fields struct {
		transport http.RoundTripper
	}

	tests := []struct {
		name    string
		fields  fields
		id      string
		want    *v1alpha1.User
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusOK,
				},
			},
			id:   "186c5a52-4421-4573-8bbf-78d85d3c277e",
			want: testResp(testUserResponse),
		},
		{
			name: "non-success",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id:      "186c5a52-4421-4573-8bbf-78d85d3c277e",
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
			id:      "186c5a52-4421-4573-8bbf-78d85d3c277e",
			wantErr: true,
		},
		{
			name: "missing id in request",
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
			got, err := c.User(context.TODO(), tt.id, false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_CreateUser(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.User {
		resp := v1alpha1.User{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return &resp
	}

	type fields struct {
		transport http.RoundTripper
	}

	tests := []struct {
		name    string
		fields  fields
		req     *v1alpha1.UserReq
		want    *v1alpha1.User
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.UserReq{
				ExternalIDs: &[]types.ExternalID{{Iss: "https://sso.example.com/oauth2/default", ID: "000016"}},
				Name:        "John Trumbull",
				Email:       "jtrumbull@ct.gov",
			},
			want: testResp(testUserResponse),
		},
		{
			name: "example request status accepted",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusAccepted,
				},
			},
			req: &v1alpha1.UserReq{
				ExternalIDs: &[]types.ExternalID{{Iss: "https://sso.example.com/oauth2/default", ID: "000001"}},
				Name:        "John Trumbull",
				Email:       "jtrumbull@ct.gov",
			},
			want: testResp(testUserResponse),
		},
		{
			name: "non-success",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req: &v1alpha1.UserReq{
				ExternalIDs: &[]types.ExternalID{{Iss: "https://sso.example.com/oauth2/default", ID: "999991"}},
				Name:        "Test One",
				Email:       "test1@test.gov",
			},
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
			req: &v1alpha1.UserReq{
				ExternalIDs: &[]types.ExternalID{{Iss: "https://sso.example.com/oauth2/default", ID: "999992"}},
				Name:        "Test Two",
				Email:       "test2@test.gov",
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
			got, err := c.CreateUser(context.TODO(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_DeleteUser(t *testing.T) {
	type fields struct {
		transport http.RoundTripper
	}

	tests := []struct {
		name    string
		fields  fields
		id      string
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusOK,
				},
			},
			id: "186c5a52-4421-4573-8bbf-78d85d3c277e",
		},
		{
			name: "example request accepted",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusAccepted,
				},
			},
			id: "186c5a52-4421-4573-8bbf-78d85d3c277e",
		},
		{
			name: "non-success",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id:      "186c5a52-4421-4573-8bbf-78d85d3c277e",
			wantErr: true,
		},
		{
			name: "missing id in request",
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
			err := c.DeleteUser(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_UpdateUser(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.User {
		resp := v1alpha1.User{}
		if err := json.Unmarshal(r, &resp); err != nil {
			t.Error(err)
		}

		return &resp
	}

	type fields struct {
		transport http.RoundTripper
	}

	tests := []struct {
		name    string
		fields  fields
		id      string
		req     *v1alpha1.UserReq
		want    *v1alpha1.User
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusOK,
				},
			},
			id: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: &v1alpha1.UserReq{
				GithubUsername: "johnnyTog",
			},
			want: testResp(testUserResponse),
		},
		{
			name: "example request status accepted",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					resp:       testUserResponse,
					statusCode: http.StatusAccepted,
				},
			},
			id: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: &v1alpha1.UserReq{
				GithubUsername: "johnnyTog",
			},
			want: testResp(testUserResponse),
		},
		{
			name: "non-success",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: &v1alpha1.UserReq{
				GithubUsername: "johnnyTog",
			},
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
			id: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			req: &v1alpha1.UserReq{
				GithubUsername: "johnnyTog",
			},
			wantErr: true,
		},
		{
			name: "missing user id",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.UserReq{
				GithubUsername: "johnnyTog",
			},
			wantErr: true,
		},
		{
			name: "missing request",
			fields: fields{
				transport: &mockTransport{
					t:          t,
					statusCode: http.StatusOK,
				},
			},
			id:      "186c5a52-4421-4573-8bbf-78d85d3c277e",
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
			got, err := c.UpdateUser(context.TODO(), tt.id, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUserResponseExternalIDs(t *testing.T) {
	var u v1alpha1.User
	require.NoError(t, json.Unmarshal(testUserResponse, &u))

	assert.Equal(t, []types.ExternalID{
		{Iss: "https://sso.example.com/oauth2/default", ID: "000016"},
		{Iss: "https://auth.pingone.example.com/as", ID: "ping-000016"},
	}, u.ExternalIDs)
	assert.Equal(t, null.StringFrom("000016"), u.ExternalID) //nolint:staticcheck // clients still decode the deprecated field for one release
}

func TestClient_UserByExternalID(t *testing.T) {
	const (
		iss        = "https://auth.pingone.example.com/as"
		externalID = "ping-000016"
	)

	testResp := func(r []byte) *v1alpha1.User {
		var u v1alpha1.User
		if err := json.Unmarshal(r, &u); err != nil {
			t.Error(err)
		}

		return &u
	}

	tests := []struct {
		name       string
		statusCode int
		resp       []byte
		want       *v1alpha1.User
		wantErr    error
	}{
		{
			name:       "found",
			statusCode: http.StatusOK,
			resp:       []byte("[" + string(testUserResponse) + "]"),
			want:       testResp(testUserResponse),
		},
		{
			name:       "not found",
			statusCode: http.StatusOK,
			resp:       []byte(`[]`),
			wantErr:    ErrUserNotFound,
		},
		{
			name:       "non-success",
			statusCode: http.StatusInternalServerError,
			wantErr:    ErrRequestNonSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &mockTransport{t: t, statusCode: tt.statusCode, resp: tt.resp}
			c := &Client{
				url:        "https://the.gov",
				logger:     zap.NewNop(),
				httpClient: &http.Client{Transport: transport},
			}

			got, err := c.UserByExternalID(context.TODO(), iss, externalID)

			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, "/api/v1alpha1/users", transport.Request().URL.Path)
			assert.Equal(t, url.Values{"external_id": {externalID}, "iss": {iss}}, transport.Request().URL.Query())
		})
	}
}

// TestUserReqExternalIDsFromPublicTypes builds a request with the external
// IDs type that client modules can import.
func TestUserReqExternalIDsFromPublicTypes(t *testing.T) {
	transport := &mockTransport{t: t, statusCode: http.StatusAccepted, resp: testUserResponse}
	c := &Client{
		url:        "https://the.gov",
		logger:     zap.NewNop(),
		httpClient: &http.Client{Transport: transport},
	}

	req := &v1alpha1.UserReq{
		Name:        "John Trumbull",
		Email:       "jtrumbull@ct.gov",
		ExternalIDs: &[]types.ExternalID{{Iss: "https://sso.example.com/oauth2/default", ID: "000016"}},
	}

	got, err := c.CreateUser(context.TODO(), req)
	require.NoError(t, err)

	body, err := io.ReadAll(transport.Request().Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name": "John Trumbull", "email": "jtrumbull@ct.gov",
		"external_ids": [{"iss": "https://sso.example.com/oauth2/default", "id": "000016"}]}`, string(body))

	assert.Equal(t, []types.ExternalID{
		{Iss: "https://sso.example.com/oauth2/default", ID: "000016"},
		{Iss: "https://auth.pingone.example.com/as", ID: "ping-000016"},
	}, got.ExternalIDs)
}
