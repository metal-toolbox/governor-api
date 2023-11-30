package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// UserExtensionResource fetches a user extension resource
func (c *Client) UserExtensionResource(
	ctx context.Context, userID, extensionSlug, erdSlugPlural, erdVersion, resourceID string,
	deleted bool,
) (*v1alpha1.UserExtensionResource, error) {
	if userID == "" {
		return nil, ErrMissingUserID
	}

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
		"%s/api/%s/users/%s/extension-resources/%s/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		userID,
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

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, ErrRequestNonSuccess
	}

	uer := &v1alpha1.UserExtensionResource{}
	if err := json.Unmarshal(respBody, uer); err != nil {
		return nil, err
	}

	return uer, nil
}

// UserExtensionResources lists all user extension resources for a user
func (c *Client) UserExtensionResources(
	ctx context.Context, userID, extensionSlug, erdSlugPlural, erdVersion string,
	deleted bool,
) ([]*v1alpha1.UserExtensionResource, error) {
	if userID == "" {
		return nil, ErrMissingUserID
	}

	if extensionSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	if erdSlugPlural == "" {
		return nil, ErrMissingERDIDOrSlug
	}

	u := fmt.Sprintf(
		"%s/api/%s/users/%s/extension-resources/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		userID,
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

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleResourceStatusNotFound(respBody)
	}

	uer := []*v1alpha1.UserExtensionResource{}
	if err := json.Unmarshal(respBody, &uer); err != nil {
		return nil, err
	}

	return uer, nil
}

// CreateUserExtensionResource creates a user extension resource
func (c *Client) CreateUserExtensionResource(
	ctx context.Context, userID, extensionSlug, erdSlugPlural, erdVersion string,
	resource interface{}, reqOpts ...RequestOption,
) (*v1alpha1.UserExtensionResource, error) {
	if userID == "" {
		return nil, ErrMissingUserID
	}

	if extensionSlug == "" {
		return nil, ErrMissingExtensionIDOrSlug
	}

	if erdSlugPlural == "" {
		return nil, ErrMissingERDIDOrSlug
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPost,
		fmt.Sprintf(
			"%s/api/%s/users/%s/extension-resources/%s/%s/%s",
			c.url,
			governorAPIVersionAlpha,
			userID,
			extensionSlug,
			erdSlugPlural,
			erdVersion,
		),
	)
	if err != nil {
		return nil, err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	for _, opt := range reqOpts {
		opt(req)
	}

	reqBody, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(reqBody))

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

	uer := &v1alpha1.UserExtensionResource{}
	if err := json.Unmarshal(respBody, uer); err != nil {
		return nil, err
	}

	return uer, nil
}

// UpdateUserExtensionResource updates a user extension resource
func (c *Client) UpdateUserExtensionResource(
	ctx context.Context, userID, extensionSlug, erdSlugPlural, erdVersion, resourceID string,
	resource interface{}, reqOpts ...RequestOption,
) (*v1alpha1.UserExtensionResource, error) {
	if userID == "" {
		return nil, ErrMissingUserID
	}

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
		"%s/api/%s/users/%s/extension-resources/%s/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		userID,
		extensionSlug,
		erdSlugPlural,
		erdVersion,
		resourceID,
	)

	req, err := c.newGovernorRequest(ctx, http.MethodPatch, u)
	if err != nil {
		return nil, err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	for _, opt := range reqOpts {
		opt(req)
	}

	reqBody, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(reqBody))

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

	uer := &v1alpha1.UserExtensionResource{}
	if err := json.Unmarshal(respBody, uer); err != nil {
		return nil, err
	}

	return uer, nil
}

// DeleteUserExtensionResource deletes a user extension resource
func (c *Client) DeleteUserExtensionResource(
	ctx context.Context, userID, extensionSlug, erdSlugPlural, erdVersion, resourceID string,
	reqOpts ...RequestOption,
) error {
	if userID == "" {
		return ErrMissingUserID
	}

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
		"%s/api/%s/users/%s/extension-resources/%s/%s/%s/%s",
		c.url,
		governorAPIVersionAlpha,
		userID,
		extensionSlug,
		erdSlugPlural,
		erdVersion,
		resourceID,
	)

	req, err := c.newGovernorRequest(ctx, http.MethodDelete, u)
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return handleResourceStatusNotFound(respBody)
	}

	return nil
}
