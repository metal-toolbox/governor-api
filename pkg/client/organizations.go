package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/goccy/go-json"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

// Organization gets the details of an org from governor
func (c *Client) Organization(ctx context.Context, id string) (*v1alpha1.Organization, error) {
	if id == "" {
		return nil, ErrMissingOrganizationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/organizations/%s", c.url, governorAPIVersionAlpha, id))
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

	out := v1alpha1.Organization{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// Organizations gets the list of organizations from governor
func (c *Client) Organizations(ctx context.Context) ([]*v1alpha1.Organization, error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/organizations", c.url, governorAPIVersionAlpha))
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

	out := []*v1alpha1.Organization{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// OrganizationGroups gets the list of groups assigned to an organization from governor
func (c *Client) OrganizationGroups(ctx context.Context, org string) ([]*v1alpha1.Group, error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/organizations/%s/groups", c.url, governorAPIVersionAlpha, org))
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
