package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/volatiletech/null/v8"
)

// GroupHierarchies lists all hierarchical group relationships in governor
func (c *Client) GroupHierarchies(ctx context.Context) (*[]v1alpha1.GroupHierarchy, error) {
	u := fmt.Sprintf("%s/api/%s/groups/hierarchies", c.url, governorAPIVersionAlpha)

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

	out := []v1alpha1.GroupHierarchy{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// MemberGroups lists member groups of a parent group in governor
func (c *Client) MemberGroups(ctx context.Context, id string) (*[]v1alpha1.GroupHierarchy, error) {
	if id == "" {
		return nil, ErrMissingGroupID
	}

	u := fmt.Sprintf("%s/api/%s/groups/%s/hierarchies", c.url, governorAPIVersionAlpha, id)

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

	out := []v1alpha1.GroupHierarchy{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// AddMemberGroup creates a new group hierarchy relationship in governor
func (c *Client) AddMemberGroup(ctx context.Context, parentGroupID, memberGroupID string, expiresAt null.Time) error {
	if parentGroupID == "" || memberGroupID == "" {
		return ErrNilGroupRequest
	}

	body := struct {
		ExpiresAt     null.Time `json:"expires_at"`
		MemberGroupID string    `json:"member_group_id"`
	}{
		ExpiresAt:     expiresAt,
		MemberGroupID: memberGroupID,
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPost, fmt.Sprintf("%s/api/%s/groups/%s/hierarchies", c.url, governorAPIVersionAlpha, parentGroupID))
	if err != nil {
		return err
	}

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(b))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// UpdateMemberGroup updates the expiration on a group hierarchy relationship in governor
func (c *Client) UpdateMemberGroup(ctx context.Context, parentGroupID, memberGroupID string, expiresAt null.Time) error {
	if parentGroupID == "" || memberGroupID == "" {
		return ErrNilGroupRequest
	}

	body := struct {
		ExpiresAt null.Time `json:"expires_at"`
	}{
		ExpiresAt: expiresAt,
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPatch, fmt.Sprintf("%s/api/%s/groups/%s/hierarchies/%s", c.url, governorAPIVersionAlpha, parentGroupID, memberGroupID))
	if err != nil {
		return err
	}

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req.Body = io.NopCloser(bytes.NewBuffer(b))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}

// DeleteMemberGroup deletes a group hierarchy relationship in governor
func (c *Client) DeleteMemberGroup(ctx context.Context, parentGroupID, memberGroupID string) error {
	if parentGroupID == "" || memberGroupID == "" {
		return ErrNilGroupRequest
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/groups/%s/hierarchies/%s", c.url, governorAPIVersionAlpha, parentGroupID, memberGroupID))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}
