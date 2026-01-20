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

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "list-groups", Description: "List all groups (warning: can return very large arrays, prefer search-groups)"},
		s.ListGroups,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "search-groups", Description: "Search groups by name, slug, or description (preferred over list-groups)"},
		s.SearchGroups,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "get-group", Description: "Get detailed information about a specific group by ID"},
		s.GetGroup,
	)

	// Authenticated User Tools
	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "remove-authenticated-user-group", Description: "Remove the current user from a group"},
		s.RemoveAuthenticatedUserGroup,
	)

	// Group Management Tools
	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "create-group", Description: "Create a new group with name and description"},
		s.CreateGroup,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "get-group-requests-all", Description: "Get all group requests across the system (admin access required)"},
		s.GetGroupRequestsAll,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "delete-group", Description: "Delete a group"},
		s.DeleteGroup,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "create-group-request", Description: "Create a request to join a group"},
		s.CreateGroupRequest,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "get-group-requests", Description: "Get requests for a specific group"},
		s.GetGroupRequests,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "process-group-request", Description: "Approve or deny a group membership request"},
		s.ProcessGroupRequest,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "delete-group-request", Description: "Delete a group membership request"},
		s.DeleteGroupRequest,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "list-group-members", Description: "List all members of a specific group"},
		s.ListGroupMembers,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "add-group-member", Description: "Add a user to a group"},
		s.AddGroupMember,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "remove-group-member", Description: "Remove a user from a group"},
		s.RemoveGroupMember,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "list-member-groups", Description: "List child groups (group hierarchies) for a group"},
		s.ListMemberGroups,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "add-member-group", Description: "Add a child group to a parent group (group hierarchy)"},
		s.AddMemberGroup,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "update-member-group", Description: "Update a group hierarchy relationship"},
		s.UpdateMemberGroup,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "remove-member-group", Description: "Remove a child group from a parent group"},
		s.RemoveMemberGroup,
	)

	// User Management Tools
	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "get-user", Description: "Get details of a specific user by ID"},
		s.GetUser,
	)

	mcp.AddTool(
		v1alpha1,
		&mcp.Tool{Name: "list-users", Description: "List all users in the system"},
		s.ListUsers,
	)

	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return v1alpha1
	}, nil)
}
