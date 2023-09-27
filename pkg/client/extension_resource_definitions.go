package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

func handleERDStatusNotFound(respBody []byte) error {
	respErr := map[string]string{}
	if err := json.Unmarshal(respBody, &respErr); err != nil {
		return err
	}

	if errMsg, ok := respErr["error"]; ok && strings.Contains(errMsg, "extension does not exist") {
		return v1alpha1.ErrExtensionNotFound
	}

	return v1alpha1.ErrERDNotFound
}

// ExtensionResourceDefinition fetches an ERD
func (c *Client) ExtensionResourceDefinition(
	ctx context.Context, extensionIDOrSlug, erdIDOrSlug string, deleted bool,
) (*v1alpha1.ExtensionResourceDefinition, error) {
	if extensionIDOrSlug == "" {
		return nil, ErrMissingExtensionID
	}

	if erdIDOrSlug == "" {
		return nil, ErrMissingERDID
	}

	u := fmt.Sprintf(
		"%s/api/%s/extensions/%s/erds/%s",
		c.url,
		governorAPIVersionAlpha,
		extensionIDOrSlug,
		erdIDOrSlug,
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

	nt := &v1alpha1.ExtensionResourceDefinition{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// ExtensionResourceDefinitions list all ERDs
func (c *Client) ExtensionResourceDefinitions(
	ctx context.Context, extensionIDOrSlug string, deleted bool,
) ([]*v1alpha1.ExtensionResourceDefinition, error) {
	if extensionIDOrSlug == "" {
		return nil, ErrMissingExtensionID
	}

	u := fmt.Sprintf(
		"%s/api/%s/extensions/%s/erds",
		c.url,
		governorAPIVersionAlpha,
		extensionIDOrSlug,
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

	erds := []*v1alpha1.ExtensionResourceDefinition{}
	if err := json.Unmarshal(respBody, &erds); err != nil {
		return nil, err
	}

	return erds, nil
}

// CreateExtensionResourceDefinition creates an ERD
func (c *Client) CreateExtensionResourceDefinition(
	ctx context.Context, extensionIDOrSlug string, erdReq *v1alpha1.ExtensionResourceDefinitionReq,
) (*v1alpha1.ExtensionResourceDefinition, error) {
	if extensionIDOrSlug == "" {
		return nil, ErrMissingExtensionID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPost,
		fmt.Sprintf(
			"%s/api/%s/extensions/%s/erds",
			c.url, governorAPIVersionAlpha, extensionIDOrSlug,
		),
	)
	if err != nil {
		return nil, err
	}

	erdReqJSON, err := json.Marshal(erdReq)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(erdReqJSON))

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

	erd := &v1alpha1.ExtensionResourceDefinition{}
	if err := json.Unmarshal(respBody, erd); err != nil {
		return nil, err
	}

	return erd, nil
}

// UpdateExtensionResourceDefinition updates an ERD
func (c *Client) UpdateExtensionResourceDefinition(
	ctx context.Context, extensionIDOrSlug, erdIDOrSlug string, erdReq *v1alpha1.ExtensionResourceDefinitionReq,
) (*v1alpha1.ExtensionResourceDefinition, error) {
	if extensionIDOrSlug == "" {
		return nil, ErrMissingExtensionID
	}

	if erdIDOrSlug == "" {
		return nil, ErrMissingERDID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPatch,
		fmt.Sprintf(
			"%s/api/%s/extensions/%s/erds/%s",
			c.url,
			governorAPIVersionAlpha,
			extensionIDOrSlug,
			erdIDOrSlug,
		),
	)
	if err != nil {
		return nil, err
	}

	erdReqJSON, err := json.Marshal(erdReq)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(erdReqJSON))

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

	erd := &v1alpha1.ExtensionResourceDefinition{}
	if err := json.Unmarshal(respBody, erd); err != nil {
		return nil, err
	}

	return erd, nil
}

// DeleteExtensionResourceDefinition deletes a extension
func (c *Client) DeleteExtensionResourceDefinition(
	ctx context.Context, extensionIDOrSlug, erdIDOrSlug string,
) error {
	if extensionIDOrSlug == "" {
		return ErrMissingExtensionID
	}

	if erdIDOrSlug == "" {
		return ErrMissingERDID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodDelete,
		fmt.Sprintf(
			"%s/api/%s/extensions/%s/erds/%s",
			c.url,
			governorAPIVersionAlpha,
			extensionIDOrSlug,
			erdIDOrSlug,
		),
	)
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
