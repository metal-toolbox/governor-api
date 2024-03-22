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
	testGroupsResponse = []byte(`
	[
		{
			"id": "70674d51-43e0-4539-b6be-b030c0f9e6aa",
			"name": "Streets and Sanitation",
			"slug": "streets-and-sanitation",
			"description": "Keepin it clean",
			"created_at": "2022-08-11T14:38:33.027346Z",
			"updated_at": "2022-08-11T14:38:33.027346Z",
			"deleted_at": null
		},
		{
			"id": "6a603c55-4787-4916-9934-70dbeb8467f7",
			"name": "Arts and Culture",
			"slug": "arts-and-culture",
			"description": "Keepin it classy",
			"created_at": "2022-08-11T14:38:33.027346Z",
			"updated_at": "2022-08-11T14:38:33.027346Z",
			"deleted_at": null
		},
		{
			"id": "6a603c55-4787-4916-9934-70dbeb8467f7",
			"name": "Budget Office",
			"slug": "budget-office",
			"description": "Keepin it real",
			"created_at": "2022-08-11T14:38:33.027346Z",
			"updated_at": "2022-08-11T14:38:33.027346Z",
			"deleted_at": null
		}
	]
	`)

	testGroupResponse = []byte(`
	{
		"id": "8923e54d-0df6-407a-832d-2917915a3ff7",
		"name": "Parks and Public Works",
		"slug": "parks-and-public-works",
		"description": "Go out and play",
		"created_at": "2022-08-11T14:38:33.027346Z",
		"updated_at": "2022-08-11T14:38:33.027346Z",
		"deleted_at": null
	}
	`)

	testGroupMembersResponse = []byte(`
	[
		{
			"id": "e62f740b-6657-4344-97ef-be74a820c794",
			"name": "Mighty Mole",
			"email": "mmole@gopher.net",
			"avatar_url": "https://gopher.net/avatars/mmole.png",
			"status": "active",
			"is_admin": true
		},
		{
			"id": "8ce61cb3-b243-4b9e-8a09-deb59c639ef6",
			"name": "Dirt Devil",
			"email": "ddevil@gopher.net",
			"avatar_url": "https://gopher.net/avatars/ddevil.png",
			"status": "active",
			"is_admin": false
		}
	]
	`)

	testGroupMemberRequestsResponse = []byte(`
	[
		{
			"id": "4da2d4ac-1a12-400c-91cc-fba8ee00cae9",
			"group_id": "36747a95-3952-464a-84d6-cb12d56a4921",
			"group_name": "Gophers",
			"group_slug": "gophers",
			"user_id": "aa6d9425-ed15-415f-950d-b3a8c6d01430",
			"user_name": "Burrow Blaster",
			"user_email": "bblaster@gopher.net",
			"user_avatar_url": "https://gopher.net/avatars/bblaster.png",
			"created_at": "2023-05-05T18:10:14.14363Z",
			"updated_at": "2023-05-05T18:10:14.14363Z",
			"is_admin": false,
			"note": "I belong in this group."
		}
	]
`)

	testGroupMembersAllResponse = []byte(`
[
	{
		"id": "4da2d4ac-1a12-400c-91cc-fba8ee00cae9",
		"group_id": "36747a95-3952-464a-84d6-cb12d56a4921",
		"group_slug": "gophers",
		"user_id": "aa6d9425-ed15-415f-950d-b3a8c6d01430",
		"user_email": "bblaster@gopher.net"
	}
]
`)
)

func TestClient_Groups(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.Group {
		resp := []*v1alpha1.Group{}
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
		want    []*v1alpha1.Group
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupsResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testGroupsResponse),
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
			got, err := c.Groups(context.TODO())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_Group(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Group {
		resp := v1alpha1.Group{}
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
		want    *v1alpha1.Group
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupResponse,
					statusCode: http.StatusOK,
				},
			},
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
			want: testResp(testGroupResponse),
		},
		{
			name: "not found",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusNotFound,
				},
			},
			id:      "8923e54d-0df6-407a-832d-2917915a3ff7",
			wantErr: true,
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			id:      "8923e54d-0df6-407a-832d-2917915a3ff7",
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
			id:      "8923e54d-0df6-407a-832d-2917915a3ff7",
			wantErr: true,
		},
		{
			name: "missing id in request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupResponse,
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
			got, err := c.Group(context.TODO(), tt.id, false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_GroupMembers(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.GroupMember {
		resp := []*v1alpha1.GroupMember{}
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
		id      string
		fields  fields
		want    []*v1alpha1.GroupMember
		wantErr bool
	}{
		{
			name: "example request",
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupMembersResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testGroupMembersResponse),
		},
		{
			name: "missing id in request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupMembersResponse,
					statusCode: http.StatusOK,
				},
			},
			wantErr: true,
		},
		{
			name: "non-success",
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
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
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
			fields: fields{
				httpClient: &mockHTTPDoer{
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
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.GroupMembers(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_GroupMemberRequests(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.GroupMemberRequest {
		resp := []*v1alpha1.GroupMemberRequest{}
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
		id      string
		fields  fields
		want    []*v1alpha1.GroupMemberRequest
		wantErr bool
	}{
		{
			name: "example request",
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupMemberRequestsResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testGroupMemberRequestsResponse),
		},
		{
			name: "missing id in request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupMemberRequestsResponse,
					statusCode: http.StatusOK,
				},
			},
			wantErr: true,
		},
		{
			name: "non-success",
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
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
			id:   "8923e54d-0df6-407a-832d-2917915a3ff7",
			fields: fields{
				httpClient: &mockHTTPDoer{
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
				url:                    "https://the.gov/",
				logger:                 zap.NewNop(),
				httpClient:             tt.fields.httpClient,
				clientCredentialConfig: &mockTokener{t: t},
				token:                  &oauth2.Token{AccessToken: "topSekret"},
			}
			got, err := c.GroupMemberRequests(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_CreateGroup(t *testing.T) {
	testResp := func(r []byte) *v1alpha1.Group {
		resp := v1alpha1.Group{}
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
		req     *v1alpha1.GroupReq
		want    *v1alpha1.Group
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupResponse,
					statusCode: http.StatusOK,
				},
			},
			req: &v1alpha1.GroupReq{
				Name:        "Gophers",
				Description: "A group for gophers",
				Note:        "I propose we create a group for gophers.",
			},
			want: testResp(testGroupResponse),
		},
		{
			name: "example request status accepted",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupResponse,
					statusCode: http.StatusAccepted,
				},
			},
			req: &v1alpha1.GroupReq{
				Name:        "Gophers",
				Description: "A group for gophers",
				Note:        "I propose we create a group for gophers.",
			},
			want: testResp(testGroupResponse),
		},
		{
			name: "non-success",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					statusCode: http.StatusInternalServerError,
				},
			},
			req: &v1alpha1.GroupReq{
				Name:        "Gophers",
				Description: "A group for gophers",
				Note:        "I propose we create a group for gophers.",
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
			req: &v1alpha1.GroupReq{
				Name:        "Gophers",
				Description: "A group for gophers",
				Note:        "I propose we create a group for gophers.",
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
			got, err := c.CreateGroup(context.TODO(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_DeleteGroup(t *testing.T) {
	tests := []struct {
		name       string
		id         string
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
			id: "8923e54d-0df6-407a-832d-2917915a3ff7",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			id: "8923e54d-0df6-407a-832d-2917915a3ff7",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			id:      "8923e54d-0df6-407a-832d-2917915a3ff7",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			id:      "8923e54d-0df6-407a-832d-2917915a3ff7",
			wantErr: true,
		},
		{
			name: "missing id in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
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
			err := c.DeleteGroup(context.TODO(), tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_AddGroupMember(t *testing.T) {
	tests := []struct {
		name       string
		httpClient HTTPDoer
		groupID    string
		userID     string
		wantErr    bool
	}{
		{
			name: "example ok request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "Ninja",
			userID:  "Zane",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			groupID: "Ninja",
			userID:  "Cole",
		},
		{
			name: "example no content request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusNoContent,
			},
			groupID: "Ninja",
			userID:  "JayWalker",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			groupID: "Ninja",
			userID:  "Nya",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			groupID: "Ninja",
			userID:  "Kruncha",
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
			userID:  "MasterWu",
		},
		{
			name: "missing orgID in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "Skulkin",
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
			err := c.AddGroupMember(context.TODO(), tt.groupID, tt.userID, false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_RemoveGroupMember(t *testing.T) {
	tests := []struct {
		name       string
		httpClient HTTPDoer
		groupID    string
		userID     string
		wantErr    bool
	}{
		{
			name: "example ok request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "mushroom-kingdom",
			userID:  "mario",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			groupID: "mushroom-kingdom",
			userID:  "peach",
		},
		{
			name: "example no content request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusNoContent,
			},
			groupID: "mushroom-kingdom",
			userID:  "toadstool",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			groupID: "mushroom-kingdom",
			userID:  "kong",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			groupID: "mushroom-kingdom",
			userID:  "bowser",
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
			userID:  "cappy",
		},
		{
			name: "missing orgID in request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusOK,
			},
			groupID: "mushroom-kingdom",
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
			err := c.RemoveGroupMember(context.TODO(), tt.groupID, tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_AddGroupToOrganization(t *testing.T) {
	tests := []struct {
		name       string
		groupID    string
		orgID      string
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
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example no content request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusNoContent,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
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
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "missing orgID in request",
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
			err := c.AddGroupToOrganization(context.TODO(), tt.groupID, tt.orgID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_RemoveGroupFromOrganization(t *testing.T) {
	tests := []struct {
		name       string
		groupID    string
		orgID      string
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
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example accepted request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusAccepted,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "example no content request",
			httpClient: &mockHTTPDoer{
				t:          t,
				resp:       testGroupResponse,
				statusCode: http.StatusNoContent,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "not found",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusNotFound,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
			wantErr: true,
		},
		{
			name: "non-success",
			httpClient: &mockHTTPDoer{
				t:          t,
				statusCode: http.StatusInternalServerError,
			},
			groupID: "8923e54d-0df6-407a-832d-2917915a3ff7",
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
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
			orgID:   "bde11bd6-66b7-4f1b-9d4b-0a8a86b2e097",
		},
		{
			name: "missing orgID in request",
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
			err := c.RemoveGroupFromOrganization(context.TODO(), tt.groupID, tt.orgID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestClient_GroupMembersAll(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.GroupMembership {
		resp := []*v1alpha1.GroupMembership{}
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
		want    []*v1alpha1.GroupMembership
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupMembersAllResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testGroupMembersAllResponse),
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
			got, err := c.GroupMembersAll(context.TODO(), false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_GroupMemberRequestsAll(t *testing.T) {
	testResp := func(r []byte) []*v1alpha1.GroupMemberRequest {
		resp := []*v1alpha1.GroupMemberRequest{}
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
		want    []*v1alpha1.GroupMemberRequest
		wantErr bool
	}{
		{
			name: "example request",
			fields: fields{
				httpClient: &mockHTTPDoer{
					t:          t,
					resp:       testGroupMemberRequestsResponse,
					statusCode: http.StatusOK,
				},
			},
			want: testResp(testGroupMemberRequestsResponse),
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
			got, err := c.GroupMembershipRequestsAll(context.TODO(), false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
