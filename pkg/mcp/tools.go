package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *GovernorMCPServer) v1alpha1() *mcp.StreamableHTTPHandler {
	v1alpha1 := mcp.NewServer(&mcp.Implementation{Name: "governor-v1alpha1"}, nil)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "current-user-info", Description: "Get current user information"},
		s.CurrentUserInfo,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "current-user-groups", Description: "Get current user group memberships"},
		s.CurrentUserGroups,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "current-user-group-requests", Description: "Get current user group membership and application requests"},
		s.CurrentUserGroupRequests,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "current-user-group-approvals", Description: "Get group membership and application requests that the current user can approve"},
		s.CurrentUserGroupApprovals,
	)

	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return v1alpha1
	}, nil)
}
