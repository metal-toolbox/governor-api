package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/goccy/go-json"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/metal-toolbox/governor-api/pkg/api/v1beta1"
)

// Users gets the list of users from governor
// when deleted is true it will also return deleted users
func (c *Client) Users(ctx context.Context, deleted bool) ([]*v1alpha1.User, error) {
	u := fmt.Sprintf("%s/api/%s/users", c.url, governorAPIVersionAlpha)
	if deleted {
		u += "?deleted"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrRequestNonSuccess
	}

	out := []*v1alpha1.User{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// UsersQuery searches for a user in governor with the passed query
func (c *Client) UsersQuery(ctx context.Context, query map[string][]string) ([]*v1alpha1.User, error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/users", c.url, governorAPIVersionAlpha))
	if err != nil {
		return nil, err
	}

	q := url.Values{}

	for k, vals := range query {
		for _, v := range vals {
			q.Add(k, v)
		}
	}

	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrRequestNonSuccess
	}

	out := []*v1alpha1.User{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// User gets the details of a user from governor
// when deleted is true it will return information about a deleted user
func (c *Client) User(ctx context.Context, id string, deleted bool) (*v1alpha1.User, error) {
	if id == "" {
		return nil, ErrMissingUserID
	}

	u := fmt.Sprintf("%s/api/%s/users/%s", c.url, governorAPIVersionAlpha, id)
	if deleted {
		u += "?deleted"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrRequestNonSuccess
	}

	out := v1alpha1.User{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// CreateUser creates a user in governor and returns the user
func (c *Client) CreateUser(ctx context.Context, user *v1alpha1.UserReq) (*v1alpha1.User, error) {
	if user == nil {
		return nil, ErrNilUserRequest
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPost, fmt.Sprintf("%s/api/%s/users", c.url, governorAPIVersionAlpha))
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(b))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, ErrRequestNonSuccess
	}

	out := v1alpha1.User{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// DeleteUser deletes a user in governor
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	if id == "" {
		return ErrMissingUserID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/users/%s", c.url, governorAPIVersionAlpha, id))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return ErrRequestNonSuccess
	}

	return nil
}

// UpdateUser updates a user in governor and returns the user
func (c *Client) UpdateUser(ctx context.Context, id string, user *v1alpha1.UserReq) (*v1alpha1.User, error) {
	if id == "" {
		return nil, ErrMissingUserID
	}

	if user == nil {
		return nil, ErrNilUserRequest
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPut, fmt.Sprintf("%s/api/%s/users/%s", c.url, governorAPIVersionAlpha, id))
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(b))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, ErrRequestNonSuccess
	}

	out := v1alpha1.User{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// UsersV2 gets the list of users from governor using paginated api
func (c *Client) UsersV2(ctx context.Context, queryParams map[string][]string) ([]*v1beta1.User, error) {
	users := []*v1beta1.User{}

	var nextCursor string

	// iterate until we get all the records
	for {
		if nextCursor != "" {
			queryParams["next_cursor"] = []string{nextCursor}
		}

		out, err := c.UsersQueryV2(ctx, queryParams)
		if err != nil {
			return nil, err
		}

		users = append(users, out.Records...)

		if out.NextCursor == "" {
			break
		}

		nextCursor = out.NextCursor
	}

	return users, nil
}

// UsersQueryV2 searches for users in governor with the passed query
func (c *Client) UsersQueryV2(ctx context.Context, query map[string][]string) (*v1beta1.PaginationResponse[*v1beta1.User], error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/users", c.url, governorAPIVersionBeta))
	nilResp := v1beta1.PaginationResponse[*v1beta1.User]{}

	if err != nil {
		return &nilResp, err
	}

	q := url.Values{}

	for k, vals := range query {
		for _, v := range vals {
			q.Add(k, v)
		}
	}

	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return &nilResp, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &nilResp, ErrRequestNonSuccess
	}

	out := v1beta1.PaginationResponse[*v1beta1.User]{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return &nilResp, err
	}

	return &out, nil
}
