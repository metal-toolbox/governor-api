package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"go.opentelemetry.io/otel/codes"
)

// CreateExtensionResource creates an extension resource
func (c *Client) CreateExtensionResource(
	ctx context.Context, res *v1alpha1.ExtensionResource,
) (*v1alpha1.ExtensionResource, error) {
	ctx, span := tracer.Start(ctx, "CreateExtensionResource")
	defer span.End()

	if res == nil {
		return nil, ErrNilRequest
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPost, fmt.Sprintf(
		"%s/api/%s/extension-resources",
		c.url,
		governorAPIVersionAlpha,
	))
	if err != nil {
		span.SetStatus(codes.Error, "failed to create request")
		span.RecordError(err)

		return nil, err
	}

	reqBody, err := json.Marshal(res)
	if err != nil {
		span.SetStatus(codes.Error, "failed to marshal request body")
		span.RecordError(err)

		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		span.SetStatus(codes.Error, "request failed")
		span.RecordError(err)

		return nil, err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		span.SetStatus(codes.Error, "failed to read response body")
		span.RecordError(err)

		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		err := fmt.Errorf("%w: %s", ErrRequestNonSuccess, string(respBody))

		span.SetStatus(codes.Error, "bad request")
		span.RecordError(err)

		return nil, err
	}

	created := &v1alpha1.ExtensionResource{}
	if err := json.Unmarshal(respBody, created); err != nil {
		span.SetStatus(codes.Error, "failed to unmarshal response body")
		span.RecordError(err)

		return nil, err
	}

	return created, nil
}

func (c *Client) UpdateExtensionResource(
	ctx context.Context, resource *v1alpha1.ExtensionResource, id string,
) (*v1alpha1.ExtensionResource, error) {
	ctx, span := tracer.Start(ctx, "UpdateExtensionResource")
	defer span.End()

	if resource == nil {
		return nil, ErrNilRequest
	}

	req, err := c.newGovernorRequest(ctx, http.MethodPatch, fmt.Sprintf(
		"%s/api/%s/extension-resources/%s",
		c.url,
		governorAPIVersionAlpha,
		id,
	))
	if err != nil {
		span.SetStatus(codes.Error, "failed to create request")
		span.RecordError(err)

		return nil, err
	}

	reqBody, err := json.Marshal(resource)
	if err != nil {
		span.SetStatus(codes.Error, "failed to marshal request body")
		span.RecordError(err)

		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		span.SetStatus(codes.Error, "request failed")
		span.RecordError(err)

		return nil, err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		span.SetStatus(codes.Error, "failed to read response body")
		span.RecordError(err)

		return nil, err
	}

	if resp.StatusCode != http.StatusAccepted {
		err := fmt.Errorf("%w: %s", ErrRequestNonSuccess, string(respBody))

		span.SetStatus(codes.Error, "bad request")
		span.RecordError(err)

		return nil, err
	}

	updated := &v1alpha1.ExtensionResource{}
	if err := json.Unmarshal(respBody, updated); err != nil {
		span.SetStatus(codes.Error, "failed to unmarshal response body")
		span.RecordError(err)

		return nil, err
	}

	return updated, nil
}
