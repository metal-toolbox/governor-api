package v1alpha1

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"go.uber.org/zap"
)

// Extension is the extension response
type Extension struct {
	*models.Extension
}

// ExtensionReq is a request to create a extension
type ExtensionReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// listExtensions lists extensions as JSON
func (r *Router) listExtensions(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	extensions, err := models.Extensions(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching extensions", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing extensions: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, extensions)
}

// createExtension creates a extension in DB
func (r *Router) createExtension(c *gin.Context) {
	req := &ExtensionReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "extension name is required")
		return
	}

	if req.Description == "" {
		sendError(c, http.StatusBadRequest, "extension description is required")
		return
	}

	if req.Enabled == nil {
		sendError(c, http.StatusBadRequest, "extension enabled is required")
		return
	}

	extension := &models.Extension{
		Name:        req.Name,
		Description: req.Description,
		Enabled:     *req.Enabled,
	}

	extension.Slug = slug.Make(extension.Name)

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting extension create transaction: "+err.Error())
		return
	}

	if err := extension.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error creating extension: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditExtensionCreated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		extension,
	)
	if err != nil {
		msg := fmt.Sprintf("error creating extension (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating extension: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension create: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorExtensionsEventSubject,
		&events.Event{
			Version:     events.Version,
			Action:      events.GovernorEventCreate,
			AuditID:     c.GetString(ginaudit.AuditIDContextKey),
			ActorID:     getCtxActorID(c),
			ExtensionID: extension.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension create event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, extension)
}

// getExtension fetch a extension from DB with given id
func (r *Router) getExtension(c *gin.Context) {
	queryMods := []qm.QueryMod{}
	id := c.Param("eid")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted extension by slug, use the id")
			return
		}

		q = qm.Where("slug = ?", id)
	}

	queryMods = append(queryMods, q)

	extension, err := models.Extensions(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "extension not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting extension"+err.Error())

		return
	}

	c.JSON(http.StatusOK, Extension{extension})
}

// deleteExtension marks a extension deleted
func (r *Router) deleteExtension(c *gin.Context) {
	id := c.Param("eid")

	q := qm.Where("id = ?", id)
	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	extension, err := models.Extensions(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "extension not found"+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting extension: "+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	if _, err := extension.Delete(c.Request.Context(), tx, false); err != nil {
		msg := fmt.Sprintf("error deleting extension: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditExtensionDeleted(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		extension,
	)
	if err != nil {
		msg := fmt.Sprintf("error deleting extension (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error deleting extension: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension delete: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorExtensionsEventSubject,
		&events.Event{
			Version:     events.Version,
			Action:      events.GovernorEventDelete,
			AuditID:     c.GetString(ginaudit.AuditIDContextKey),
			ActorID:     getCtxActorID(c),
			ExtensionID: extension.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension delete event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, extension)
}

// updateExtension updates a extension in DB
func (r *Router) updateExtension(c *gin.Context) {
	id := c.Param("eid")

	q := qm.Where("id = ?", id)
	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	extension, err := models.Extensions(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "extension not found"+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting extension: "+err.Error())

		return
	}

	original := *extension

	req := &ExtensionReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name != "" {
		sendError(c, http.StatusBadRequest, "modifying extension name is not allowed")
		return
	}

	if req.Description != "" {
		extension.Description = req.Description
	}

	if req.Enabled != nil {
		extension.Enabled = *req.Enabled
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	if _, err := extension.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error updating extension: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditExtensionUpdated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		&original,
		extension,
	)
	if err != nil {
		msg := fmt.Sprintf("error updating extension (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating extension: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension update: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorExtensionsEventSubject,
		&events.Event{
			Version:     events.Version,
			Action:      events.GovernorEventUpdate,
			AuditID:     c.GetString(ginaudit.AuditIDContextKey),
			ActorID:     getCtxActorID(c),
			ExtensionID: extension.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension update event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, extension)
}
