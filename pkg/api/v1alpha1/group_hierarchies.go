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

// GroupHierarchy is the relationship between a parent group and a member group
type GroupHierarchy struct {
	ID              string    `json:"id"`
	ParentGroupID   string    `json:"parent_group_id"`
	ParentGroupSlug string    `json:"parent_group_slug"`
	MemberGroupID   string    `json:"member_group_id"`
	MemberGroupSlug string    `json:"member_group_slug"`
	ExpiresAt       null.Time `json:"expires_at"`
}

// listMemberGroups returns a list of member groups in a parent
func (r *Router) listMemberGroups(c *gin.Context) {
	gid := c.Param("id")

	queryMods := []qm.QueryMod{
		qm.Load(models.GroupHierarchyRels.ParentGroup),
		qm.Load(models.GroupHierarchyRels.MemberGroup),
	}

	q := qm.Where("parent_group_id = ?", gid)

	if _, err := uuid.Parse(gid); err != nil {
		sendError(c, http.StatusNotFound, "could not parse uuid: "+err.Error())

		return
	}

	queryMods = append(queryMods, q)

	groups, err := models.GroupHierarchies(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting member groups: "+err.Error())

		return
	}

	hierarchies := make([]GroupHierarchy, len(groups))
	for i, h := range groups {
		hierarchies[i] = GroupHierarchy{
			ID:              h.ID,
			ParentGroupID:   h.ParentGroupID,
			ParentGroupSlug: h.R.ParentGroup.Slug,
			MemberGroupID:   h.MemberGroupID,
			MemberGroupSlug: h.R.MemberGroup.Slug,
			ExpiresAt:       h.ExpiresAt,
		}
	}

	c.JSON(http.StatusOK, hierarchies)
}

// addMemberGroup adds a member group to a parent group
func (r *Router) addMemberGroup(c *gin.Context) {
	parentGroupID := c.Param("id")

	req := struct {
		ExpiresAt     null.Time `json:"expires_at"`
		MemberGroupID string    `json:"member_group_id"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	parentGroup, err := models.FindGroup(c.Request.Context(), r.DB, parentGroupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group"+err.Error())

		return
	}

	memberGroup, err := models.FindGroup(c.Request.Context(), r.DB, req.MemberGroupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group"+err.Error())

		return
	}

	exists, err := models.GroupHierarchies(
		qm.Where("parent_group_id = ?", parentGroup.ID),
		qm.And("member_group_id = ?", memberGroup.ID),
	).Exists(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error checking group hierarchy exists: "+err.Error())
		return
	}

	if exists {
		sendError(c, http.StatusConflict, "group is already a member")
		return
	}

	createsCycle, err := dbtools.CheckNewHierarchyWouldCreateCycle(c, r.DB.DB, parentGroup.ID, memberGroup.ID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "could not determine whether the desired hierarchy creates a cycle")
		return
	}

	if createsCycle {
		sendError(c, http.StatusBadRequest, "invalid relationship: hierarchy would create a cycle")
		return
	}

	groupHierarchy := &models.GroupHierarchy{
		ParentGroupID: parentGroup.ID,
		MemberGroupID: memberGroup.ID,
		ExpiresAt:     req.ExpiresAt,
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting add group hierarchy transaction: "+err.Error())
		return
	}

	membershipsBefore, err := dbtools.GetAllGroupMemberships(c, r.DB.DB, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := groupHierarchy.Insert(c.Request.Context(), r.DB, boil.Infer()); err != nil {
		msg := "failed to update group hierarchy: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupHierarchyCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), groupHierarchy)
	if err != nil {
		msg := "error creating groups hierarchy (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating groups hierarchy (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	membershipsAfter, err := dbtools.GetAllGroupMemberships(c, r.DB.DB, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing groups hierarchy, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	membersAdded := dbtools.FindMemberDiff(membershipsBefore, membershipsAfter)

	for _, enumeratedMembership := range membersAdded {
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

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorHierarchiesEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventCreate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		GroupID: parentGroupID,
		ActorID: getCtxActorID(c),
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish hierarchy create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// updateMemberGroup sets expiration on a group hierarchy
func (r *Router) updateMemberGroup(c *gin.Context) {
	parentGroupID := c.Param("id")
	memberGroupID := c.Param("member_id")

	hierarchy, err := models.GroupHierarchies(
		qm.Where("parent_group_id = ?", parentGroupID),
		qm.And("member_group_id = ?", memberGroupID),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "hierarchy not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting hierarchy"+err.Error())

		return
	}

	req := struct {
		ExpiresAt null.Time `json:"expires_at"`
	}{}

	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	hierarchy.ExpiresAt = req.ExpiresAt

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update hierarchy transaction: "+err.Error())
		return
	}

	if _, err := hierarchy.Update(c.Request.Context(), r.DB, boil.Infer()); err != nil {
		msg := "failed to update hierarchy: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	var event *models.AuditEvent

	event, err = dbtools.AuditGroupHierarchyUpdated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), hierarchy)
	if err != nil {
		msg := "error creating hierarchy (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating hierarchy (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing hierarchy update, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorHierarchiesEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		GroupID: hierarchy.ParentGroupID,
		ActorID: getCtxActorID(c),
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish hierarchy update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// removeGroupMember removes a user from a group
func (r *Router) removeMemberGroup(c *gin.Context) {
	parentGroupID := c.Param("id")
	memberGroupID := c.Param("member_id")

	hierarchy, err := models.GroupHierarchies(
		qm.Where("parent_group_id = ?", parentGroupID),
		qm.And("member_group_id = ?", memberGroupID),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "hierarchy not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting hierarchy"+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete group hierarchy transaction: "+err.Error())
		return
	}

	membershipsBefore, err := dbtools.GetAllGroupMemberships(c, r.DB.DB, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if _, err := hierarchy.Delete(c.Request.Context(), r.DB); err != nil {
		msg := "error removing hierarchy: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupHierarchyDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), hierarchy)
	if err != nil {
		msg := "error deleting groups hierarchy (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting group hierarchy (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	membershipsAfter, err := dbtools.GetAllGroupMemberships(c, r.DB.DB, false)
	if err != nil {
		msg := "failed to compute new effective memberships: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing hierarchy delete, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	membersAdded := dbtools.FindMemberDiff(membershipsAfter, membershipsBefore)

	for _, enumeratedMembership := range membersAdded {
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

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorHierarchiesEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventDelete,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		GroupID: parentGroupID,
		ActorID: getCtxActorID(c),
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish hierarchy delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// getGroupHierarchiesAll returns all group hierarchies for all groups
func (r *Router) getGroupHierarchiesAll(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.Load("ParentGroup"),
		qm.Load("MemberGroup"),
	}

	hierarchies, err := models.GroupHierarchies(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting group hierarchies: "+err.Error())

		return
	}

	hierarchiesResponse := make([]GroupHierarchy, len(hierarchies))
	for i, h := range hierarchies {
		hierarchiesResponse[i] = GroupHierarchy{
			ID:              h.ID,
			ParentGroupID:   h.ParentGroupID,
			ParentGroupSlug: h.R.ParentGroup.Slug,
			MemberGroupID:   h.MemberGroupID,
			MemberGroupSlug: h.R.MemberGroup.Slug,
			ExpiresAt:       h.ExpiresAt,
		}
	}

	c.JSON(http.StatusOK, hierarchiesResponse)
}
