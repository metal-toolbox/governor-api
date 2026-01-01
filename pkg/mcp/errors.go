package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/metal-toolbox/governor-api/pkg/client"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	// ErrInvalidTokenClaims is returned when the token claims are invalid
	ErrInvalidTokenClaims = errors.New("invalid token claims")
	// ErrTokenExpired is returned when the token is expired
	ErrTokenExpired = errors.New("token is expired")
	// ErrNoTokenFound is returned when no token is found in the request
	ErrNoTokenFound = errors.New("no token found in the request")
)

func (s *GovernorMCPServer) handleHTTPError(ctx context.Context, resp *http.Response) error {
	span := trace.SpanFromContext(ctx)

	msg := ""

	respbody, err := io.ReadAll(resp.Body)
	if err == nil {
		msg = string(respbody)
	}

	if msg == "" {
		msg = resp.Status
	}

	err = fmt.Errorf("%w: %s", client.ErrRequestNonSuccess, msg)

	span.SetStatus(codes.Error, msg)
	span.RecordError(err)

	return err
}
