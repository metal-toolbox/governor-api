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

// NotificationType fetch a notification type
func (c *Client) NotificationType(ctx context.Context, idOrSlug string, deleted bool) (*v1alpha1.NotificationType, error) {
	if idOrSlug == "" {
		return nil, ErrMissingNotificationTypeID
	}

	u := fmt.Sprintf(
		"%s/api/%s/notification-types/%s",
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

	nt := &v1alpha1.NotificationType{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// NotificationTypes list all notification types
func (c *Client) NotificationTypes(ctx context.Context, deleted bool) ([]*v1alpha1.NotificationType, error) {
	u := fmt.Sprintf(
		"%s/api/%s/notification-types",
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

	nt := []*v1alpha1.NotificationType{}
	if err := json.Unmarshal(respBody, &nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// CreateNotificationType creates a notification type
func (c *Client) CreateNotificationType(
	ctx context.Context, ntReq *v1alpha1.NotificationTypeReq,
) (*v1alpha1.NotificationType, error) {
	req, err := c.newGovernorRequest(
		ctx, http.MethodPost,
		fmt.Sprintf("%s/api/%s/notification-types", c.url, governorAPIVersionAlpha),
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

	nt := &v1alpha1.NotificationType{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// UpdateNotificationType updates a notification type
func (c *Client) UpdateNotificationType(
	ctx context.Context, idOrSlug string, ntReq *v1alpha1.NotificationTypeReq,
) (*v1alpha1.NotificationType, error) {
	if idOrSlug == "" {
		return nil, ErrMissingNotificationTypeID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPut,
		fmt.Sprintf(
			"%s/api/%s/notification-types/%s",
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

	nt := &v1alpha1.NotificationType{}
	if err := json.Unmarshal(respBody, nt); err != nil {
		return nil, err
	}

	return nt, nil
}

// DeleteNotificationType deletes a notification type
func (c *Client) DeleteNotificationType(ctx context.Context, idOrSlug string) error {
	if idOrSlug == "" {
		return ErrMissingNotificationTypeID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodDelete,
		fmt.Sprintf(
			"%s/api/%s/notification-types/%s",
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
