package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
)

// PermanentlyDeleteUserRecords permanently deletes all records of a user that has been soft deleted
// this includes anonymizing the user record to remove PII data
// and deleting all audit logs associated with the user
func (c *Client) PermanentlyDeleteUserRecords(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrMissingUserID
	}

	req, err := c.newGovernorRequest(
		ctx, http.MethodDelete,
		fmt.Sprintf("%s/api/%s/retentions/users/%s", c.url, governorAPIVersionAlpha, userID),
	)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	respbody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrUserNotFound
	case http.StatusOK, http.StatusAccepted:
		return nil
	}

	if strings.Contains(string(respbody), v1alpha1.ErrRemoveActiveRecord.Error()) {
		return v1alpha1.ErrRemoveActiveRecord
	}

	return ErrRequestNonSuccess
}
