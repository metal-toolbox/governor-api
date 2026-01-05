package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

// Get User
type GetUserInput struct {
	UserID string `json:"user_id" jsonschema:"the unique identifier of the user"`
}

func (s *GovernorMCPServer) GetUser(ctx context.Context, req *mcp.CallToolRequest, args GetUserInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.GetUser")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	govclient, err := s.newGovernorClient(rawToken)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Debug("getting user", zap.String("user_id", args.UserID))
	span.SetAttributes(
		attribute.String("user-id", args.UserID),
	)

	user, err := govclient.User(ctx, args.UserID, false)
	if err != nil {
		span.SetStatus(codes.Error, "failed to get user")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, user, nil
}

type ListUsersInput struct {
	Deleted bool `json:"deleted" jsonschema:"whether to include deleted users"`
}

// List Users
func (s *GovernorMCPServer) ListUsers(ctx context.Context, req *mcp.CallToolRequest, args ListUsersInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.ListUsers")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	govclient, err := s.newGovernorClient(rawToken)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Debug("listing users")

	users, err := govclient.Users(ctx, args.Deleted)
	if err != nil {
		span.SetStatus(codes.Error, "failed to list users")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, users, nil
}
