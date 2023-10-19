package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

// SystemExtensionResource fetches a system extension resource
func (c *Client) SystemExtensionResource(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion, resourceID string, deleted bool,
) (*v1alpha1.SystemExtensionResource, error) {
	if extensionSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	if erdSlugPlural == "" {
		return nil, ErrMissingERDIDOrSlug
	}

	if resourceID == "" {
		return nil, ErrMissingResourceID
	}

	u := fmt.Sprintf(
		"%s/api/%s/extension-resources/%s/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		extensionSlug,
		erdSlugPlural,
		erdVersion,
		resourceID,
	)

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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, handleERDStatusNotFound(respBody)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	ser := &v1alpha1.SystemExtensionResource{}
	if err := json.Unmarshal(respBody, ser); err != nil {
		return nil, err
	}

	return ser, nil
}

// SystemExtensionResources list all system resources
func (c *Client) SystemExtensionResources(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion string, deleted bool,
) ([]*v1alpha1.SystemExtensionResource, error) {
	if extensionSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	u := fmt.Sprintf(
		"%s/api/%s/extension-resources/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		extensionSlug,
		erdSlugPlural,
		erdVersion,
	)

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

	if resp.StatusCode == http.StatusNotFound {
		return nil, v1alpha1.ErrExtensionNotFound
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	sers := []*v1alpha1.SystemExtensionResource{}
	if err := json.Unmarshal(respBody, &sers); err != nil {
		return nil, err
	}

	return sers, nil
}

// CreateSystemExtensionResource creates a system extension resource
func (c *Client) CreateSystemExtensionResource(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion string, resource interface{},
) (*v1alpha1.SystemExtensionResource, error) {
	if extensionSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	if erdSlugPlural == "" {
		return nil, ErrMissingERDIDOrSlug
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPost,
		fmt.Sprintf(
			"%s/api/%s/extension-resources/%s/%s/%s",
			c.url,
			governorAPIVersionAlpha,
			extensionSlug,
			erdSlugPlural,
			erdVersion,
		),
	)
	if err != nil {
		return nil, err
	}

	resourceReq, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(resourceReq))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, v1alpha1.ErrExtensionNotFound
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ser := &v1alpha1.SystemExtensionResource{}
	if err := json.Unmarshal(respBody, ser); err != nil {
		return nil, err
	}

	return ser, nil
}

// UpdateSystemExtensionResource updates a system extension resource
func (c *Client) UpdateSystemExtensionResource(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion, resourceID string, resource interface{},
) (*v1alpha1.SystemExtensionResource, error) {
	if extensionSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	if erdSlugPlural == "" {
		return nil, ErrMissingERDIDOrSlug
	}

	if resourceID == "" {
		return nil, ErrMissingResourceID
	}

	u := fmt.Sprintf(
		"%s/api/%s/extension-resources/%s/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		extensionSlug,
		erdSlugPlural,
		erdVersion,
		resourceID,
	)

	req, err := c.newGovernorRequest(ctx, http.MethodPatch, u)
	if err != nil {
		return nil, err
	}

	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(resourceJSON))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, handleERDStatusNotFound(respBody)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	ser := &v1alpha1.SystemExtensionResource{}
	if err := json.Unmarshal(respBody, ser); err != nil {
		return nil, err
	}

	return ser, nil
}

// DeleteSystemExtensionResource deletes a system extension resource
func (c *Client) DeleteSystemExtensionResource(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion, resourceID string,
) error {
	if extensionSlug == "" {
		return ErrMissingExtensionIDOrSlug
	}

	if erdSlugPlural == "" {
		return ErrMissingERDIDOrSlug
	}

	if resourceID == "" {
		return ErrMissingResourceID
	}

	u := fmt.Sprintf(
		"%s/api/%s/extension-resources/%s/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		extensionSlug,
		erdSlugPlural,
		erdVersion,
		resourceID,
	)

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, u)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return handleERDStatusNotFound(respBody)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}
