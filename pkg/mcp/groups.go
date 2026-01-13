package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type SearchGroupsOutput struct {
	Groups []ListGroupsGroupInfo `json:"groups" jsonschema:"list of groups matching the search query"`
}

type ListGroupInput struct {
	Deleted bool `json:"deleted,omitempty" jsonschema:"whether to include deleted groups"`
}

type ListGroupsGroupInfo struct {
	ID        string     `json:"id" jsonschema:"the unique identifier of the group"`
	Name      string     `json:"name" jsonschema:"the name of the group"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" jsonschema:"the time the group was deleted, if applicable"`
}

type ListGroupsOutput struct {
	Groups []ListGroupsGroupInfo `json:"groups" jsonschema:"list of all groups"`
}

// SimpleGroupOperationResult is a generic result for simple group operations
type SimpleGroupOperationResult struct {
	Status  string `json:"status" jsonschema:"operation status (success/error)"`
	GroupID string `json:"group_id" jsonschema:"unique identifier of the group"`
}

type GroupRequestResult struct {
	Status    string `json:"status" jsonschema:"operation status (success/error)"`
	GroupID   string `json:"group_id" jsonschema:"unique identifier of the group"`
	RequestID string `json:"request_id" jsonschema:"unique identifier of the request"`
}

type GroupMemberResult struct {
	Status  string `json:"status" jsonschema:"operation status (success/error)"`
	GroupID string `json:"group_id" jsonschema:"unique identifier of the group"`
	UserID  string `json:"user_id" jsonschema:"unique identifier of the user"`
	IsAdmin bool   `json:"is_admin" jsonschema:"whether the user has admin privileges in the group"`
}

type UserGroupResult struct {
	Status  string `json:"status" jsonschema:"operation status (success/error)"`
	GroupID string `json:"group_id" jsonschema:"unique identifier of the group"`
	UserID  string `json:"user_id" jsonschema:"unique identifier of the user"`
}

type GroupHierarchyResult struct {
	Status        string `json:"status" jsonschema:"operation status (success/error)"`
	GroupID       string `json:"group_id" jsonschema:"unique identifier of the parent group"`
	MemberGroupID string `json:"member_group_id" jsonschema:"unique identifier of the member group"`
}

func (s *GovernorMCPServer) ListGroups(ctx context.Context, req *mcp.CallToolRequest, args ListGroupInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.ListGroups")
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

	groups, err := govclient.Groups(ctx)
	if err != nil {
		span.SetStatus(codes.Error, "failed to get groups from governor")
		span.RecordError(err)

		return nil, nil, err
	}

	resp := &ListGroupsOutput{}

	for _, g := range groups {
		info := ListGroupsGroupInfo{
			ID:   g.ID,
			Name: g.Name,
		}

		if g.DeletedAt.Valid {
			info.DeletedAt = &g.DeletedAt.Time
		}

		resp.Groups = append(resp.Groups, info)
	}

	return nil, resp, nil
}

type SearchGroupsInput struct {
	Query   string `json:"query" jsonschema:"the search query string"`
	Deleted bool   `json:"deleted,omitempty" jsonschema:"whether to include deleted groups"`
}

func (s *GovernorMCPServer) SearchGroups(ctx context.Context, req *mcp.CallToolRequest, args SearchGroupsInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.SearchGroups")
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

	allgroups, err := govclient.Groups(ctx)
	if err != nil {
		span.SetStatus(codes.Error, "failed to get groups from governor")
		span.RecordError(err)

		return nil, nil, err
	}

	resp := &SearchGroupsOutput{}

	for _, g := range allgroups {
		q := strings.ToLower(args.Query)
		slug := strings.ToLower(g.Slug)
		name := strings.ToLower(g.Name)
		description := strings.ToLower(g.Description)

		if strings.Contains(name, q) || strings.Contains(slug, q) || strings.Contains(description, q) {
			resp.Groups = append(resp.Groups, ListGroupsGroupInfo{
				ID:   g.ID,
				Name: g.Name,
			})
		}
	}

	return nil, resp, nil
}

type GetGroupInput struct {
	GroupID string `json:"group_id" jsonschema:"the unique identifier of the group"`
	Deleted bool   `json:"deleted,omitempty" jsonschema:"whether to include deleted groups"`
}

func (s *GovernorMCPServer) GetGroup(ctx context.Context, req *mcp.CallToolRequest, args GetGroupInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.GetGroup")
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

	group, err := govclient.Group(ctx, args.GroupID, args.Deleted)
	if err != nil {
		span.SetStatus(codes.Error, "failed to get group from governor")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, group, nil
}

// Create Group
type CreateGroupInput struct {
	Name        string `json:"name" jsonschema:"the name of the group"`
	Description string `json:"description" jsonschema:"the description of the group"`
	Note        string `json:"note,omitempty" jsonschema:"optional note for the group creation"`
}

func (s *GovernorMCPServer) CreateGroup(ctx context.Context, req *mcp.CallToolRequest, args CreateGroupInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.CreateGroup")
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

	groupReq := &v1alpha1.GroupReq{
		Name:        args.Name,
		Description: args.Description,
		Note:        args.Note,
	}

	s.logger.Debug("creating group", zap.String("name", args.Name))
	span.SetAttributes(
		attribute.String("group-name", args.Name),
	)

	group, err := govclient.CreateGroup(ctx, groupReq)
	if err != nil {
		span.SetStatus(codes.Error, "failed to create group")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, group, nil
}

// Get All Group Requests
func (s *GovernorMCPServer) GetGroupRequestsAll(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.GetGroupRequestsAll")
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

	s.logger.Debug("getting all group requests")

	requests, err := govclient.GroupMembershipRequestsAll(ctx, false)
	if err != nil {
		span.SetStatus(codes.Error, "failed to get all group requests")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, requests, nil
}

// Delete Group
type DeleteGroupInput struct {
	GroupID string `json:"group_id" jsonschema:"the unique identifier of the group"`
}

func (s *GovernorMCPServer) DeleteGroup(ctx context.Context, req *mcp.CallToolRequest, args DeleteGroupInput) (*mcp.CallToolResult, *SimpleGroupOperationResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.DeleteGroup")
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

	s.logger.Debug("deleting group", zap.String("group_id", args.GroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
	)

	err = govclient.DeleteGroup(ctx, args.GroupID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to delete group")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &SimpleGroupOperationResult{Status: "success", GroupID: args.GroupID}, nil
}

// Create Group Request
type CreateGroupRequestInput struct {
	GroupID        string     `json:"group_id" jsonschema:"the unique identifier of the group"`
	Note           string     `json:"note" jsonschema:"optional note for the request"`
	Kind           string     `json:"kind,omitempty" jsonschema:"optional kind of request, there are two kinds: 'new_member' and 'admin_promotion'"`
	IsAdmin        bool       `json:"is_admin" jsonschema:"whether requesting admin privileges"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty" jsonschema:"optional expiration time for admin privileges"`
	AdminExpiresAt *time.Time `json:"admin_expires_at,omitempty" jsonschema:"optional expiration time for admin privileges"`
}

func (s *GovernorMCPServer) CreateGroupRequest(ctx context.Context, req *mcp.CallToolRequest, args CreateGroupRequestInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.CreateGroupRequest")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/groups/%s/requests", s.govURL, args.GroupID)

	jsonPayload, err := json.Marshal(args)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Debug("creating group request", zap.String("url", u), zap.String("group_id", args.GroupID))
	span.SetAttributes(
		attribute.String("governor-url", u),
		attribute.String("group-id", args.GroupID),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)
	govreq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Request to join group created successfully, a group admin has been notified.",
			},
		},
	}, nil, nil
}

// Get Group Requests
func (s *GovernorMCPServer) GetGroupRequests(ctx context.Context, req *mcp.CallToolRequest, args GetGroupInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.GetGroupRequests")
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

	s.logger.Debug("getting group requests", zap.String("group_id", args.GroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
	)

	requests, err := govclient.GroupMemberRequests(ctx, args.GroupID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to get group requests")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, requests, nil
}

// Process Group Request
type ProcessGroupRequestInput struct {
	GroupID   string `json:"group_id" jsonschema:"the unique identifier of the group"`
	RequestID string `json:"request_id" jsonschema:"the unique identifier of the request"`
	Approve   bool   `json:"approve" jsonschema:"whether to approve or deny the request"`
	Note      string `json:"note,omitempty" jsonschema:"optional note for the decision"`
}

func (s *GovernorMCPServer) ProcessGroupRequest(ctx context.Context, req *mcp.CallToolRequest, args ProcessGroupRequestInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.ProcessGroupRequest")
	defer span.End()

	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	u := fmt.Sprintf("%s/api/v1alpha1/groups/%s/requests/%s", s.govURL, args.GroupID, args.RequestID)

	payload := map[string]interface{}{
		"approve": args.Approve,
	}
	if args.Note != "" {
		payload["note"] = args.Note
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Debug("processing group request", zap.String("url", u), zap.String("group_id", args.GroupID), zap.String("request_id", args.RequestID))
	span.SetAttributes(
		attribute.String("governor-url", u),
		attribute.String("group-id", args.GroupID),
		attribute.String("request-id", args.RequestID),
	)

	govreq, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, nil, err
	}

	govreq.Header.Set("Authorization", "Bearer "+rawToken)
	govreq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpclient.Do(govreq)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := s.handleHTTPError(ctx, resp)
		return nil, nil, err
	}

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	return nil, result, nil
}

// Delete Group Request
type DeleteGroupRequestInput struct {
	GroupID   string `json:"group_id" jsonschema:"the unique identifier of the group"`
	RequestID string `json:"request_id" jsonschema:"the unique identifier of the request"`
}

func (s *GovernorMCPServer) DeleteGroupRequest(ctx context.Context, req *mcp.CallToolRequest, args DeleteGroupRequestInput) (*mcp.CallToolResult, *GroupRequestResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.DeleteGroupRequest")
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

	s.logger.Debug("deleting group request", zap.String("group_id", args.GroupID), zap.String("request_id", args.RequestID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
		attribute.String("request-id", args.RequestID),
	)

	err = govclient.RemoveGroupMembershipRequest(ctx, args.GroupID, args.RequestID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to delete group request")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &GroupRequestResult{Status: "success", GroupID: args.GroupID, RequestID: args.RequestID}, nil
}

// List Group Members
func (s *GovernorMCPServer) ListGroupMembers(ctx context.Context, req *mcp.CallToolRequest, args GetGroupInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.ListGroupMembers")
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

	s.logger.Debug("listing group members", zap.String("group_id", args.GroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
	)

	members, err := govclient.GroupMembers(ctx, args.GroupID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to list group members")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, members, nil
}

// Add Group Member
type AddGroupMemberInput struct {
	GroupID string `json:"group_id" jsonschema:"the unique identifier of the group"`
	UserID  string `json:"user_id" jsonschema:"the unique identifier of the user"`
	IsAdmin bool   `json:"is_admin,omitempty" jsonschema:"whether the user should have admin privileges"`
}

func (s *GovernorMCPServer) AddGroupMember(ctx context.Context, req *mcp.CallToolRequest, args AddGroupMemberInput) (*mcp.CallToolResult, *GroupMemberResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.AddGroupMember")
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

	s.logger.Debug("adding group member", zap.String("group_id", args.GroupID), zap.String("user_id", args.UserID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
		attribute.String("user-id", args.UserID),
	)

	err = govclient.AddGroupMember(ctx, args.GroupID, args.UserID, args.IsAdmin)
	if err != nil {
		span.SetStatus(codes.Error, "failed to add group member")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &GroupMemberResult{Status: "success", GroupID: args.GroupID, UserID: args.UserID, IsAdmin: args.IsAdmin}, nil
}

// Remove Group Member
type RemoveGroupMemberInput struct {
	GroupID string `json:"group_id" jsonschema:"the unique identifier of the group"`
	UserID  string `json:"user_id" jsonschema:"the unique identifier of the user"`
}

func (s *GovernorMCPServer) RemoveGroupMember(ctx context.Context, req *mcp.CallToolRequest, args RemoveGroupMemberInput) (*mcp.CallToolResult, *UserGroupResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.RemoveGroupMember")
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

	s.logger.Debug("removing group member", zap.String("group_id", args.GroupID), zap.String("user_id", args.UserID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
		attribute.String("user-id", args.UserID),
	)

	err = govclient.RemoveGroupMember(ctx, args.GroupID, args.UserID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to remove group member")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &UserGroupResult{Status: "success", GroupID: args.GroupID, UserID: args.UserID}, nil
}

// List Member Groups (Group Hierarchies)
func (s *GovernorMCPServer) ListMemberGroups(ctx context.Context, req *mcp.CallToolRequest, args GetGroupInput) (*mcp.CallToolResult, any, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.ListMemberGroups")
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

	s.logger.Debug("listing member groups", zap.String("group_id", args.GroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
	)

	hierarchies, err := govclient.MemberGroups(ctx, args.GroupID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to list member groups")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, hierarchies, nil
}

// Add Member Group
type AddMemberGroupInput struct {
	GroupID       string `json:"group_id" jsonschema:"the unique identifier of the parent group"`
	MemberGroupID string `json:"member_group_id" jsonschema:"the unique identifier of the member group"`
}

func (s *GovernorMCPServer) AddMemberGroup(ctx context.Context, req *mcp.CallToolRequest, args AddMemberGroupInput) (*mcp.CallToolResult, *GroupHierarchyResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.AddMemberGroup")
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

	s.logger.Debug("adding member group", zap.String("group_id", args.GroupID), zap.String("member_group_id", args.MemberGroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
		attribute.String("member-group-id", args.MemberGroupID),
	)

	// Use null.Time{} for no expiration
	err = govclient.AddMemberGroup(ctx, args.GroupID, args.MemberGroupID, null.Time{})
	if err != nil {
		span.SetStatus(codes.Error, "failed to add member group")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &GroupHierarchyResult{Status: "success", GroupID: args.GroupID, MemberGroupID: args.MemberGroupID}, nil
}

// Update Member Group
type UpdateMemberGroupInput struct {
	GroupID       string `json:"group_id" jsonschema:"the unique identifier of the parent group"`
	MemberGroupID string `json:"member_group_id" jsonschema:"the unique identifier of the member group"`
	// Add other fields as needed based on what can be updated
}

func (s *GovernorMCPServer) UpdateMemberGroup(ctx context.Context, req *mcp.CallToolRequest, args UpdateMemberGroupInput) (*mcp.CallToolResult, *GroupHierarchyResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.UpdateMemberGroup")
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

	s.logger.Debug("updating member group", zap.String("group_id", args.GroupID), zap.String("member_group_id", args.MemberGroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
		attribute.String("member-group-id", args.MemberGroupID),
	)

	// Use null.Time{} for no expiration
	err = govclient.UpdateMemberGroup(ctx, args.GroupID, args.MemberGroupID, null.Time{})
	if err != nil {
		span.SetStatus(codes.Error, "failed to update member group")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &GroupHierarchyResult{Status: "success", GroupID: args.GroupID, MemberGroupID: args.MemberGroupID}, nil
}

// Remove Member Group
type RemoveMemberGroupInput struct {
	GroupID       string `json:"group_id" jsonschema:"the unique identifier of the parent group"`
	MemberGroupID string `json:"member_group_id" jsonschema:"the unique identifier of the member group"`
}

func (s *GovernorMCPServer) RemoveMemberGroup(ctx context.Context, req *mcp.CallToolRequest, args RemoveMemberGroupInput) (*mcp.CallToolResult, *GroupHierarchyResult, error) {
	ctx, span := s.tracer.Start(ctx, "GovernorMCPServer.RemoveMemberGroup")
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

	s.logger.Debug("removing member group", zap.String("group_id", args.GroupID), zap.String("member_group_id", args.MemberGroupID))
	span.SetAttributes(
		attribute.String("group-id", args.GroupID),
		attribute.String("member-group-id", args.MemberGroupID),
	)

	err = govclient.DeleteMemberGroup(ctx, args.GroupID, args.MemberGroupID)
	if err != nil {
		span.SetStatus(codes.Error, "failed to remove member group")
		span.RecordError(err)

		return nil, nil, err
	}

	return nil, &GroupHierarchyResult{Status: "success", GroupID: args.GroupID, MemberGroupID: args.MemberGroupID}, nil
}
