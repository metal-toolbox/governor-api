package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

// AuthenticatedUser is an authenticated user response
type AuthenticatedUser struct {
	*User
	Admin bool `json:"admin"`
}

// AuthenticatedUserReq is an authenticated user request payload for updating selected details
type AuthenticatedUserReq struct {
	AvatarURL               *string                     `json:"avatar_url"`
	GithubUsername          *string                     `json:"github_username"`
	NotificationPreferences UserNotificationPreferences `json:"notification_preferences,omitempty"`
}

// AuthenticatedUserGroup is an authenticated user group response
type AuthenticatedUserGroup struct {
	*models.Group
	Organizations models.OrganizationSlice `json:"organizations"`
	Applications  models.ApplicationSlice  `json:"applications"`
	Admin         bool                     `json:"admin"`
	Direct        bool                     `json:"direct"`
}

// AuthenticatedUserRequests is a list of application and member requests for the authenticated user
type AuthenticatedUserRequests struct {
	ApplicationRequests []AuthenticatedUserGroupApplicationRequest `json:"application_requests"`
	MemberRequests      []AuthenticatedUserGroupMemberRequest      `json:"member_requests"`
}

// AuthenticatedUserGroupApplicationRequest is an authenticated user group application request
type AuthenticatedUserGroupApplicationRequest struct {
	*GroupApplicationRequest
}

// AuthenticatedUserGroupMemberRequest is an authenticated user group membership request
type AuthenticatedUserGroupMemberRequest struct {
	*GroupMemberRequest
	Admin bool `json:"admin"`
}

const expectedAuthzHeaderParts = 2

// getAuthenticatedUser gets information about the currently authenticated OAuth user. If the
// user is not found in the db they will be automatically added using details from oidc
func (r *Router) getAuthenticatedUser(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	ctxAdmin := getCtxAdmin(c)
	if ctxAdmin == nil {
		sendError(c, http.StatusUnauthorized, "no admin in context")
		return
	}

	notificationPreferences, err := dbtools.GetNotificationPreferences(c.Request.Context(), ctxUser.ID, r.DB, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting notification preferences: "+err.Error())
		return
	}

	if ctxUser.R == nil {
		c.JSON(http.StatusOK, AuthenticatedUser{
			User: &User{
				User:                    ctxUser,
				Memberships:             []string{},
				MembershipsDirect:       []string{},
				MembershipRequests:      []string{},
				NotificationPreferences: notificationPreferences,
			},
			Admin: *ctxAdmin,
		})

		return
	}

	enumeratedMemberships, err := dbtools.GetMembershipsForUser(c, r.DB.DB, ctxUser.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	memberships := make([]string, len(enumeratedMemberships))

	membershipsDirect := make([]string, 0)

	for i, m := range enumeratedMemberships {
		memberships[i] = m.GroupID

		if m.Direct {
			membershipsDirect = append(membershipsDirect, m.GroupID)
		}
	}

	requests := make([]string, len(ctxUser.R.GroupMembershipRequests))
	for i, r := range ctxUser.R.GroupMembershipRequests {
		requests[i] = r.GroupID
	}

	c.JSON(http.StatusOK, AuthenticatedUser{
		User: &User{
			User:                    ctxUser,
			Memberships:             memberships,
			MembershipsDirect:       membershipsDirect,
			MembershipRequests:      requests,
			NotificationPreferences: notificationPreferences,
		},
		Admin: *ctxAdmin,
	})
}

// getAuthenticatedUserGroups returns a list of groups that the authenticated user is a member of
func (r *Router) getAuthenticatedUserGroups(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	var userAdminGroups []string

	var userDirectGroups []string

	enumeratedMemberships, err := dbtools.GetMembershipsForUser(c, r.DB.DB, ctxUser.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	gids := make([]interface{}, len(enumeratedMemberships))
	for i, g := range enumeratedMemberships {
		gids[i] = g.GroupID

		if g.IsAdmin {
			userAdminGroups = append(userAdminGroups, g.GroupID)
		}

		if g.Direct {
			userDirectGroups = append(userDirectGroups, g.GroupID)
		}
	}

	groups, err := models.Groups(
		qm.WhereIn("id IN ?", gids...),
		qm.Load("GroupOrganizations"),
		qm.Load("GroupOrganizations.Organization"),
		qm.Load("GroupApplications"),
		qm.Load("GroupApplications.Application"),
	).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error getting user groups: "+err.Error())
		return
	}

	var userGroups []AuthenticatedUserGroup

	for _, g := range groups {
		var orgs models.OrganizationSlice
		for _, o := range g.R.GroupOrganizations {
			orgs = append(orgs, o.R.Organization)
		}

		var apps models.ApplicationSlice
		for _, a := range g.R.GroupApplications {
			apps = append(apps, a.R.Application)
		}

		userGroups = append(userGroups, AuthenticatedUserGroup{
			Group:         g,
			Organizations: orgs,
			Applications:  apps,
			Admin:         contains(userAdminGroups, g.ID),
			Direct:        contains(userDirectGroups, g.ID),
		})
	}

	c.JSON(http.StatusOK, userGroups)
}

// getAuthenticatedUserGroupApprovals returns a list of group member requests and
// group application requests that the authenticated user can approve
func (r *Router) getAuthenticatedUserGroupApprovals(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	ctxAdmin := getCtxAdmin(c)
	if ctxAdmin == nil {
		sendError(c, http.StatusUnauthorized, "no admin in context")
		return
	}

	var userGroups, userAdminGroups []string

	enumeratedMemberships, err := dbtools.GetMembershipsForUser(c, r.DB.DB, ctxUser.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	for _, g := range enumeratedMemberships {
		userGroups = append(userGroups, g.GroupID)

		if g.IsAdmin {
			userAdminGroups = append(userAdminGroups, g.GroupID)
		}
	}

	membershipRequests, err := models.GroupMembershipRequests(
		qm.Load("User"),
		qm.Load("Group"),
	).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting group membership requests: "+err.Error())
		return
	}

	applicationRequests, err := models.GroupApplicationRequests(
		qm.Load("Application"),
		qm.Load("Group"),
		qm.Load("ApproverGroup"),
		qm.Load("RequesterUser"),
	).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting group application requests: "+err.Error())
		return
	}

	var memberApprovals []AuthenticatedUserGroupMemberRequest

	for _, m := range membershipRequests {
		if ctxUser.ID == m.UserID {
			continue
		}

		isGroupAdmin := contains(userAdminGroups, m.GroupID)

		if *ctxAdmin || isGroupAdmin {
			memberApprovals = append(memberApprovals, AuthenticatedUserGroupMemberRequest{&GroupMemberRequest{
				ID:            m.ID,
				GroupID:       m.GroupID,
				GroupName:     m.R.Group.Name,
				GroupSlug:     m.R.Group.Slug,
				UserID:        m.UserID,
				UserName:      m.R.User.Name,
				UserEmail:     m.R.User.Email,
				UserAvatarURL: m.R.User.AvatarURL.String,
				CreatedAt:     m.CreatedAt,
				UpdatedAt:     m.UpdatedAt,
				IsAdmin:       m.IsAdmin,
			}, isGroupAdmin})
		}
	}

	var applicationApprovals []AuthenticatedUserGroupApplicationRequest

	for _, a := range applicationRequests {
		if ctxUser.ID == a.RequesterUserID {
			continue
		}

		if !contains(userGroups, a.ApproverGroupID) {
			continue
		}

		applicationApprovals = append(applicationApprovals, AuthenticatedUserGroupApplicationRequest{&GroupApplicationRequest{
			ID:                     a.ID,
			ApplicationID:          a.ApplicationID,
			ApplicationName:        a.R.Application.Name,
			ApplicationSlug:        a.R.Application.Slug,
			ApproverGroupID:        a.ApproverGroupID,
			ApproverGroupName:      a.R.ApproverGroup.Name,
			ApproverGroupSlug:      a.R.ApproverGroup.Slug,
			GroupID:                a.GroupID,
			GroupName:              a.R.Group.Name,
			GroupSlug:              a.R.Group.Slug,
			RequesterUserID:        a.RequesterUserID,
			RequesterUserName:      a.R.RequesterUser.Name,
			RequesterUserEmail:     a.R.RequesterUser.Email,
			RequesterUserAvatarURL: a.R.RequesterUser.AvatarURL.String,
			Note:                   a.Note.String,
			CreatedAt:              a.CreatedAt,
			UpdatedAt:              a.UpdatedAt,
		}})
	}

	c.JSON(http.StatusOK, AuthenticatedUserRequests{
		ApplicationRequests: applicationApprovals,
		MemberRequests:      memberApprovals,
	})
}

// getAuthenticatedUserGroupRequests returns a list of group member requests and
// group application requests that the authenticated user has made
func (r *Router) getAuthenticatedUserGroupRequests(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	memberRequests := make([]AuthenticatedUserGroupMemberRequest, len(ctxUser.R.GroupMembershipRequests))

	for i, m := range ctxUser.R.GroupMembershipRequests {
		gmr := GroupMemberRequest{
			ID:            m.ID,
			GroupID:       m.GroupID,
			GroupName:     m.R.Group.Name,
			GroupSlug:     m.R.Group.Slug,
			UserID:        m.UserID,
			UserName:      m.R.User.Name,
			UserEmail:     m.R.User.Email,
			UserAvatarURL: m.R.User.AvatarURL.String,
			CreatedAt:     m.CreatedAt,
			UpdatedAt:     m.UpdatedAt,
			IsAdmin:       m.IsAdmin,
			Note:          m.Note,
		}

		memberRequests[i] = AuthenticatedUserGroupMemberRequest{
			GroupMemberRequest: &gmr,
			Admin:              false,
		}
	}

	applicationRequests := make([]AuthenticatedUserGroupApplicationRequest, len(ctxUser.R.RequesterUserGroupApplicationRequests))

	for i, a := range ctxUser.R.RequesterUserGroupApplicationRequests {
		gar := GroupApplicationRequest{
			ID:                     a.ID,
			ApplicationID:          a.ApplicationID,
			ApplicationName:        a.R.Application.Name,
			ApplicationSlug:        a.R.Application.Slug,
			ApproverGroupID:        a.ApproverGroupID,
			ApproverGroupName:      a.R.ApproverGroup.Name,
			ApproverGroupSlug:      a.R.ApproverGroup.Slug,
			GroupID:                a.GroupID,
			GroupName:              a.R.Group.Name,
			GroupSlug:              a.R.Group.Slug,
			RequesterUserID:        a.RequesterUserID,
			RequesterUserName:      a.R.RequesterUser.Name,
			RequesterUserEmail:     a.R.RequesterUser.Email,
			RequesterUserAvatarURL: a.R.RequesterUser.AvatarURL.String,
			Note:                   a.Note.String,
			CreatedAt:              a.CreatedAt,
			UpdatedAt:              a.UpdatedAt,
		}

		applicationRequests[i] = AuthenticatedUserGroupApplicationRequest{
			GroupApplicationRequest: &gar,
		}
	}

	c.JSON(http.StatusOK, AuthenticatedUserRequests{
		ApplicationRequests: applicationRequests,
		MemberRequests:      memberRequests,
	})
}

// removeAuthenticatedUserGroup removes the authenticated user from the specified group
func (r *Router) removeAuthenticatedUserGroup(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	gid := c.Param("id")

	q := qm.Where("id = ?", gid)
	if _, err := uuid.Parse(gid); err != nil {
		q = qm.Where("slug = ?", gid)
	}

	group, err := models.Groups(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group"+err.Error())

		return
	}

	membership, err := models.GroupMemberships(
		qm.Where("group_id = ?", group.ID),
		qm.And("user_id = ?", ctxUser.ID),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not in group (or not a direct member)")
			return
		}

		sendError(c, http.StatusInternalServerError, "error checking membership exists: "+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group membership delete transaction: "+err.Error())
		return
	}

	if _, err := membership.Delete(c.Request.Context(), tx); err != nil {
		msg := "error removing membership: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupMembershipDeleted(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, membership)
	if err != nil {
		msg := "error removing membership (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error removing membership (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing remove membership, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// only publish events for active users
	if !isActiveUser(ctxUser) {
		c.JSON(http.StatusNoContent, nil)
		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMembersEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventDelete,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: gid,
		UserID:  ctxUser.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish members delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// updateAuthenticatedUser updates details about the authenticated user
func (r *Router) updateAuthenticatedUser(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	original := *ctxUser

	req := AuthenticatedUserReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.AvatarURL != nil {
		ctxUser.AvatarURL = null.StringFrom(*req.AvatarURL)
	}

	if req.GithubUsername != nil {
		ctxUser.GithubUsername = null.StringFrom(*req.GithubUsername)
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	if _, err := ctxUser.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error updating user: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditUserUpdated(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, &original, ctxUser)
	if err != nil {
		msg := "error updating user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	var updateNotificationPublishEventErr error

	_, status, err := handleUpdateNotificationPreferencesRequests(
		c, tx, ctxUser, r.EventBus, req.NotificationPreferences,
	)
	if err != nil && !errors.Is(err, ErrNotificationPreferencesEmptyInput) {
		if errors.Is(err, ErrPublishUpdateNotificationPreferences) {
			updateNotificationPublishEventErr = err
		} else {
			msg := err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, status, msg)

			return
		}
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing user update, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if updateNotificationPublishEventErr != nil {
		sendError(c, http.StatusBadRequest, updateNotificationPublishEventErr.Error())
		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorUsersEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  ctxUser.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish user update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, ctxUser)
}
