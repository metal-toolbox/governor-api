package v1alpha1

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/metal-toolbox/auditevent/ginaudit"
	"go.hollow.sh/toolbox/ginauth"
	"go.hollow.sh/toolbox/ginjwt"

	"github.com/metal-toolbox/governor-api/internal/eventbus"
)

const (
	// Version is the API version constant
	Version = "v1alpha1"
)

// Router is the API router
type Router struct {
	AdminGroups    []string
	AuditLogWriter io.Writer
	AuditMW        *ginaudit.Middleware
	AuthMW         *ginauth.MultiTokenMiddleware
	AuthConf       []ginjwt.AuthConfig
	DB             *sqlx.DB
	EventBus       *eventbus.Client
	Logger         *zap.Logger
}

// Routes sets up protected routes and sets the scopes for said routes
func (r *Router) Routes(rg *gin.RouterGroup) {
	rg.GET(
		"/user",
		r.AuditMW.AuditWithType("GetUser"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.getAuthenticatedUser,
	)

	rg.PUT(
		"/user",
		r.AuditMW.AuditWithType("UpdateUser"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.updateAuthenticatedUser,
	)

	rg.GET(
		"/user/groups",
		r.AuditMW.AuditWithType("GetUserGroups"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.getAuthenticatedUserGroups,
	)

	rg.DELETE(
		"/user/groups/:id",
		r.AuditMW.AuditWithType("RemoveUserGroup"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.removeAuthenticatedUserGroup,
	)

	rg.GET(
		"/user/groups/requests",
		r.AuditMW.AuditWithType("GetUserGroupRequests"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.getAuthenticatedUserGroupRequests,
	)

	rg.GET(
		"/user/groups/approvals",
		r.AuditMW.AuditWithType("GetUserGroupApprovals"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.getAuthenticatedUserGroupApprovals,
	)

	rg.GET(
		"/user/notification-preferences",
		r.AuditMW.AuditWithType("GetUserNotificationPreferences"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.getAuthenticatedUserNotificationPreferences,
	)

	rg.PUT(
		"/user/notification-preferences",
		r.AuditMW.AuditWithType("UpdateUserNotificationPreferences"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.updateAuthenticatedUserNotificationPreferences,
	)

	rg.GET(
		"/users",
		r.AuditMW.AuditWithType("ListUsers"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:users")),
		r.listUsers,
	)

	rg.POST(
		"/users",
		r.AuditMW.AuditWithType("CreateUser"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:users")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createUser,
	)

	rg.GET(
		"/users/:id",
		r.AuditMW.AuditWithType("GetUser"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:users")),
		r.getUser,
	)

	rg.PUT(
		"/users/:id",
		r.AuditMW.AuditWithType("UpdateUser"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:users")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.updateUser,
	)

	rg.DELETE(
		"/users/:id",
		r.AuditMW.AuditWithType("DeleteUser"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:users")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteUser,
	)

	rg.GET(
		"/groups",
		r.AuditMW.AuditWithType("ListGroups"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.listGroups,
	)

	rg.POST(
		"/groups",
		r.AuditMW.AuditWithType("CreateGroup"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:groups")),
		r.mwUserAuthRequired(AuthRoleUser),
		r.createGroup,
	)

	rg.GET(
		"/groups/requests",
		r.AuditMW.AuditWithType("GetGroupRequestsAll"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.getGroupRequestsAll,
	)

	rg.GET(
		"/groups/memberships",
		r.AuditMW.AuditWithType("GetGroupMembersipsAll"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.getGroupMembershipsAll,
	)

	rg.GET(
		"/groups/hierarchies",
		r.AuditMW.AuditWithType("GetGroupHierarchiesAll"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.getGroupHierarchiesAll,
	)

	rg.GET(
		"/groups/:id",
		r.AuditMW.AuditWithType("GetGroup"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.getGroup,
	)

	rg.PUT(
		"/groups/:id",
		r.AuditMW.AuditWithType("UpdateGroup"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.updateGroup,
	)

	rg.DELETE(
		"/groups/:id",
		r.AuditMW.AuditWithType("DeleteGroup"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleAdminOrGroupAdmin),
		r.deleteGroup,
	)

	rg.GET(
		"/groups/:id/events",
		r.AuditMW.AuditWithType("GetGroupEvents"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupMember),
		r.listGroupEvents,
	)

	rg.POST(
		"/groups/:id/requests",
		r.AuditMW.AuditWithType("CreateGroupRequest"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.createGroupRequest,
	)

	rg.GET(
		"/groups/:id/requests",
		r.AuditMW.AuditWithType("GetGroupRequests"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.getGroupRequests,
	)

	rg.PUT(
		"/groups/:id/requests/:rid",
		r.AuditMW.AuditWithType("ProcessGroupRequest"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwGroupAuthRequired(AuthRoleAdminOrGroupAdmin),
		r.processGroupRequest,
	)

	rg.DELETE(
		"/groups/:id/requests/:rid",
		r.AuditMW.AuditWithType("DeleteGroupRequest"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwUserAuthRequired(AuthRoleUser),
		r.deleteGroupRequest,
	)

	rg.GET(
		"/groups/:id/users",
		r.AuditMW.AuditWithType("GetGroupMembers"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.listGroupMembers,
	)

	rg.PUT(
		"/groups/:id/users/:uid",
		r.AuditMW.AuditWithType("AddGroupMember"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.addGroupMember,
	)

	rg.PATCH(
		"/groups/:id/users/:uid",
		r.AuditMW.AuditWithType("UpdateGroupMember"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleAdminOrGroupAdmin),
		r.updateGroupMember,
	)

	rg.DELETE(
		"/groups/:id/users/:uid",
		r.AuditMW.AuditWithType("RemoveGroupMember"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.removeGroupMember,
	)

	rg.PUT(
		"/groups/:id/applications/:oid",
		r.AuditMW.AuditWithType("AddGroupApplication"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.addGroupApplication,
	)

	rg.DELETE(
		"/groups/:id/applications/:oid",
		r.AuditMW.AuditWithType("RemoveGroupApplication"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.removeGroupApplication,
	)

	rg.POST(
		"/groups/:id/apprequests",
		r.AuditMW.AuditWithType("CreateGroupAppRequest"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.createGroupAppRequest,
	)

	rg.GET(
		"/groups/:id/apprequests",
		r.AuditMW.AuditWithType("GetGroupAppRequests"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.getGroupAppRequests,
	)

	rg.PUT(
		"/groups/:id/apprequests/:rid",
		r.AuditMW.AuditWithType("ProcessGroupAppRequest"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.processGroupAppRequest,
	)

	rg.DELETE(
		"/groups/:id/apprequests/:rid",
		r.AuditMW.AuditWithType("DeleteGroupAppRequest"),
		r.AuthMW.AuthRequired([]string{oidcScope}),
		r.mwUserAuthRequired(AuthRoleUser),
		r.deleteGroupAppRequest,
	)

	rg.PUT(
		"/groups/:id/organizations/:oid",
		r.AuditMW.AuditWithType("AddGroupOrganization"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.addGroupOrganization,
	)

	rg.DELETE(
		"/groups/:id/organizations/:oid",
		r.AuditMW.AuditWithType("RemoveGroupOrganization"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.removeGroupOrganization,
	)

	rg.GET(
		"/groups/:id/hierarchies",
		r.AuditMW.AuditWithType("GetGroupHierarchies"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.listMemberGroups,
	)

	rg.POST(
		"/groups/:id/hierarchies",
		r.AuditMW.AuditWithType("CreateGroupHierarchy"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.addMemberGroup,
	)

	rg.PATCH(
		"/groups/:id/hierarchies/:member_id",
		r.AuditMW.AuditWithType("UpdateGroupHierarchy"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.updateMemberGroup,
	)

	rg.DELETE(
		"/groups/:id/hierarchies/:member_id",
		r.AuditMW.AuditWithType("DeleteGroupHierarchy"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:groups")),
		r.mwGroupAuthRequired(AuthRoleGroupAdmin),
		r.removeMemberGroup,
	)

	rg.GET(
		"/events",
		r.AuditMW.AuditWithType("ListEvents"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:events")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.listEvents,
	)

	rg.GET(
		"/organizations",
		r.AuditMW.AuditWithType("ListOrganizations"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:organizations")),
		r.listOrganizations,
	)

	rg.POST(
		"/organizations",
		r.AuditMW.AuditWithType("CreateOrganization"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:organizations")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createOrganization,
	)

	rg.GET(
		"/organizations/:id",
		r.AuditMW.AuditWithType("GetOrganizations"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:organizations")),
		r.getOrganization,
	)

	rg.DELETE(
		"/organizations/:id",
		r.AuditMW.AuditWithType("DeleteOrganization"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:organizations")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteOrganization,
	)

	rg.GET(
		"/organizations/:id/groups",
		r.AuditMW.AuditWithType("GetOrganizationGroups"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:organizations")),
		r.listOrganizationGroups,
	)

	rg.GET(
		"/applications",
		r.AuditMW.AuditWithType("ListApplciations"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:applications")),
		r.listApplications,
	)

	rg.POST(
		"/applications",
		r.AuditMW.AuditWithType("CreateApplications"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:applications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createApplication,
	)

	rg.GET(
		"/applications/:id",
		r.AuditMW.AuditWithType("GetApplication"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:applications")),
		r.getApplication,
	)

	rg.PUT(
		"/applications/:id",
		r.AuditMW.AuditWithType("UpdateApplication"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:applications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.updateApplication,
	)

	rg.DELETE(
		"/applications/:id",
		r.AuditMW.AuditWithType("DeleteApplication"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:applications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteApplication,
	)

	rg.GET(
		"/applications/:id/groups",
		r.AuditMW.AuditWithType("GetApplicationGroups"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:applications")),
		r.listApplicationGroups,
	)

	rg.GET(
		"/application-types",
		r.AuditMW.AuditWithType("ListApplciationTypes"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:applications")),
		r.listApplicationTypes,
	)

	rg.POST(
		"/application-types",
		r.AuditMW.AuditWithType("CreateApplicationType"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:applications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createApplicationType,
	)

	rg.GET(
		"/application-types/:id",
		r.AuditMW.AuditWithType("GetApplicationType"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:applications")),
		r.getApplicationType,
	)

	rg.PUT(
		"/application-types/:id",
		r.AuditMW.AuditWithType("UpdateApplicationType"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:applications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.updateApplicationType,
	)

	rg.DELETE(
		"/application-types/:id",
		r.AuditMW.AuditWithType("DeleteApplicationType"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:applications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteApplicationType,
	)

	rg.GET(
		"/application-types/:id/applications",
		r.AuditMW.AuditWithType("GetApplicationTypeApps"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:applications")),
		r.listApplicationTypeApps,
	)

	rg.GET(
		"/notification-types",
		r.AuditMW.AuditWithType("ListNotificationTypes"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:notifications")),
		r.listNotificationTypes,
	)

	rg.GET(
		"/notification-types/:id",
		r.AuditMW.AuditWithType("GetNotificationType"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:notifications")),
		r.getNotificationType,
	)

	rg.POST(
		"/notification-types",
		r.AuditMW.AuditWithType("CreateNotificationType"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:notifications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createNotificationType,
	)

	rg.PUT(
		"/notification-types/:id",
		r.AuditMW.AuditWithType("UpdateNotificationType"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:notifications")),
		r.updateNotificationType,
	)

	rg.DELETE(
		"/notification-types/:id",
		r.AuditMW.AuditWithType("DeleteNotificationType"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:notifications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteNotificationType,
	)

	rg.GET(
		"/notification-targets",
		r.AuditMW.AuditWithType("ListNotificationTargets"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:notifications")),
		r.listNotificationTargets,
	)

	rg.GET(
		"/notification-targets/:id",
		r.AuditMW.AuditWithType("GetNotificationTarget"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:notifications")),
		r.getNotificationTarget,
	)

	rg.POST(
		"/notification-targets",
		r.AuditMW.AuditWithType("CreateNotificationTarget"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:notifications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createNotificationTarget,
	)

	rg.PUT(
		"/notification-targets/:id",
		r.AuditMW.AuditWithType("UpdateNotificationTarget"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:notifications")),
		r.updateNotificationTarget,
	)

	rg.DELETE(
		"/notification-targets/:id",
		r.AuditMW.AuditWithType("DeleteNotificationTarget"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:notifications")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteNotificationTarget,
	)

	// extensions
	rg.GET(
		"/extensions",
		r.AuditMW.AuditWithType("ListExtensions"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:extensions")),
		r.listExtensions,
	)

	rg.GET(
		"/extensions/:eid",
		r.AuditMW.AuditWithType("GetExtension"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:extensions")),
		r.getExtension,
	)

	rg.POST(
		"/extensions",
		r.AuditMW.AuditWithType("CreateExtension"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensions")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.createExtension,
	)

	rg.PATCH(
		"/extensions/:eid",
		r.AuditMW.AuditWithType("UpdateExtension"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:extensions")),
		r.updateExtension,
	)

	rg.DELETE(
		"/extensions/:eid",
		r.AuditMW.AuditWithType("DeleteExtension"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:extensions")),
		r.mwUserAuthRequired(AuthRoleAdmin),
		r.deleteExtension,
	)

	// extension resource definitions
	rg.GET(
		"/extensions/:eid/erds",
		r.AuditMW.AuditWithType("ListExtensionResourceDefinitions"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:extensions")),
		r.listExtensionResourceDefinitions,
	)

	rg.POST(
		"/extensions/:eid/erds",
		r.AuditMW.AuditWithType("CreateExtensionResourceDefinition"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensions")),
		r.createExtensionResourceDefinition,
	)

	rg.GET(
		"/extensions/:eid/erds/:erd-id-slug",
		r.AuditMW.AuditWithType("GetExtensionResourceDefinitionByID"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:extensions")),
		r.getExtensionResourceDefinition,
	)

	rg.GET(
		"/extensions/:eid/erds/:erd-id-slug/:erd-version",
		r.AuditMW.AuditWithType("GetExtensionResourceDefinitionBySlug"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:extensions")),
		r.getExtensionResourceDefinition,
	)

	rg.PATCH(
		"/extensions/:eid/erds/:erd-id-slug",
		r.AuditMW.AuditWithType("UpdateExtensionResourceDefinitionByID"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:extensions")),
		r.updateExtensionResourceDefinition,
	)

	rg.PATCH(
		"/extensions/:eid/erds/:erd-id-slug/:erd-version",
		r.AuditMW.AuditWithType("UpdateExtensionResourceDefinitionBySlug"),
		r.AuthMW.AuthRequired(updateScopesWithOpenID("governor:extensions")),
		r.updateExtensionResourceDefinition,
	)

	rg.DELETE(
		"/extensions/:eid/erds/:erd-id-slug",
		r.AuditMW.AuditWithType("DeleteExtensionResourceDefinitionByID"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:extensions")),
		r.deleteExtensionResourceDefinition,
	)

	rg.DELETE(
		"/extensions/:eid/erds/:erd-id-slug/:erd-version",
		r.AuditMW.AuditWithType("DeleteExtensionResourceDefinitionBySlug"),
		r.AuthMW.AuthRequired(deleteScopesWithOpenID("governor:extensions")),
		r.deleteExtensionResourceDefinition,
	)

	// system-wise extension resources
	rg.POST(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("CreateSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.createSystemExtensionResource,
	)

	rg.GET(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version",
		r.AuditMW.AuditWithType("ListSystemExtensionResources"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.listSystemExtensionResources,
	)

	rg.GET(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("GetSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.getSystemExtensionResource,
	)

	rg.PATCH(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("UpdateSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.updateSystemExtensionResource,
	)

	rg.DELETE(
		"/extension-resources/:ex-slug/:erd-slug-plural/:erd-version/:resource-id",
		r.AuditMW.AuditWithType("DeleteSystemExtensionResource"),
		r.AuthMW.AuthRequired(createScopesWithOpenID("governor:extensionresources")),
		r.deleteSystemExtensionResource,
	)
}

func contains(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}

	return false
}

// createScopesWithOpenID returns the openid scope in addition to the standard governor create scopes
func createScopesWithOpenID(sc string) []string {
	return append(ginjwt.CreateScopes(sc), oidcScope)
}

// deleteScopesWithOpenID returns the openid scope in addition to the standard governor delete scopes
func deleteScopesWithOpenID(sc string) []string {
	return append(ginjwt.DeleteScopes(sc), oidcScope)
}

// readScopesWithOpenID returns the openid scope in addition to the standard governor read scopes
func readScopesWithOpenID(sc string) []string {
	return append(ginjwt.ReadScopes(sc), oidcScope)
}

// updateScopesWithOpenID returns the openid scope in addition to the standard governor update scopes
func updateScopesWithOpenID(sc string) []string {
	return append(ginjwt.UpdateScopes(sc), oidcScope)
}
