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
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"go.equinixmetal.net/governor-api/internal/dbtools"
	"go.equinixmetal.net/governor-api/internal/models"
	events "go.equinixmetal.net/governor-api/pkg/events/v1alpha1"
	"go.uber.org/zap"
)

// NotificationTarget is the notification target response
type NotificationTarget struct {
	*models.NotificationTarget
}

// NotificationTargetReq is a request to create a notification target
type NotificationTargetReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// listNotificationTargets lists notification targets as JSON
func (r *Router) listNotificationTargets(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	notificationTargets, err := models.NotificationTargets(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching notification targets", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing notification targets: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, notificationTargets)
}

// createNotificationTarget creates a notification target in DB
func (r *Router) createNotificationTarget(c *gin.Context) {
	req := &NotificationTargetReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "notification target name is required")
		return
	}

	if req.Description == "" {
		sendError(c, http.StatusBadRequest, "notification target description is required")
		return
	}

	notificationTarget := &models.NotificationTarget{
		Name:        req.Name,
		Description: req.Description,
	}

	notificationTarget.Slug = slug.Make(notificationTarget.Name)

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting notification target create transaction: "+err.Error())
		return
	}

	if err := notificationTarget.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error creating notification target: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	event, err := dbtools.AuditNotificationTargetCreated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		notificationTarget,
	)
	if err != nil {
		msg := fmt.Sprintf("error creating notification target (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating notification target: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing notification target create: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorNotificationTargetsEventSubject,
		&events.Event{
			Version:              events.Version,
			Action:               events.GovernorEventCreate,
			AuditID:              c.GetString(ginaudit.AuditIDContextKey),
			ActorID:              getCtxActorID(c),
			NotificationTargetID: notificationTarget.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish notification target create event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)
		return
	}

	c.JSON(http.StatusAccepted, notificationTarget)
}

// getNotificationTarget fetch a notification target from DB with given id
func (r *Router) getNotificationTarget(c *gin.Context) {
	queryMods := []qm.QueryMod{}
	id := c.Param("id")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted notification target by slug, use the id")
			return
		}

		q = qm.Where("slug = ?", id)
	}

	queryMods = append(queryMods, q)

	notificationTarget, err := models.NotificationTargets(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "notification target not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting notification target"+err.Error())

		return
	}

	c.JSON(http.StatusOK, NotificationTarget{notificationTarget})
}

// deleteNotificationTarget marks a notification target deleted
func (r *Router) deleteNotificationTarget(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)
	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	n, err := models.NotificationTargets(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "notification target not found"+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting notification target: "+err.Error())
		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	if _, err := n.Delete(c.Request.Context(), tx, false); err != nil {
		msg := fmt.Sprintf("error deleting notification target: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	event, err := dbtools.AuditNotificationTargetDeleted(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		n,
	)
	if err != nil {
		msg := fmt.Sprintf("error deleting notification target (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error deleting notification target: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing notification target delete: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorNotificationTargetsEventSubject,
		&events.Event{
			Version:              events.Version,
			Action:               events.GovernorEventDelete,
			AuditID:              c.GetString(ginaudit.AuditIDContextKey),
			ActorID:              getCtxActorID(c),
			NotificationTargetID: n.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish notification target delete event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)
		return
	}

	c.JSON(http.StatusAccepted, n)
}

// updateNotificationTarget updates a notification target in DB
func (r *Router) updateNotificationTarget(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)
	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	n, err := models.NotificationTargets(q).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "notification target not found"+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting notification target: "+err.Error())
		return
	}

	original := *n

	req := &NotificationTargetReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name != "" {
		sendError(c, http.StatusBadRequest, "modifying notification target name is not allowed")
		return
	}

	n.Description = req.Description

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	if _, err := n.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error updating notification target: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	event, err := dbtools.AuditNotificationTargetUpdated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		&original,
		n,
	)
	if err != nil {
		msg := fmt.Sprintf("error updating notification target (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating notification target: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing notification target update: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)
		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorNotificationTargetsEventSubject,
		&events.Event{
			Version:              events.Version,
			Action:               events.GovernorEventUpdate,
			AuditID:              c.GetString(ginaudit.AuditIDContextKey),
			ActorID:              getCtxActorID(c),
			NotificationTargetID: n.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish notification target update event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)
		return
	}

	c.JSON(http.StatusAccepted, n)
}
