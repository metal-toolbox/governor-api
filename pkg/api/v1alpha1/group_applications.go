package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

// GroupApplicationRequest is a pending request to link an application to a group
type GroupApplicationRequest struct {
	ID                     string    `json:"id"`
	ApplicationID          string    `json:"application_id"`
	ApplicationName        string    `json:"application_name"`
	ApplicationSlug        string    `json:"application_slug"`
	ApproverGroupID        string    `json:"approver_group_id"`
	ApproverGroupName      string    `json:"approver_group_name"`
	ApproverGroupSlug      string    `json:"approver_group_slug"`
	GroupID                string    `json:"group_id"`
	GroupName              string    `json:"group_name"`
	GroupSlug              string    `json:"group_slug"`
	RequesterUserID        string    `json:"requester_user_id"`
	RequesterUserName      string    `json:"requester_user_name"`
	RequesterUserEmail     string    `json:"requester_user_email"`
	RequesterUserAvatarURL string    `json:"requester_user_avatar_url"`
	Note                   string    `json:"note"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// addGroupApplication links an application to a group
func (r *Router) addGroupApplication(c *gin.Context) {
	gid := c.Param("id")
	oid := c.Param("oid")

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

	qa := qm.Where("id = ?", oid)

	if _, err := uuid.Parse(oid); err != nil {
		qa = qm.Where("slug = ?", oid)
	}

	app, err := models.Applications(qa).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

		return
	}

	exists, err := models.GroupApplications(qm.Where("group_id=?", group.ID), qm.And("application_id=?", app.ID)).Exists(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error checking group application link exists "+err.Error())
		return
	}

	if exists {
		sendError(c, http.StatusConflict, "application already linked to group")
		return
	}

	// if the application requires approval we'll return an error
	if !app.ApproverGroupID.IsZero() {
		sendError(c, http.StatusBadRequest, "application requires approval to link to group")
		return
	}

	// if the application doesn't require approval we'll just add the relationship in the database
	groupApp := &models.GroupApplication{
		GroupID:       group.ID,
		ApplicationID: app.ID,
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group application update transaction: "+err.Error())
		return
	}

	if err := group.AddGroupApplications(c.Request.Context(), tx, true, groupApp); err != nil {
		sendError(c, http.StatusBadRequest, "failed to update group application: "+err.Error())
		return
	}

	event, err := dbtools.AuditGroupApplicationCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), groupApp)
	if err != nil {
		msg := "error updating group applications (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating group applications (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group application update, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinksEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventCreate,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		GroupID:       group.ID,
		ApplicationID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application link create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// removeGroupApplication removes an application link from a group
func (r *Router) removeGroupApplication(c *gin.Context) {
	gid := c.Param("id")
	oid := c.Param("oid")

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

	qo := qm.Where("id = ?", oid)

	if _, err := uuid.Parse(oid); err != nil {
		qo = qm.Where("slug = ?", oid)
	}

	app, err := models.Applications(qo).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

		return
	}

	groupApp, err := models.GroupApplications(
		qm.Where("group_id=?", group.ID),
		qm.And("application_id=?", app.ID),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application not linked to group")
			return
		}

		sendError(c, http.StatusInternalServerError, "error checking application link: "+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group application delete transaction: "+err.Error())
		return
	}

	if _, err := groupApp.Delete(c.Request.Context(), tx, false); err != nil {
		msg := "failed to delete group application link: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupApplicationDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), groupApp)
	if err != nil {
		msg := "error deleting group application (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting group applications (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group application delete, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinksEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventDelete,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		GroupID:       group.ID,
		ApplicationID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application link delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// createGroupAppRequest creates a request to link application to a group
func (r *Router) createGroupAppRequest(c *gin.Context) {
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

	req := struct {
		ApplicationID string `json:"application_id"`
		Note          string `json:"note"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.ApplicationID == "" {
		sendError(c, http.StatusBadRequest, "application_id is required")
		return
	}

	if _, err := uuid.Parse(req.ApplicationID); err != nil {
		sendError(c, http.StatusBadRequest, "invalid application_id format")
		return
	}

	queryMods := []qm.QueryMod{
		qm.Load("GroupApplications"),
		qm.Load("GroupApplicationRequests"),
	}

	queryMods = append(queryMods, qm.Where("id = ?", req.ApplicationID))

	app, err := models.Applications(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusBadRequest, "requested application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

		return
	}

	// if the application doesn't require approval we'll return an error
	if app.ApproverGroupID.IsZero() {
		sendError(c, http.StatusBadRequest, "application doesn't require approval to link to group")
		return
	}

	for _, r := range app.R.GroupApplications {
		if r.GroupID == group.ID {
			sendError(c, http.StatusConflict, "application already linked to group")
			return
		}
	}

	for _, r := range app.R.GroupApplicationRequests {
		if r.GroupID == group.ID {
			sendError(c, http.StatusConflict, "there is a pending request for this application/group")
			return
		}
	}

	groupAppReq := &models.GroupApplicationRequest{
		GroupID:         group.ID,
		ApplicationID:   app.ID,
		ApproverGroupID: app.ApproverGroupID.String,
		RequesterUserID: ctxUser.ID,
		Note:            null.StringFrom(req.Note),
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group application request transaction: "+err.Error())
		return
	}

	if err := group.AddGroupApplicationRequests(c.Request.Context(), tx, true, groupAppReq); err != nil {
		msg := "failed to create group application request: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupApplicationRequestCreated(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, groupAppReq)
	if err != nil {
		msg := "error creating group application request (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error group application request (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group application request, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinkRequestsEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventCreate,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		GroupID:       groupAppReq.GroupID,
		ApplicationID: groupAppReq.ApplicationID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application link request create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// deleteGroupAppRequest deletes/revokes a pending request to link application to a group.
// This can only be done by the user who created the request.
func (r *Router) deleteGroupAppRequest(c *gin.Context) {
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

	request, err := models.GroupApplicationRequests(qm.Where("id = ?", rid)).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group application request not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group application request"+err.Error())

		return
	}

	if request.GroupID != group.ID {
		sendError(c, http.StatusBadRequest, "application request not associated with this group")
		return
	}

	if request.RequesterUserID != ctxUser.ID {
		sendError(c, http.StatusBadRequest, "application request not associated with this user")
		return
	}

	appID := request.ApplicationID

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete group application request transaction: "+err.Error())
		return
	}

	if _, err := request.Delete(c.Request.Context(), tx); err != nil {
		msg := "failed to delete group application request: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupApplicationRequestRevoked(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, request)
	if err != nil {
		msg := "error revoking group application request (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error revoking group application request (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group application request delete, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinkRequestsEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventRevoke,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		GroupID:       group.ID,
		ApplicationID: appID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application link request revoke event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// getGroupAppRequests returns all pending requests to link an application to a group.
// This will return requests associated with either the requesting or approving group.
func (r *Router) getGroupAppRequests(c *gin.Context) {
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

	queryMods := []qm.QueryMod{
		qm.Load("Application"),
		qm.Load("Group"),
		qm.Load("ApproverGroup"),
		qm.Load("RequesterUser"),
	}

	qmGroupID := qm.Where("group_id = ?", group.ID)
	qmApproverGroupID := qm.Or("approver_group_id = ?", group.ID)

	queryMods = append(queryMods, qm.Expr(qmGroupID, qmApproverGroupID))

	appRequests, err := models.GroupApplicationRequests(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group application request not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group application request"+err.Error())

		return
	}

	requests := make([]GroupApplicationRequest, len(appRequests))
	for i, m := range appRequests {
		requests[i] = GroupApplicationRequest{
			ID:                     m.ID,
			ApplicationID:          m.ApplicationID,
			ApplicationName:        m.R.Application.Name,
			ApplicationSlug:        m.R.Application.Slug,
			ApproverGroupID:        m.ApproverGroupID,
			ApproverGroupName:      m.R.ApproverGroup.Name,
			ApproverGroupSlug:      m.R.ApproverGroup.Slug,
			GroupID:                m.GroupID,
			GroupName:              m.R.Group.Name,
			GroupSlug:              m.R.Group.Slug,
			RequesterUserID:        m.RequesterUserID,
			RequesterUserName:      m.R.RequesterUser.Name,
			RequesterUserEmail:     m.R.RequesterUser.Email,
			RequesterUserAvatarURL: m.R.RequesterUser.AvatarURL.String,
			Note:                   m.Note.String,
			CreatedAt:              m.CreatedAt,
			UpdatedAt:              m.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, requests)
}

// processGroupAppRequest approves or denies a pending request to link an application to a group.
// This can only be done by a member of the approver group for the application.
//
//nolint:gocyclo
func (r *Router) processGroupAppRequest(c *gin.Context) {
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

	request, err := models.GroupApplicationRequests(qm.Where("id = ?", rid)).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group application request not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group application request"+err.Error())

		return
	}

	if request.GroupID != group.ID && request.ApproverGroupID != group.ID {
		sendError(c, http.StatusBadRequest, "application request not associated with this group")
		return
	}

	if request.RequesterUserID == ctxUser.ID {
		sendError(c, http.StatusBadRequest, "unable to approve/deny own request")
		return
	}

	app, err := models.FindApplication(c.Request.Context(), r.DB, request.ApplicationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusBadRequest, "application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application "+err.Error())

		return
	}

	if app.ApproverGroupID.IsZero() {
		sendError(c, http.StatusBadRequest, "application doesn't require approval")
		return
	}

	if request.ApproverGroupID != app.ApproverGroupID.String {
		sendError(c, http.StatusBadRequest, "application request approver group doesn't match application approver group")
		return
	}

	// check that the authenticated user is member of the approver group
	isApprover := false

	enumeratedMemberships, err := dbtools.GetMembershipsForUser(c.Request.Context(), r.DB.DB, ctxUser.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	for _, m := range enumeratedMemberships {
		if request.ApproverGroupID == m.GroupID {
			isApprover = true
			break
		}
	}

	if !isApprover {
		sendError(c, http.StatusUnauthorized, "user not member of approver group")
		return
	}

	req := struct {
		Action string `json:"action"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Action == "" {
		sendError(c, http.StatusBadRequest, "missing action: expecting approve or deny")
		return
	}

	switch req.Action {
	case "approve":
		// approving a request will first check that the application is not already associated
		// with the group, then link it to the group, and finally delete the request
		exists, err := models.GroupApplications(
			qm.Where("group_id = ?", request.GroupID),
			qm.And("application_id = ?", request.ApplicationID),
		).Exists(c.Request.Context(), r.DB)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "error checking application membership exists: "+err.Error())
			return
		}

		if exists {
			// if the application is already linked to the group, we can just delete the request
			if _, err := request.Delete(c.Request.Context(), r.DB); err != nil {
				sendError(c, http.StatusBadRequest, "failed to delete group application request: "+err.Error())
				return
			}

			sendError(c, http.StatusConflict, "application already linked to group")

			return
		}

		groupApp := &models.GroupApplication{
			GroupID:       request.GroupID,
			ApplicationID: request.ApplicationID,
		}

		tx, err := r.DB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			sendError(c, http.StatusBadRequest, "error starting group application approval transaction: "+err.Error())
			return
		}

		if err := groupApp.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
			msg := "error approving group application request, rolling back: " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if _, err := request.Delete(c.Request.Context(), tx); err != nil {
			msg := "error deleting group application request on approval, rolling back: " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		event, err := dbtools.AuditGroupApplicationApproved(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, groupApp)
		if err != nil {
			msg := "error approving group application request (audit): " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := updateContextWithAuditEventData(c, event); err != nil {
			msg := "error approving group application request (audit): " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := tx.Commit(); err != nil {
			msg := "error committing group application approval, rolling back: " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinkRequestsEventSubject, &events.Event{
			Version:       events.Version,
			Action:        events.GovernorEventApprove,
			AuditID:       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:       getCtxActorID(c),
			GroupID:       groupApp.GroupID,
			ApplicationID: groupApp.ApplicationID,
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish application link request approve event, downstream changes may be delayed "+err.Error())
			return
		}

		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinksEventSubject, &events.Event{
			Version:       events.Version,
			Action:        events.GovernorEventCreate,
			AuditID:       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:       getCtxActorID(c),
			GroupID:       groupApp.GroupID,
			ApplicationID: groupApp.ApplicationID,
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish group application link create event, downstream changes may be delayed "+err.Error())
			return
		}

		c.JSON(http.StatusNoContent, nil)

		return

	case "deny":
		tx, err := r.DB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			sendError(c, http.StatusBadRequest, "error starting group application denial transaction: "+err.Error())
			return
		}

		// denying a request simply deletes it
		if _, err := request.Delete(c.Request.Context(), tx); err != nil {
			sendError(c, http.StatusBadRequest, "failed to delete group application request: "+err.Error())
			return
		}

		event, err := dbtools.AuditGroupApplicationDenied(c.Request.Context(), tx, getCtxAuditID(c), ctxUser, request)
		if err != nil {
			msg := "error denying group application request (audit): " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := tx.Commit(); err != nil {
			msg := "error committing group application deny, rolling back: " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := updateContextWithAuditEventData(c, event); err != nil {
			msg := "error denying group application request (audit): " + err.Error()
			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)

			return
		}

		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationLinkRequestsEventSubject, &events.Event{
			Version:       events.Version,
			Action:        events.GovernorEventDeny,
			AuditID:       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:       getCtxActorID(c),
			GroupID:       request.GroupID,
			ApplicationID: request.ApplicationID,
		}); err != nil {
			sendError(c, http.StatusBadRequest, "failed to publish application link request deny event, downstream changes may be delayed "+err.Error())
			return
		}

		c.JSON(http.StatusNoContent, nil)

		return

	default:
		sendError(c, http.StatusBadRequest, "invalid action "+req.Action)
		return
	}
}
