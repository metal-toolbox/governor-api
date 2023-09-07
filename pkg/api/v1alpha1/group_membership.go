package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

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

// GroupMember is a group member (user)
type GroupMember struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	Status    string    `json:"status"`
	IsAdmin   bool      `json:"is_admin"`
	ExpiresAt null.Time `json:"expires_at"`
	Direct    bool      `json:"direct"`
}

// GroupMembership is the relationship between user and groups
type GroupMembership struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	GroupSlug string    `json:"group_slug"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	ExpiresAt null.Time `json:"expires_at"`
}

// GroupMemberRequest is a pending user request for group membership
type GroupMemberRequest struct {
	ID            string    `json:"id"`
	GroupID       string    `json:"group_id"`
	GroupName     string    `json:"group_name"`
	GroupSlug     string    `json:"group_slug"`
	UserID        string    `json:"user_id"`
	UserName      string    `json:"user_name"`
	UserEmail     string    `json:"user_email"`
	UserAvatarURL string    `json:"user_avatar_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	IsAdmin       bool      `json:"is_admin"`
	Note          string    `json:"note"`
	ExpiresAt     null.Time `json:"expires_at"`
}

type createGroupMemberReq struct {
	IsAdmin   bool      `json:"is_admin"`
	Note      string    `json:"note"`
	ExpiresAt null.Time `json:"expires_at"`
}

// listGroupMembers returns a list of users in a group
func (r *Router) listGroupMembers(c *gin.Context) {
	gid := c.Param("id")

	queryMods := []qm.QueryMod{}

	q := qm.Where("id = ?", gid)

	if _, err := uuid.Parse(gid); err != nil {
		q = qm.Where("slug = ?", gid)
	}

	queryMods = append(queryMods, q)

	group, err := models.Groups(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group"+err.Error())

		return
	}

	enumeratedMembers, err := dbtools.GetMembersOfGroup(c, r.DB.DB, group.ID, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	members := make([]GroupMember, len(enumeratedMembers))
	for i, m := range enumeratedMembers {
		members[i] = GroupMember{
			ID:        m.User.ID,
			Name:      m.User.Name,
			Email:     m.User.Email,
			AvatarURL: m.User.AvatarURL.String,
			Status:    m.User.Status.String,
			IsAdmin:   m.IsAdmin,
			ExpiresAt: m.ExpiresAt,
			Direct:    m.Direct,
		}
	}

	c.JSON(http.StatusOK, members)
}

// addGroupMember adds a user to a group
func (r *Router) addGroupMember(c *gin.Context) {
	gid := c.Param("id")
	uid := c.Param("uid")

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

	user, err := models.FindUser(c.Request.Context(), r.DB, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

		return
	}

	req := struct {
		IsAdmin   bool      `json:"is_admin"`
		ExpiresAt null.Time `json:"expires_at"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	exists, err := models.GroupMemberships(
		qm.Where("group_id = ?", group.ID),
		qm.And("user_id = ?", user.ID),
	).Exists(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error checking membership exists: "+err.Error())
		return
	}

	if exists {
		sendError(c, http.StatusConflict, "user already in group")
		return
	}

	groupMem := &models.GroupMembership{
		GroupID:   group.ID,
		UserID:    user.ID,
		IsAdmin:   req.IsAdmin,
		ExpiresAt: req.ExpiresAt,
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting add groups membership transaction: "+err.Error())
		return
	}

	membershipsBefore, err := dbtools.GetMembershipsForUser(c, tx, user.ID, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := groupMem.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "failed to update group membership: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupMembershipCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), groupMem)
	if err != nil {
		msg := "error creating groups membership (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating groups membership (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	membershipsAfter, err := dbtools.GetMembershipsForUser(c, tx, user.ID, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing groups membership, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// only publish events for active users
	if !isActiveUser(user) {
		c.JSON(http.StatusNoContent, nil)
		return
	}

	groupsDiff := dbtools.FindMemberDiff(membershipsBefore, membershipsAfter)

	for _, enumeratedMembership := range groupsDiff {
		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMembersEventSubject, &events.Event{
			Version: events.Version,
			Action:  events.GovernorEventCreate,
			AuditID: c.GetString(ginaudit.AuditIDContextKey),
			GroupID: enumeratedMembership.GroupID,
			UserID:  enumeratedMembership.UserID,
			ActorID: getCtxActorID(c),
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish members create event, downstream changes may be delayed "+err.Error())
			return
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

// updateGroupMember promotes or demotes a user to a group admin
func (r *Router) updateGroupMember(c *gin.Context) {
	gid := c.Param("id")
	uid := c.Param("uid")

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

	user, err := models.FindUser(c.Request.Context(), r.DB, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

		return
	}

	req := struct {
		IsAdmin bool `json:"is_admin"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	membership, err := models.GroupMemberships(
		qm.Where("group_id = ?", group.ID),
		qm.And("user_id = ?", user.ID),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not in group (or not a direct member)")
			return
		}

		sendError(c, http.StatusInternalServerError, "error checking membership exists: "+err.Error())

		return
	}

	// if there is user in the context, check that they are not trying to promote themselves
	// (but they are allowed to step down as admin)
	ctxUser := getCtxUser(c)
	if ctxUser != nil && ctxUser.ID == user.ID && !(membership.IsAdmin && !req.IsAdmin) {
		sendError(c, http.StatusBadRequest, "unable to change own membership")
		return
	}

	original := *membership

	membership.IsAdmin = req.IsAdmin

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update groups membership transaction: "+err.Error())
		return
	}

	if _, err := membership.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		rollbackWithError(c, tx, err, http.StatusBadRequest, "failed to update group member admin flag")

		return
	}

	var event *models.AuditEvent

	switch {
	case membership.IsAdmin && !original.IsAdmin:
		var err error

		// user is promoted
		event, err = dbtools.AuditGroupMemberPromoted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), membership)
		if err != nil {
			rollbackWithError(c, tx, err, http.StatusBadRequest, "error updating groups membership (audit)")

			return
		}
	case original.IsAdmin && !membership.IsAdmin:
		var err error

		// user is demoted
		event, err = dbtools.AuditGroupMemberDemoted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), membership)
		if err != nil {
			rollbackWithError(c, tx, err, http.StatusBadRequest, "error updating groups membership (audit)")

			return
		}
	default:
		var err error

		// something else was updated
		event, err = dbtools.AuditGroupMembershipUpdated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, membership)
		if err != nil {
			rollbackWithError(c, tx, err, http.StatusBadRequest, "error updating groups membership (audit)")

			return
		}
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		rollbackWithError(c, tx, err, http.StatusBadRequest, "error updating groups membership (audit)")

		return
	}

	if err := tx.Commit(); err != nil {
		rollbackWithError(c, tx, err, http.StatusBadRequest, "error committing membership update, rolling back")

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMembersEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		GroupID: group.ID,
		UserID:  user.ID,
		ActorID: getCtxActorID(c),
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish member update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// removeGroupMember removes a user from a group
func (r *Router) removeGroupMember(c *gin.Context) {
	gid := c.Param("id")
	uid := c.Param("uid")

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

	user, err := models.FindUser(c.Request.Context(), r.DB, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

		return
	}

	membership, err := models.GroupMemberships(
		qm.Where("group_id = ?", group.ID),
		qm.And("user_id = ?", user.ID),
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
		sendError(c, http.StatusBadRequest, "error starting delete groups membership transaction: "+err.Error())
		return
	}

	membershipsBefore, err := dbtools.GetMembershipsForUser(c, tx, user.ID, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

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

	event, err := dbtools.AuditGroupMembershipDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), membership)
	if err != nil {
		msg := "error deleting groups membership (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting group membership (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	membershipsAfter, err := dbtools.GetMembershipsForUser(c, tx, user.ID, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing membership delete, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// only publish events for active users
	if !isActiveUser(user) {
		c.JSON(http.StatusNoContent, nil)
		return
	}

	groupsDiff := dbtools.FindMemberDiff(membershipsAfter, membershipsBefore)

	for _, enumeratedMembership := range groupsDiff {
		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMembersEventSubject, &events.Event{
			Version: events.Version,
			Action:  events.GovernorEventDelete,
			AuditID: c.GetString(ginaudit.AuditIDContextKey),
			GroupID: enumeratedMembership.GroupID,
			UserID:  enumeratedMembership.UserID,
			ActorID: getCtxActorID(c),
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish members delete event, downstream changes may be delayed "+err.Error())
			return
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

// createGroupRequest creates a request to join a group
func (r *Router) createGroupRequest(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	req := createGroupMemberReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
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

	for _, m := range ctxUser.R.GroupMemberships {
		if m.GroupID == group.ID {
			sendError(c, http.StatusBadRequest, "user already member of the group")
			return
		}
	}

	for _, r := range ctxUser.R.GroupMembershipRequests {
		if r.GroupID == group.ID {
			sendError(c, http.StatusConflict, "user already requested access to the group")
			return
		}
	}

	groupMembershipRequest := &models.GroupMembershipRequest{
		GroupID:   group.ID,
		UserID:    ctxUser.ID,
		IsAdmin:   req.IsAdmin,
		Note:      req.Note,
		ExpiresAt: req.ExpiresAt,
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting groups membership request transaction: "+err.Error())
		return
	}

	if err := group.AddGroupMembershipRequests(c.Request.Context(), tx, true, groupMembershipRequest); err != nil {
		msg := "failed to create group request: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupMembershipRequestCreated(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, groupMembershipRequest)
	if err != nil {
		msg := "error creating group membership request (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error group membership request (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group membership request, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMemberRequestsEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventCreate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: groupMembershipRequest.GroupID,
		UserID:  groupMembershipRequest.UserID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish member request create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// deleteGroupRequest deletes/revokes a pending request to join a group. This can only be done
// by the user who created the request.
func (r *Router) deleteGroupRequest(c *gin.Context) {
	ctxUser := getCtxUser(c)

	gid := c.Param("id")
	rid := c.Param("rid")

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

	request, err := models.GroupMembershipRequests(qm.Where("id = ?", rid)).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group request not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group request"+err.Error())

		return
	}

	if request.GroupID != group.ID {
		sendError(c, http.StatusBadRequest, "request not associated with this group")
		return
	}

	if ctxUser != nil && request.UserID != ctxUser.ID {
		sendError(c, http.StatusBadRequest, "request not associated with this user")
		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete group request transaction: "+err.Error())
		return
	}

	if _, err := request.Delete(c.Request.Context(), tx); err != nil {
		msg := "failed to delete group request: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupMembershipRevoked(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, request)
	if err != nil {
		msg := "error deleting group membership request (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error group membership request (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group request delete, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	userID := ""
	if ctxUser != nil {
		userID = ctxUser.ID
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMemberRequestsEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventRevoke,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: group.ID,
		UserID:  userID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish member request revoke event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// getGroupRequests returns all pending requests to join a group
func (r *Router) getGroupRequests(c *gin.Context) {
	gid := c.Param("id")

	queryMods := []qm.QueryMod{
		qm.Load("GroupMembershipRequests"),
		qm.Load("GroupMembershipRequests.User"),
		qm.Load("GroupMembershipRequests.Group"),
	}

	q := qm.Where("id = ?", gid)
	if _, err := uuid.Parse(gid); err != nil {
		q = qm.Where("slug = ?", gid)
	}

	queryMods = append(queryMods, q)

	group, err := models.Groups(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group"+err.Error())

		return
	}

	requests := make([]GroupMemberRequest, len(group.R.GroupMembershipRequests))
	for i, m := range group.R.GroupMembershipRequests {
		requests[i] = GroupMemberRequest{
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
			ExpiresAt:     m.ExpiresAt,
		}
	}

	c.JSON(http.StatusOK, requests)
}

// processGroupRequest approves or denies a pending request to join a group. This can only be done
// by an admin or a group admin.
//
//nolint:gocyclo
func (r *Router) processGroupRequest(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	gid := c.Param("id")
	rid := c.Param("rid")

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

	request, err := models.GroupMembershipRequests(qm.Where("id = ?", rid)).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group request not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group request"+err.Error())

		return
	}

	if request.GroupID != group.ID {
		sendError(c, http.StatusBadRequest, "request not associated with this group")
		return
	}

	if request.UserID == ctxUser.ID {
		sendError(c, http.StatusBadRequest, "unable to approve/deny own request")
		return
	}

	req := struct {
		Action string `json:"action"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	switch req.Action {
	case "approve":
		// approving a request will first check that the requesting user is not already a member
		// of the group, then add them to the group, and finally delete the request
		user, err := models.FindUser(c.Request.Context(), r.DB, request.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				sendError(c, http.StatusBadRequest, "requesting user not found: "+err.Error())
				return
			}

			sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

			return
		}

		exists, err := models.GroupMemberships(
			qm.Where("group_id = ?", request.GroupID),
			qm.And("user_id = ?", request.UserID),
		).Exists(c.Request.Context(), r.DB)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "error checking membership exists: "+err.Error())
			return
		}

		if exists {
			// if the user is already a member of the group, we can just delete the request
			if _, err := request.Delete(c.Request.Context(), r.DB); err != nil {
				sendError(c, http.StatusBadRequest, "failed to delete group request: "+err.Error())
				return
			}

			sendError(c, http.StatusConflict, "user already in group")

			return
		}

		groupMem := &models.GroupMembership{
			GroupID:   request.GroupID,
			UserID:    request.UserID,
			IsAdmin:   request.IsAdmin,
			ExpiresAt: request.ExpiresAt,
		}

		tx, err := r.DB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			sendError(c, http.StatusBadRequest, "error starting group membership approval transaction: "+err.Error())
			return
		}

		membershipsBefore, err := dbtools.GetMembershipsForUser(c, tx, user.ID, false)
		if err != nil {
			msg := "failed to compute new effective memberships: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := groupMem.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
			msg := "error approving group membership request , rolling back: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if _, err := request.Delete(c.Request.Context(), tx); err != nil {
			msg := "error deleting group membership request on approval, rolling back: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		event, err := dbtools.AuditGroupMembershipApproved(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, groupMem)
		if err != nil {
			msg := "error approving group membership request (audit): " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := updateContextWithAuditEventData(c, event); err != nil {
			msg := "error approving group membership request (audit): " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		membershipsAfter, err := dbtools.GetMembershipsForUser(c, tx, user.ID, false)
		if err != nil {
			msg := "failed to compute new effective memberships: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := tx.Commit(); err != nil {
			msg := "error committing group membership approval, rolling back: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		// only publish events for active users
		if !isActiveUser(user) {
			c.JSON(http.StatusNoContent, nil)
			return
		}

		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMemberRequestsEventSubject, &events.Event{
			Version: events.Version,
			Action:  events.GovernorEventApprove,
			AuditID: c.GetString(ginaudit.AuditIDContextKey),
			GroupID: groupMem.GroupID,
			UserID:  groupMem.UserID,
			ActorID: getCtxActorID(c),
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish member request approve event, downstream changes may be delayed "+err.Error())
			return
		}

		groupsDiff := dbtools.FindMemberDiff(membershipsBefore, membershipsAfter)

		for _, enumeratedMembership := range groupsDiff {
			if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMembersEventSubject, &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: c.GetString(ginaudit.AuditIDContextKey),
				GroupID: enumeratedMembership.GroupID,
				UserID:  enumeratedMembership.UserID,
				ActorID: getCtxActorID(c),
			}); err != nil {
				sendError(c, http.StatusBadRequest, "failed to publish members create event, downstream changes may be delayed "+err.Error())
				return
			}
		}

		c.JSON(http.StatusNoContent, nil)

		return

	case "deny":
		tx, err := r.DB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			sendError(c, http.StatusBadRequest, "error starting group membership denial transaction: "+err.Error())
			return
		}

		// denying a request simply deletes it
		if _, err := request.Delete(c.Request.Context(), tx); err != nil {
			sendError(c, http.StatusBadRequest, "failed to delete group request: "+err.Error())
			return
		}

		event, err := dbtools.AuditGroupMembershipDenied(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, request)
		if err != nil {
			msg := "error denying group membership request (audit): " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := tx.Commit(); err != nil {
			msg := "error committing group membership deny, rolling back: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := updateContextWithAuditEventData(c, event); err != nil {
			msg := "error denying group membership request (audit): " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMemberRequestsEventSubject, &events.Event{
			Version: events.Version,
			Action:  events.GovernorEventDeny,
			AuditID: c.GetString(ginaudit.AuditIDContextKey),
			GroupID: request.GroupID,
			UserID:  request.UserID,
			ActorID: getCtxActorID(c),
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish member request deny event, downstream changes may be delayed "+err.Error())
			return
		}

		c.JSON(http.StatusNoContent, nil)

		return

	default:
		sendError(c, http.StatusBadRequest, "invalid action "+req.Action)
		return
	}
}

// getGroupMembershipsAll returns all group memberships for all groups
func (r *Router) getGroupMembershipsAll(c *gin.Context) {
	ctx := c.Request.Context()
	queryMods := []qm.QueryMod{
		qm.Load("User"),
		qm.Load("Group"),
	}

	var response []GroupMembership

	if _, ok := c.GetQuery("expired"); ok {
		queryMods = append(queryMods, qm.Where("expires_at <= NOW()"))

		groupMemberships, err := models.GroupMemberships(queryMods...).All(ctx, r.DB)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				sendError(c, http.StatusInternalServerError, "error getting group memberships"+err.Error())
				return
			}
		}

		response = make([]GroupMembership, len(groupMemberships))
		for i, m := range groupMemberships {
			response[i] = GroupMembership{
				ID:        m.ID,
				GroupID:   m.GroupID,
				GroupSlug: m.R.Group.Slug,
				UserID:    m.UserID,
				UserEmail: m.R.User.Email,
				ExpiresAt: m.ExpiresAt,
			}
		}
	} else {
		enumeratedMemberships, err := dbtools.GetAllGroupMemberships(c, r.DB.DB, true)
		if err != nil {
			if err != nil {
				sendError(c, http.StatusInternalServerError, "error getting group memberships"+err.Error())
				return
			}
		}

		response = make([]GroupMembership, len(enumeratedMemberships))
		for i, m := range enumeratedMemberships {
			response[i] = GroupMembership{
				ID:        "",
				GroupID:   m.GroupID,
				GroupSlug: m.Group.Slug,
				UserID:    m.UserID,
				UserEmail: m.User.Email,
				ExpiresAt: m.ExpiresAt,
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// getGroupRequests returns all pending requests to join any group
func (r *Router) getGroupRequestsAll(c *gin.Context) {
	ctx := c.Request.Context()
	queryMods := []qm.QueryMod{
		qm.Load("User"),
		qm.Load("Group"),
	}

	if _, ok := c.GetQuery("expired"); ok {
		queryMods = append(queryMods, qm.Where("expires_at <= NOW()"))
	}

	groupMembershipRequests, err := models.GroupMembershipRequests(queryMods...).All(ctx, r.DB)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusInternalServerError, "error getting group membership requests"+err.Error())
			return
		}
	}

	response := make([]GroupMemberRequest, len(groupMembershipRequests))
	for i, m := range groupMembershipRequests {
		response[i] = GroupMemberRequest{
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
			ExpiresAt:     m.ExpiresAt,
		}
	}

	c.JSON(http.StatusOK, response)
}
