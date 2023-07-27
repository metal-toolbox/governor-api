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

// NotificationTarget fetch a notification target
func (c *Client) NotificationTarget(ctx context.Context, idOrSlug string, deleted bool) (*v1alpha1.NotificationTarget, error) {
	if idOrSlug == "" {
		return nil, ErrMissingNotificationTargetID
	}

	u := fmt.Sprintf(
		"%s/api/%s/notification-targets/%s",
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

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrGroupNotFound
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

	nt := &v1alpha1.NotificationTarget{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// NotificationTargets list all notification targets
func (c *Client) NotificationTargets(ctx context.Context, deleted bool) ([]*v1alpha1.NotificationTarget, error) {
	u := fmt.Sprintf(
		"%s/api/%s/notification-targets",
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

	nt := []*v1alpha1.NotificationTarget{}
	if err := json.Unmarshal(respBody, &nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// CreateNotificationTarget creates a notification target
func (c *Client) CreateNotificationTarget(
	ctx context.Context, ntReq *v1alpha1.NotificationTargetReq,
) (*v1alpha1.NotificationTarget, error) {
	req, err := c.newGovernorRequest(
		ctx, http.MethodPost,
		fmt.Sprintf("%s/api/%s/notification-targets", c.url, governorAPIVersionAlpha),
	)
	if err != nil {
		return nil, err
	}

	ntReqJSON, err := json.Marshal(ntReq)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(ntReqJSON))

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

	nt := &v1alpha1.NotificationTarget{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// UpdateNotificationTarget updates a notification target
func (c *Client) UpdateNotificationTarget(
	ctx context.Context, idOrSlug string, ntReq *v1alpha1.NotificationTargetReq,
) (*v1alpha1.NotificationTarget, error) {
	if idOrSlug == "" {
		return nil, ErrMissingNotificationTargetID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPut,
		fmt.Sprintf(
			"%s/api/%s/notification-targets/%s",
			c.url,
			governorAPIVersionAlpha,
			idOrSlug,
		),
	)
	if err != nil {
		return nil, err
	}

	ntReqJSON, err := json.Marshal(ntReq)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(ntReqJSON))

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

	nt := &v1alpha1.NotificationTarget{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// DeleteNotificationTarget deletes a notification target
func (c *Client) DeleteNotificationTarget(ctx context.Context, idOrSlug string) error {
	if idOrSlug == "" {
		return ErrMissingNotificationTargetID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodDelete,
		fmt.Sprintf(
			"%s/api/%s/notification-targets/%s",
			c.url,
			governorAPIVersionAlpha,
			idOrSlug,
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

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusNoContent {
		return ErrRequestNonSuccess
	}

	return nil
}
