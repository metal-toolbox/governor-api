package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

// Groups gets the list of groups from governor
func (c *Client) Groups(ctx context.Context) ([]*v1alpha1.Group, error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/groups", c.url, governorAPIVersionAlpha))
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

	out := []*v1alpha1.Group{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// Group gets the details of a group from governor
func (c *Client) Group(ctx context.Context, id string, deleted bool) (*v1alpha1.Group, error) {
	if id == "" {
		return nil, ErrMissingGroupID
	}

	g := fmt.Sprintf("%s/api/%s/groups/%s", c.url, governorAPIVersionAlpha, id)
	if deleted {
		g += "?deleted"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, g)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	c.logger.Debug("status code", zap.String("status code", resp.Status))

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrGroupNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrRequestNonSuccess
	}

	out := v1alpha1.Group{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GroupMembers returns a list of users in the given governor group
func (c *Client) GroupMembers(ctx context.Context, id string) ([]*v1alpha1.GroupMember, error) {
	if id == "" {
		return nil, ErrMissingGroupID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/groups/%s/users", c.url, governorAPIVersionAlpha, id))
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

	out := []*v1alpha1.GroupMember{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// GroupMemberRequests returns a list of member requests in the given governor group
func (c *Client) GroupMemberRequests(ctx context.Context, id string) ([]*v1alpha1.GroupMemberRequest, error) {
	if id == "" {
		return nil, ErrMissingGroupID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/groups/%s/requests", c.url, governorAPIVersionAlpha, id))
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

	out := []*v1alpha1.GroupMemberRequest{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// CreateGroup creates a new group in governor
func (c *Client) CreateGroup(ctx context.Context, group *v1alpha1.GroupReq) (*v1alpha1.Group, error) {
	if group == nil {
		return nil, ErrNilGroupRequest
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPost, fmt.Sprintf("%s/api/%s/groups", c.url, governorAPIVersionAlpha))
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(group)
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

	out := v1alpha1.Group{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// DeleteGroup deletes a group from governor
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	if id == "" {
		return ErrMissingGroupID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/groups/%s", c.url, governorAPIVersionAlpha, id))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	c.logger.Debug("status code", zap.String("status code", resp.Status))

	if resp.StatusCode == http.StatusNotFound {
		return ErrGroupNotFound
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return ErrRequestNonSuccess
	}

	return nil
}

// AddGroupMember adds a user to a group in governor
func (c *Client) AddGroupMember(ctx context.Context, groupID, userID string, admin bool) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if userID == "" {
		return ErrMissingUserID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPut, fmt.Sprintf("%s/api/%s/groups/%s/users/%s", c.url, governorAPIVersionAlpha, groupID, userID))
	if err != nil {
		return err
	}

	b, err := json.Marshal(struct {
		IsAdmin bool `json:"is_admin"`
	}{admin})
	if err != nil {
		return err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(b))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// RemoveGroupMember removes a user from a group in governor
func (c *Client) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if userID == "" {
		return ErrMissingUserID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/groups/%s/users/%s", c.url, governorAPIVersionAlpha, groupID, userID))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// AddGroupToOrganization links the group to the organization
func (c *Client) AddGroupToOrganization(ctx context.Context, groupID, orgID string) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if orgID == "" {
		return ErrMissingOrganizationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPut, fmt.Sprintf("%s/api/%s/groups/%s/organizations/%s", c.url, governorAPIVersionAlpha, groupID, orgID))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// RemoveGroupFromOrganization unlinks the group from the organization
func (c *Client) RemoveGroupFromOrganization(ctx context.Context, groupID, orgID string) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if orgID == "" {
		return ErrMissingOrganizationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/groups/%s/organizations/%s", c.url, governorAPIVersionAlpha, groupID, orgID))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// RemoveGroupMembershipRequest removes a user from a group in governor
func (c *Client) RemoveGroupMembershipRequest(ctx context.Context, groupID, requestID string) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if requestID == "" {
		return ErrMissingRequestID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/groups/%s/requests/%s", c.url, governorAPIVersionAlpha, groupID, requestID))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// GroupMembersAll returns a list of all group memberships across all groups
func (c *Client) GroupMembersAll(ctx context.Context, expired bool) ([]*v1alpha1.GroupMembership, error) {
	url := fmt.Sprintf("%s/api/%s/groups/memberships", c.url, governorAPIVersionAlpha)
	if expired {
		url += "?expired"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, url)
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

	out := []*v1alpha1.GroupMembership{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// GroupMembershipRequestsAll returns all group membership requests across all users and groups
func (c *Client) GroupMembershipRequestsAll(ctx context.Context, expired bool) ([]*v1alpha1.GroupMemberRequest, error) {
	url := fmt.Sprintf("%s/api/%s/groups/requests", c.url, governorAPIVersionAlpha)
	if expired {
		url += "?expired"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, url)
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

	out := []*v1alpha1.GroupMemberRequest{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}
