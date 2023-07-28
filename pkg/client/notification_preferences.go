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

// NotificationPreferences list all notification preferences for a user
func (c *Client) NotificationPreferences(ctx context.Context, userID string) (v1alpha1.UserNotificationPreferences, error) {
	u := fmt.Sprintf(
		"%s/api/%s/users/%s/notification-preferences",
		c.url,
		governorAPIVersionAlpha,
		userID,
	)

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

	p := v1alpha1.UserNotificationPreferences{}
	if err := json.Unmarshal(respBody, &p); err != nil {
		return nil, err
	}

	return p, nil
}

// UpdateNotificationPreferences updates notification preferences for a user
func (c *Client) UpdateNotificationPreferences(
	ctx context.Context, userID string, p v1alpha1.UserNotificationPreferences,
) (v1alpha1.UserNotificationPreferences, error) {
	if userID == "" {
		return nil, ErrMissingUserID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodPut,
		fmt.Sprintf(
			"%s/api/%s/notification-preferences/%s",
			c.url,
			governorAPIVersionAlpha,
			userID,
		),
	)
	if err != nil {
		return nil, err
	}

	ntReqJSON, err := json.Marshal(p)
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

	preferences := v1alpha1.UserNotificationPreferences{}
	if err := json.Unmarshal(respBody, &preferences); err != nil {
		return nil, err
	}

	return preferences, nil
}
