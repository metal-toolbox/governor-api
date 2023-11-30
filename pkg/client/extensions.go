package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Extension fetch an extension
func (c *Client) Extension(ctx context.Context, idOrSlug string, deleted bool) (*v1alpha1.Extension, error) {
	if idOrSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	u := fmt.Sprintf(
		"%s/api/%s/extensions/%s",
		c.url,
		governorAPIVersionAlpha,
		idOrSlug,
	)
	if deleted {
		u += "?deleted"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

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

	nt := &v1alpha1.Extension{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// Extensions list all extensions
func (c *Client) Extensions(ctx context.Context, deleted bool) ([]*v1alpha1.Extension, error) {
	u := fmt.Sprintf(
		"%s/api/%s/extensions",
		c.url,
		governorAPIVersionAlpha,
	)
	if deleted {
		u += "?deleted"
	}

	req, err := c.newGovernorRequest(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	nt := []*v1alpha1.Extension{}
	if err := json.Unmarshal(respBody, &nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// CreateExtension creates an extension
func (c *Client) CreateExtension(
	ctx context.Context, exReq *v1alpha1.ExtensionReq,
	reqOpts ...RequestOption,
) (*v1alpha1.Extension, error) {
	req, err := c.newGovernorRequest(
		ctx, http.MethodPost,
		fmt.Sprintf("%s/api/%s/extensions", c.url, governorAPIVersionAlpha),
	)
	if err != nil {
		return nil, err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	for _, opt := range reqOpts {
		opt(req)
	}

	exReqJSON, err := json.Marshal(exReq)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(exReqJSON))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return nil, ErrRequestNonSuccess
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	nt := &v1alpha1.Extension{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// UpdateExtension updates an extension
func (c *Client) UpdateExtension(
	ctx context.Context, idOrSlug string, exReq *v1alpha1.ExtensionReq,
	reqOpts ...RequestOption,
) (*v1alpha1.Extension, error) {
	if idOrSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPatch,
		fmt.Sprintf(
			"%s/api/%s/extensions/%s",
			c.url,
			governorAPIVersionAlpha,
			idOrSlug,
		),
	)
	if err != nil {
		return nil, err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	for _, opt := range reqOpts {
		opt(req)
	}

	exReqJSON, err := json.Marshal(exReq)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(exReqJSON))

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

	nt := &v1alpha1.Extension{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// DeleteExtension deletes an extension
func (c *Client) DeleteExtension(ctx context.Context, idOrSlug string, reqOpts ...RequestOption) error {
	if idOrSlug == "" {
		return ErrMissingExtensionIDOrSlug
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodDelete,
		fmt.Sprintf(
			"%s/api/%s/extensions/%s",
			c.url,
			governorAPIVersionAlpha,
			idOrSlug,
		),
	)
	if err != nil {
		return err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	for _, opt := range reqOpts {
		opt(req)
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
