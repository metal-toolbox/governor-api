package v1alpha1

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"go.uber.org/zap"
)

// NotificationType is the notification type response
type NotificationType struct {
	*models.NotificationType
}

// NotificationTypeReq is a request to create a notification type
type NotificationTypeReq struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	DefaultEnabled *bool  `json:"default_enabled"`
}

// listNotificationTypes lists notification types as JSON
func (r *Router) listNotificationTypes(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	notificationTypes, err := models.NotificationTypes(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching notification types", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing notification types: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, notificationTypes)
}

// createNotificationType creates a notification type in DB
func (r *Router) createNotificationType(c *gin.Context) {
	req := &NotificationTypeReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "notification type name is required")
		return
	}

	if req.Description == "" {
		sendError(c, http.StatusBadRequest, "notification type description is required")
		return
	}

	if req.DefaultEnabled == nil {
		sendError(c, http.StatusBadRequest, "notification type default enabled is required")
		return
	}

	notificationType := &models.NotificationType{
		Name:           req.Name,
		Description:    req.Description,
		DefaultEnabled: *req.DefaultEnabled,
	}

	notificationType.Slug = slug.Make(notificationType.Name)

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting notification type create transaction: "+err.Error())
		return
	}

	if err := notificationType.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error creating notification type: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditNotificationTypeCreated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		notificationType,
	)
	if err != nil {
		msg := fmt.Sprintf("error creating notification type (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating notification type: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing notification type create: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// CRDB cannot refresh a materialized view inside explicit transactions
	// https://www.cockroachlabs.com/docs/stable/views#known-limitations
	if err := dbtools.RefreshNotificationDefaults(c.Request.Context(), r.DB); err != nil {
		sendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorNotificationTypesEventSubject,
		&events.Event{
			Version:            events.Version,
			Action:             events.GovernorEventCreate,
			AuditID:            c.GetString(ginaudit.AuditIDContextKey),
			ActorID:            getCtxActorID(c),
			NotificationTypeID: notificationType.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish notification type create event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, notificationType)
}

// getNotificationType fetch a notification type from DB with given id
func (r *Router) getNotificationType(c *gin.Context) {
	queryMods := []qm.QueryMod{}
	id := c.Param("id")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted notification type by slug, use the id")
			return
		}

		q = qm.Where("slug = ?", id)
	}

	queryMods = append(queryMods, q)

	notificationType, err := models.NotificationTypes(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "notification type not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting notification type"+err.Error())

		return
	}

	c.JSON(http.StatusOK, NotificationType{notificationType})
}

// deleteNotificationType marks a notification type deleted
func (r *Router) deleteNotificationType(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)
	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	n, err := models.NotificationTypes(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "notification type not found"+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting notification type: "+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	if _, err := n.Delete(c.Request.Context(), tx, false); err != nil {
		msg := fmt.Sprintf("error deleting notification type: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditNotificationTypeDeleted(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		n,
	)
	if err != nil {
		msg := fmt.Sprintf("error deleting notification type (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error deleting notification type: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing notification type delete: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := dbtools.RefreshNotificationDefaults(c.Request.Context(), r.DB); err != nil {
		sendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorNotificationTypesEventSubject,
		&events.Event{
			Version:            events.Version,
			Action:             events.GovernorEventDelete,
			AuditID:            c.GetString(ginaudit.AuditIDContextKey),
			ActorID:            getCtxActorID(c),
			NotificationTypeID: n.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish notification type delete event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, n)
}

// updateNotificationType updates a notification type in DB
func (r *Router) updateNotificationType(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)
	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	n, err := models.NotificationTypes(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "notification type not found"+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting notification type: "+err.Error())

		return
	}

	original := *n

	req := &NotificationTypeReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name != "" {
		sendError(c, http.StatusBadRequest, "modifying notification type name is not allowed")
		return
	}

	if req.Description != "" {
		n.Description = req.Description
	}

	if req.DefaultEnabled == nil {
		sendError(c, http.StatusBadRequest, "notification type default enabled is required")
		return
	}

	n.DefaultEnabled = *req.DefaultEnabled

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	if _, err := n.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error updating notification type: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditNotificationTypeUpdated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		&original,
		n,
	)
	if err != nil {
		msg := fmt.Sprintf("error updating notification type (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating notification type: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing notification type update: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := dbtools.RefreshNotificationDefaults(c.Request.Context(), r.DB); err != nil {
		sendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorNotificationTypesEventSubject,
		&events.Event{
			Version:            events.Version,
			Action:             events.GovernorEventUpdate,
			AuditID:            c.GetString(ginaudit.AuditIDContextKey),
			ActorID:            getCtxActorID(c),
			NotificationTypeID: n.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish notification type update event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, n)
}
