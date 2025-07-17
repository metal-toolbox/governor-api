package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

// Application is the application response
type Application struct {
	*models.Application
	Type *models.ApplicationType `json:"type"`
}

// ApplicationReq is a request to create an application
type ApplicationReq struct {
	Name            string  `json:"name"`
	TypeID          string  `json:"type_id"`
	ApproverGroupID *string `json:"approver_group_id"`
}

// listApplications lists the application as JSON
func (r *Router) listApplications(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	apps, err := models.Applications(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching application", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing applications: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, apps)
}

// listApplicationGroups lists all groups associated with the application
func (r *Router) listApplicationGroups(c *gin.Context) {
	queryMods := []qm.QueryMod{}

	id := c.Param("id")
	aid := id

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted application by slug, use the id")
			return
		}

		// find the app by slug
		app, err := models.Applications(qm.Where("slug = ?", id)).One(c.Request.Context(), r.DB)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				sendError(c, http.StatusNotFound, "application not found: "+err.Error())
				return
			}

			sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

			return
		}

		aid = app.ID
	}

	queryMods = append(queryMods, qm.Where("application_id=?", aid))

	groupApps, err := models.GroupApplications(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching application groups", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing application groups: "+err.Error())

		return
	}

	gids := make([]interface{}, len(groupApps))
	for i, g := range groupApps {
		gids[i] = g.GroupID
	}

	groups, err := models.Groups(qm.WhereIn("id IN ?", gids...)).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching application groups", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing application groups: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, groups)
}

// getApplication gets an application and it's relationships
func (r *Router) getApplication(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.Load("Type"),
	}

	id := c.Param("id")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := []qm.QueryMod{qm.Where("id = ?", id)}

	if _, err := uuid.Parse(id); err != nil {
		typeID, typeExists := c.GetQuery("type_id")
		if !typeExists {
			sendError(c, http.StatusBadRequest, "type_id is required when fetching an application by slug")
			return
		}

		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted application by slug, use the id")
			return
		}

		q = []qm.QueryMod{
			qm.Where("slug = ?", id),
			qm.Where("type_id = ?", typeID),
		}
	}

	queryMods = append(queryMods, q...)

	app, err := models.Applications(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

		return
	}

	c.JSON(http.StatusOK, Application{
		Application: app,
		Type:        app.R.Type,
	})
}

// createApplication creates an application in the database
func (r *Router) createApplication(c *gin.Context) {
	req := ApplicationReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "application name is required")
		return
	}

	if req.TypeID == "" {
		sendError(c, http.StatusBadRequest, "application type_id is required")
		return
	}

	if req.TypeID != "" {
		if _, err := uuid.Parse(req.TypeID); err != nil {
			sendError(c, http.StatusBadRequest, "invalid type_id format")
			return
		}
	}

	app := &models.Application{
		Name:   req.Name,
		TypeID: null.StringFrom(req.TypeID),
	}

	if req.ApproverGroupID != nil {
		app.ApproverGroupID = null.String{}

		if *req.ApproverGroupID != "" {
			if _, err := uuid.Parse(*req.ApproverGroupID); err != nil {
				sendError(c, http.StatusBadRequest, "invalid approver_group_id format")
				return
			}

			app.ApproverGroupID = null.StringFromPtr(req.ApproverGroupID)
		}
	}

	dbtools.SetApplicationSlug(app)

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting application create transaction: "+err.Error())
		return
	}

	if err := app.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error creating application: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditApplicationCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), app)
	if err != nil {
		msg := "error creating application (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating application (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing application create, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationsEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventCreate,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		ApplicationID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, app)
}

// deleteApplication marks an application deleted
func (r *Router) deleteApplication(c *gin.Context) {
	id := c.Param("id")

	q := []qm.QueryMod{qm.Where("id = ?", id)}

	if _, err := uuid.Parse(id); err != nil {
		typeID, typeExists := c.GetQuery("type_id")
		if !typeExists {
			sendError(c, http.StatusBadRequest, "type_id is required when fetching an application by slug")
			return
		}

		q = []qm.QueryMod{
			qm.Where("slug = ?", id),
			qm.Where("type_id = ?", typeID),
		}
	}

	q = append(q, qm.Load("GroupApplications"))

	app, err := models.Applications(q...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	// delete all app links
	if _, err := app.R.GroupApplications.DeleteAll(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting group app link, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if _, err := app.Delete(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting application, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditApplicationDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), app)
	if err != nil {
		msg := "error deleting application (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting application (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing application delete, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationsEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventDelete,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		ApplicationID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, app)
}

// updateApplication updates an application
func (r *Router) updateApplication(c *gin.Context) {
	id := c.Param("id")

	q := []qm.QueryMod{qm.Where("id = ?", id)}

	if _, err := uuid.Parse(id); err != nil {
		typeID, typeExists := c.GetQuery("type_id")
		if !typeExists {
			sendError(c, http.StatusBadRequest, "type_id is required when fetching an application by slug")
			return
		}

		q = []qm.QueryMod{
			qm.Where("slug = ?", id),
			qm.Where("type_id = ?", typeID),
		}
	}

	app, err := models.Applications(q...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application"+err.Error())

		return
	}

	original := *app

	req := ApplicationReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.TypeID != "" {
		if _, err := uuid.Parse(req.TypeID); err != nil {
			sendError(c, http.StatusBadRequest, "invalid type_id format")
			return
		}

		app.TypeID = null.StringFrom(req.TypeID)
	}

	app.ApproverGroupID = null.String{}

	if *req.ApproverGroupID != "" {
		if _, err := uuid.Parse(*req.ApproverGroupID); err != nil {
			sendError(c, http.StatusBadRequest, "invalid approver_group_id format")
			return
		}

		app.ApproverGroupID = null.StringFromPtr(req.ApproverGroupID)
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting application update transaction: "+err.Error())
		return
	}

	if _, err := app.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error updating application: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditApplicationUpdated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, app)
	if err != nil {
		msg := "error updating application (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating application (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing application update, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationsEventSubject, &events.Event{
		Version:       events.Version,
		Action:        events.GovernorEventUpdate,
		AuditID:       c.GetString(ginaudit.AuditIDContextKey),
		ActorID:       getCtxActorID(c),
		ApplicationID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, app)
}
