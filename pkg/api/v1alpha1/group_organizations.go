package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

// addGroupOrganization links an organization to a group
func (r *Router) addGroupOrganization(c *gin.Context) {
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

	org, err := models.Organizations(qo).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "organization not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting organization"+err.Error())

		return
	}

	exists, err := models.GroupOrganizations(qm.Where("group_id=?", group.ID), qm.And("organization_id=?", org.ID)).Exists(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error checking group organization link exists "+err.Error())
		return
	}

	if exists {
		sendError(c, http.StatusConflict, "organization already linked with group")
		return
	}

	// Add the relationship in the database
	groupOrg := &models.GroupOrganization{
		GroupID:        group.ID,
		OrganizationID: org.ID,
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group organization create transaction: "+err.Error())
		return
	}

	if err := group.AddGroupOrganizations(c.Request.Context(), tx, true, groupOrg); err != nil {
		msg := "failed to create group organization: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupOrganizationCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), groupOrg)
	if err != nil {
		msg := "error creating group organization (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating group organization (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group organization create, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorGroupsEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: group.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish group update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// removeGroupOrganization removes an organization link from a group
func (r *Router) removeGroupOrganization(c *gin.Context) {
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

	org, err := models.Organizations(qo).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "organization not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting organization"+err.Error())

		return
	}

	groupOrg, err := models.GroupOrganizations(
		qm.Where("group_id=?", group.ID),
		qm.And("organization_id=?", org.ID),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "organization not linked to group")
			return
		}

		sendError(c, http.StatusInternalServerError, "error checking organization link: "+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group organization delete transaction: "+err.Error())
		return
	}

	if _, err := groupOrg.Delete(c.Request.Context(), r.DB); err != nil {
		msg := "failed to delete group organization link: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupOrganizationDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), groupOrg)
	if err != nil {
		msg := "error deleting group organization (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting group organization (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group organization delete, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorGroupsEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: group.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish group update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
