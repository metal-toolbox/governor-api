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

// ApplicationType is the application type response
type ApplicationType struct {
	*models.ApplicationType
}

// ApplicationTypeReq is a request to create an application type
type ApplicationTypeReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	LogoURL     *string `json:"logo_url,omitempty"`
}

// listApplications lists the application as JSON
func (r *Router) listApplicationTypes(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	apps, err := models.ApplicationTypes(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching application types", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing application types: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, apps)
}

// listApplicationTypeApps lists all applications associated with the application type
func (r *Router) listApplicationTypeApps(c *gin.Context) {
	queryMods := []qm.QueryMod{}

	id := c.Param("id")
	tid := id

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted application type by slug, use the id")
			return
		}

		// find the type by slug
		app, err := models.ApplicationTypes(qm.Where("slug = ?", id)).One(c.Request.Context(), r.DB)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				sendError(c, http.StatusNotFound, "application type not found: "+err.Error())
				return
			}

			sendError(c, http.StatusInternalServerError, "error getting application type"+err.Error())

			return
		}

		tid = app.ID
	}

	queryMods = append(queryMods, qm.Where("type_id=?", tid))

	typeApps, err := models.Applications(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching applications for type", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing applications for type: "+err.Error())

		return
	}

	aids := make([]interface{}, len(typeApps))
	for i, a := range typeApps {
		aids[i] = a.ID
	}

	apps, err := models.Applications(qm.WhereIn("id IN ?", aids...)).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching applications", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing application: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, apps)
}

// getApplicationType gets an application type and it's relationships
func (r *Router) getApplicationType(c *gin.Context) {
	queryMods := []qm.QueryMod{}

	id := c.Param("id")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted application type by slug, use the id")
			return
		}

		q = qm.Where("slug = ?", id)
	}

	queryMods = append(queryMods, q)

	app, err := models.ApplicationTypes(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application type not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application type"+err.Error())

		return
	}

	c.JSON(http.StatusOK, ApplicationType{
		ApplicationType: app,
	})
}

// createApplicationType creates an application type in the database
func (r *Router) createApplicationType(c *gin.Context) {
	req := ApplicationTypeReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "application type name is required")
		return
	}

	if req.Description == "" {
		sendError(c, http.StatusBadRequest, "application type description is required")
		return
	}

	app := &models.ApplicationType{
		Name:        req.Name,
		Description: req.Description,
	}

	dbtools.SetApplicationTypeSlug(app)

	if req.LogoURL != nil {
		app.LogoURL = null.StringFrom(*req.LogoURL)
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting application type create transaction: "+err.Error())
		return
	}

	if err := app.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error creating application type: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditApplicationTypeCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), app)
	if err != nil {
		msg := "error creating application type (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating application type (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing application type create, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationTypesEventSubject, &events.Event{
		Version:           events.Version,
		Action:            events.GovernorEventCreate,
		AuditID:           c.GetString(ginaudit.AuditIDContextKey),
		ActorID:           getCtxActorID(c),
		ApplicationTypeID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application type create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, app)
}

// deleteApplicationType marks an application type deleted
func (r *Router) deleteApplicationType(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	app, err := models.ApplicationTypes(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application type not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application type"+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	if _, err := app.Delete(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting application type, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditApplicationTypeDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), app)
	if err != nil {
		msg := "error deleting application type (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting application type (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing application type delete, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationTypesEventSubject, &events.Event{
		Version:           events.Version,
		Action:            events.GovernorEventDelete,
		AuditID:           c.GetString(ginaudit.AuditIDContextKey),
		ActorID:           getCtxActorID(c),
		ApplicationTypeID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application type delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, app)
}

// updateApplicationType updates an application type
func (r *Router) updateApplicationType(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	app, err := models.ApplicationTypes(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "application type not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting application type"+err.Error())

		return
	}

	original := *app

	req := ApplicationTypeReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	app.Description = req.Description

	if req.LogoURL != nil {
		app.LogoURL = null.String{}

		if *req.LogoURL != "" {
			app.LogoURL = null.StringFromPtr(req.LogoURL)
		}
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting application type update transaction: "+err.Error())
		return
	}

	if _, err := app.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error updating application type: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditApplicationTypeUpdated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, app)
	if err != nil {
		msg := "error updating application type (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating application type (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing application type update, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorApplicationTypesEventSubject, &events.Event{
		Version:           events.Version,
		Action:            events.GovernorEventUpdate,
		AuditID:           c.GetString(ginaudit.AuditIDContextKey),
		ActorID:           getCtxActorID(c),
		ApplicationTypeID: app.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish application type update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, app)
}
