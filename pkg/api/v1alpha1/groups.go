package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"dario.cat/mergo"
	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

// Group is a group response
type Group struct {
	*models.Group
	Members            []string `json:"members,omitempty"`
	MembersDirect      []string `json:"members_direct,omitempty"`
	MembershipRequests []string `json:"membership_requests,omitempty"`
	Organizations      []string `json:"organizations"`
	Applications       []string `json:"applications"`
}

// permittedListGroupsParams is a list of permitted query parameters for listing groups
var permittedListGroupsParams = []string{"name", "slug", "metadata"}

// GroupReq is a group creation/update request
type GroupReq struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Note            string                  `json:"note"`
	ApproverGroupID string                  `json:"approver_group_id,omitempty"`
	Metadata        *map[string]interface{} `json:"metadata,omitempty"`
}

// listGroups lists the groups as JSON
func (r *Router) listGroups(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
		qm.Load("GroupOrganizations"),
		qm.Load("GroupOrganizations.Organization"),
		qm.Load("GroupApplications"),
		qm.Load("GroupApplications.Application"),
	}

	for k, val := range c.Request.URL.Query() {
		r.Logger.Debug("checking query", zap.String("url.query.key", k), zap.Strings("url.query.value", val))

		if k == "deleted" {
			queryMods = append(queryMods, qm.WithDeleted())
			continue
		}

		// check for allowed parameters
		if !contains(permittedListGroupsParams, k) {
			r.Logger.Warn("found illegal parameter in request", zap.String("parameter", k))

			sendError(c, http.StatusBadRequest, "illegal parameter: "+k)

			return
		}

		convertedVals := make([]interface{}, len(val))
		for i, v := range val {
			convertedVals[i] = v
		}

		switch k {
		case "metadata":
			mods, err := dbtools.ParseJSONBFilterQueries("metadata", val)
			if err != nil {
				r.Logger.Error("invalid metadata query", zap.Error(err))
				sendError(c, http.StatusBadRequest, err.Error())

				return
			}

			queryMods = append(queryMods, mods...)
		default:
			queryMods = append(queryMods, qm.Or2(qm.WhereIn(k+" IN ?", convertedVals...)))
		}
	}

	groups, err := models.Groups(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching groups", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing groups: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, groups)
}

// getGroup gets a group and it's relationships
func (r *Router) getGroup(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.Load("GroupMembershipRequests"),
		qm.Load("GroupMembershipRequests.User"),
		qm.Load("GroupOrganizations"),
		qm.Load("GroupOrganizations.Organization"),
		qm.Load("GroupApplications"),
		qm.Load("GroupApplications.Application"),
	}

	id := c.Param("id")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted group by slug, use the group id")
			return
		}

		q = qm.Where("slug = ?", id)
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

	enumeratedMembers, err := dbtools.GetMembersOfGroup(c.Request.Context(), r.DB.DB, group.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	members := make([]string, len(enumeratedMembers))

	membersDirect := make([]string, 0)

	for i, m := range enumeratedMembers {
		members[i] = m.UserID

		if m.Direct {
			membersDirect = append(membersDirect, m.UserID)
		}
	}

	requests := make([]string, len(group.R.GroupMembershipRequests))
	for i, r := range group.R.GroupMembershipRequests {
		requests[i] = r.R.User.ID
	}

	organizations := make([]string, len(group.R.GroupOrganizations))
	for i, o := range group.R.GroupOrganizations {
		organizations[i] = o.R.Organization.ID
	}

	applications := make([]string, len(group.R.GroupApplications))
	for i, o := range group.R.GroupApplications {
		applications[i] = o.R.Application.ID
	}

	c.JSON(http.StatusOK, Group{
		Group:              group,
		Members:            members,
		MembersDirect:      membersDirect,
		MembershipRequests: requests,
		Organizations:      organizations,
		Applications:       applications,
	})
}

func createGroupRequestValidator(group *models.Group) (string, error) {
	if group.Name == "" || group.Description == "" {
		return "field(s) cannot be empty", ErrEmptyInput
	}

	// only allow alphanumeric characters and (, ), [, ], &, -, ., (space).
	groupNameChecker := regexp.MustCompile(`^[A-Za-z0-9\(\)\[\]\s\&\-\.]+$`).MatchString
	if !groupNameChecker(group.Name) {
		return "only alphanumeric and (, ), [, ], &, -, ., (space) characters allowed", ErrInvalidChar
	}

	return "", nil
}

// createGroup creates a user in the database
func (r *Router) createGroup(c *gin.Context) {
	req := GroupReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	var approverGroupID null.String

	if req.ApproverGroupID != "" {
		approverGroupID = null.String{
			String: req.ApproverGroupID,
			Valid:  true,
		}
	}

	group := &models.Group{
		Description:   req.Description,
		Name:          strings.TrimSpace(req.Name),
		Note:          req.Note,
		ApproverGroup: approverGroupID,
	}

	group.Metadata = types.JSON{}

	if req.Metadata == nil {
		req.Metadata = &map[string]interface{}{}
	}

	if !dbtools.IsValidMetadata(*req.Metadata) {
		sendError(c, http.StatusBadRequest, "invalid metadata keys, must match pattern [a-zA-Z0-9_/]+")
		return
	}

	if err := group.Metadata.Marshal(req.Metadata); err != nil {
		sendError(c, http.StatusBadRequest, "error marshalling metadata: "+err.Error())
		return
	}

	// Validation
	if displayMessage, err := createGroupRequestValidator(group); err != nil {
		sendErrorWithDisplayMessage(c, http.StatusBadRequest, err.Error(), displayMessage)
		return
	}

	dbtools.SetGroupSlug(group)

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group create transaction: "+err.Error())
		return
	}

	if err := group.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error creating group: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), group)
	if err != nil {
		msg := "error creating group (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating group (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group create, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorGroupsEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventCreate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: group.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish group create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, group)
}

// updateGroup updates a group in the database
func (r *Router) updateGroup(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
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

	original := *group

	req := GroupReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	var approverGroupID null.String

	if req.ApproverGroupID != "" {
		approverGroupID = null.String{
			String: req.ApproverGroupID,
			Valid:  true,
		}
	}

	if req.Name != "" {
		req.Name = strings.TrimSpace(req.Name)
	}

	group.ApproverGroup = approverGroupID

	group.Description = req.Description

	if req.Metadata != nil {
		current := map[string]interface{}{}
		incoming := *req.Metadata

		if err := group.Metadata.Unmarshal(&current); err != nil {
			sendError(c, http.StatusBadRequest, "error unmarshalling group metadata: "+err.Error())
			return
		}

		// merge the new metadata with the existing one
		if err := mergo.Merge(
			&current, incoming,
			mergo.WithOverride,
			mergo.WithOverrideEmptySlice,
		); err != nil {
			sendError(c, http.StatusBadRequest, "error merging group metadata: "+err.Error())
			return
		}

		if !dbtools.IsValidMetadata(current) {
			sendError(c, http.StatusBadRequest, "invalid metadata keys, must match pattern [a-zA-Z0-9_/]+")
			return
		}

		if err := group.Metadata.Marshal(&current); err != nil {
			sendError(c, http.StatusBadRequest, "error marshalling group metadata: "+err.Error())
			return
		}
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting group update transaction: "+err.Error())
		return
	}

	if _, err := group.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error updating group: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupUpdated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, group)
	if err != nil {
		msg := "error updating group (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating group (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group update, rolling back: " + err.Error()
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

	c.JSON(http.StatusAccepted, group)
}

// deleteGroup marks a group deleted in the database
func (r *Router) deleteGroup(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	group, err := models.Groups(
		q,
		qm.Load("GroupMemberships"),
		qm.Load("GroupMembershipRequests"),
		qm.Load("GroupOrganizations"),
		qm.Load("GroupApplications"),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "group not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting group"+err.Error())

		return
	}

	original := *group

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	// delete all group memberships
	if _, err := group.R.GroupMemberships.DeleteAll(c.Request.Context(), tx); err != nil {
		msg := "error deleting group membership, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// delete all group membership requests
	if _, err := group.R.GroupMembershipRequests.DeleteAll(c.Request.Context(), tx); err != nil {
		msg := "error deleting group membership requests, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// delete all org links
	if _, err := group.R.GroupOrganizations.DeleteAll(c.Request.Context(), tx); err != nil {
		msg := "error deleting group org link, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	appLinks := group.R.GroupApplications

	// delete all app links
	if _, err := group.R.GroupApplications.DeleteAll(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting group app link, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// finally soft delete the db
	if _, err := group.Delete(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting group, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditGroupDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, group)
	if err != nil {
		msg := "error deleting group (audit: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting group (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing group delete, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	for _, app := range appLinks {
		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationsEventSubject, &events.Event{
			Version:       events.Version,
			Action:        events.GovernorEventDelete,
			AuditID:       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:       getCtxActorID(c),
			GroupID:       app.GroupID,
			ApplicationID: app.ApplicationID,
		}); err != nil {
			r.Logger.Warn("failed to publish application unlink event, downstream changes may be delayed", zap.Error(err))
			continue
		}
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorGroupsEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventDelete,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: group.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish group delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, group)
}
