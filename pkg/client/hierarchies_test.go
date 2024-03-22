package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/volatiletech/null/v8"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func TestClient_GroupHierarchies(t *testing.T) {
	testResp := func(r []byte) *[]v1alpha1.GroupHierarchy {
		resp := []v1alpha1.GroupHierarchy{}
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
		want    *[]v1alpha1.GroupHierarchy
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupHierarchiesResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testGroupHierarchiesResponse),
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
			got, err := c.GroupHierarchies(context.TODO())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_MemberGroups(t *testing.T) {
	testResp := func(r []byte) *[]v1alpha1.GroupHierarchy {
		resp := []v1alpha1.GroupHierarchy{}
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
		want    *[]v1alpha1.GroupHierarchy
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testMemberGroupsResponse,
					statusCode: http.StatusOK,
				},
			},
			id:   "186c5a52-4421-4573-8bbf-78d85d3c277e",
			want: testResp(testMemberGroupsResponse),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
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
				httpClient: &mockHTTPDoer{
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
			got, err := c.MemberGroups(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_AddMemberGroup(t *testing.T) {
	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name          string
		fields        fields
		parentGroupID string
		memberGroupID string
		expiresAt     null.Time
		wantErr       bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testMemberGroupsResponse,
					statusCode: http.StatusOK,
				},
			},
			parentGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			memberGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277f",
			expiresAt:     null.Time{},
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			parentGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			memberGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277f",
			expiresAt:     null.Time{},
			wantErr:       true,
		},
		{
			name: "missing fields in request",
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
			err := c.AddMemberGroup(context.TODO(), tt.parentGroupID, tt.memberGroupID, tt.expiresAt)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_UpdateMemberGroup(t *testing.T) {
	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name          string
		fields        fields
		parentGroupID string
		memberGroupID string
		expiresAt     null.Time
		wantErr       bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testMemberGroupsResponse,
					statusCode: http.StatusOK,
				},
			},
			parentGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			memberGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277f",
			expiresAt:     null.Time{},
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			parentGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			memberGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277f",
			expiresAt:     null.Time{},
			wantErr:       true,
		},
		{
			name: "missing fields in request",
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
			err := c.UpdateMemberGroup(context.TODO(), tt.parentGroupID, tt.memberGroupID, tt.expiresAt)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_DeleteMemberGroup(t *testing.T) {
	type fields struct {
		httpClient HTTPDoer
	}

	tests := []struct {
		name          string
		fields        fields
		parentGroupID string
		memberGroupID string
		wantErr       bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testMemberGroupsResponse,
					statusCode: http.StatusOK,
				},
			},
			parentGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			memberGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277f",
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			parentGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277e",
			memberGroupID: "186c5a52-4421-4573-8bbf-78d85d3c277f",
			wantErr:       true,
		},
		{
			name: "missing fields in request",
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
			err := c.DeleteMemberGroup(context.TODO(), tt.parentGroupID, tt.memberGroupID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
