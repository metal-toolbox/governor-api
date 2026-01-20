package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func (s *GovernorMCPServer) CurrentUserInfo(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.CurrentUserInfo")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/user", s.govURL)

	s.logger.Debug("getting current user", zap.String("url", u))
	span.SetAttributes(
		attribute.String("governor-url", u),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	user := &v1alpha1.AuthenticatedUser{}
	if err := json.NewDecoder(resp.Body).Decode(user); err != nil {
		return nil, nil, err
	}

	return nil, user, nil
}

func (s *GovernorMCPServer) CurrentUserGroups(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.CurrentUserGroups")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/user/groups", s.govURL)

	s.logger.Debug("getting current user groups", zap.String("url", u))
	span.SetAttributes(
		attribute.String("governor-url", u),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	groups := &[]v1alpha1.AuthenticatedUserGroup{}
	if err := json.NewDecoder(resp.Body).Decode(groups); err != nil {
		return nil, nil, err
	}

	return nil, groups, nil
}

func (s *GovernorMCPServer) CurrentUserGroupRequests(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.CurrentUserGroupRequests")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/user/groups/requests", s.govURL)

	s.logger.Debug("getting current user group requests", zap.String("url", u))
	span.SetAttributes(
		attribute.String("governor-url", u),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	requests := &v1alpha1.AuthenticatedUserRequests{}
	if err := json.NewDecoder(resp.Body).Decode(requests); err != nil {
		return nil, nil, err
	}

	return nil, requests, nil
}

func (s *GovernorMCPServer) CurrentUserGroupApprovals(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.CurrentUserGroupApprovals")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/user/groups/approvals", s.govURL)

	s.logger.Debug("getting current user group approvals", zap.String("url", u))
	span.SetAttributes(
		attribute.String("governor-url", u),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	approvals := &v1alpha1.AuthenticatedUserRequests{}
	if err := json.NewDecoder(resp.Body).Decode(approvals); err != nil {
		return nil, nil, err
	}

	return nil, approvals, nil
}

type RemoveUserGroupInput struct {
	GroupID string `json:"group_id" jsonschema:"the unique identifier of the group to remove user from"`
}

func (s *GovernorMCPServer) RemoveAuthenticatedUserGroup(ctx context.Context, req *mcp.CallToolRequest, args RemoveUserGroupInput) (*mcp.CallToolResult, *SimpleGroupOperationResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.RemoveAuthenticatedUserGroup")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/user/groups/%s", s.govURL, args.GroupID)

	s.logger.Debug("removing current user from group", zap.String("url", u), zap.String("group_id", args.GroupID))
	span.SetAttributes(
		attribute.String("governor-url", u),
		attribute.String("group-id", args.GroupID),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	return nil, &SimpleGroupOperationResult{Status: "success", GroupID: args.GroupID}, nil
}
