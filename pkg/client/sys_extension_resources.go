package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

// handleResourceStatusNotFound handles a 404 responses
func handleResourceStatusNotFound(respBody []byte) error {
	respErr := map[string]string{}
	if err := json.Unmarshal(respBody, &respErr); err != nil {
		return ErrRequestNonSuccess
	}

	errMsg, ok := respErr["error"]
	if !ok {
		return ErrRequestNonSuccess
	}

	switch {
	case strings.Contains(errMsg, v1alpha1.ErrERDNotFound.Error()):
		return v1alpha1.ErrERDNotFound
	case strings.Contains(errMsg, v1alpha1.ErrExtensionNotFound.Error()):
		return v1alpha1.ErrExtensionNotFound
	case strings.Contains(errMsg, v1alpha1.ErrExtensionResourceNotFound.Error()):
		return v1alpha1.ErrExtensionResourceNotFound
	case strings.Contains(errMsg, v1alpha1.ErrUserNotFound.Error()):
		return v1alpha1.ErrUserNotFound
	default:
		return ErrRequestNonSuccess
	}
}

// SystemExtensionResource fetches a system extension resource
func (c *Client) SystemExtensionResource(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion, resourceID string, deleted bool,
) (*v1alpha1.ExtensionResource, error) {
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
		return nil, handleResourceStatusNotFound(respBody)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	ser := &v1alpha1.ExtensionResource{}
	if err := json.Unmarshal(respBody, ser); err != nil {
		return nil, err
	}

	return ser, nil
}

// SystemExtensionResources list all system resources
func (c *Client) SystemExtensionResources(
	ctx context.Context, extensionSlug, erdSlugPlural, erdVersion string,
	deleted bool, queries map[string]string,
) ([]*v1alpha1.ExtensionResource, error) {
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
		queries["deleted"] = ""
	}

	i := 0
	for k, v := range queries {
		if i == 0 {
			u += "?"
		} else {
			u += "&"
		}

		if v == "" {
			u += k
		} else {
			u += fmt.Sprintf("%s=%s", k, v)
		}

		i++
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
		return nil, handleResourceStatusNotFound(respBody)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	sers := []*v1alpha1.ExtensionResource{}
	if err := json.Unmarshal(respBody, &sers); err != nil {
		return nil, err
	}

	return sers, nil
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
		return handleResourceStatusNotFound(respBody)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}
