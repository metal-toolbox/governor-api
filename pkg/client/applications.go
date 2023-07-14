package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/goccy/go-json"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

// AddGroupToApplication links the group to the application
func (c *Client) AddGroupToApplication(ctx context.Context, groupID, appID string) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if appID == "" {
		return ErrMissingApplicationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPut, fmt.Sprintf("%s/api/%s/groups/%s/applications/%s", c.url, governorAPIVersionAlpha, groupID, appID))
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

// RemoveGroupFromApplication unlinks the group from the application
func (c *Client) RemoveGroupFromApplication(ctx context.Context, groupID, appID string) error {
	if groupID == "" {
		return ErrMissingGroupID
	}

	if appID == "" {
		return ErrMissingApplicationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/api/%s/groups/%s/applications/%s", c.url, governorAPIVersionAlpha, groupID, appID))
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

// Applications gets the list of applications from governor
func (c *Client) Applications(ctx context.Context) ([]*v1alpha1.Application, error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/applications", c.url, governorAPIVersionAlpha))
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

	out := []*v1alpha1.Application{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// Application gets the details of an app from governor
func (c *Client) Application(ctx context.Context, id string) (*v1alpha1.Application, error) {
	if id == "" {
		return nil, ErrMissingApplicationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/applications/%s", c.url, governorAPIVersionAlpha, id))
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

	out := v1alpha1.Application{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ApplicationGroups gets a list of groups linked to the given governor application
func (c *Client) ApplicationGroups(ctx context.Context, id string) ([]*v1alpha1.Group, error) {
	if id == "" {
		return nil, ErrMissingApplicationID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/applications/%s/groups", c.url, governorAPIVersionAlpha, id))
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

// ApplicationTypes gets the list of application types from governor
func (c *Client) ApplicationTypes(ctx context.Context) ([]*v1alpha1.ApplicationType, error) {
	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/application-types", c.url, governorAPIVersionAlpha))
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

	out := []*v1alpha1.ApplicationType{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

// ApplicationType gets the details of an application type from governor
func (c *Client) ApplicationType(ctx context.Context, id string) (*v1alpha1.ApplicationType, error) {
	if id == "" {
		return nil, ErrMissingApplicationTypeID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/application-types/%s", c.url, governorAPIVersionAlpha, id))
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

	out := v1alpha1.ApplicationType{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ApplicationTypeApps gets a list of governor applications with the given application type
func (c *Client) ApplicationTypeApps(ctx context.Context, id string) ([]*v1alpha1.Application, error) {
	if id == "" {
		return nil, ErrMissingApplicationTypeID
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/%s/application-types/%s/applications", c.url, governorAPIVersionAlpha, id))
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

	out := []*v1alpha1.Application{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}
