package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

func (r *Router) getUserNotificationPreferences(c *gin.Context) {
	id := c.Param("id")
	np, err := dbtools.GetNotificationPreferences(c.Request.Context(), id, r.DB, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting notification preferences: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, np)
}

func (r *Router) updateUserNotificationPreferences(c *gin.Context) {
	id := c.Param("id")

	user, err := models.FindUser(c.Request.Context(), r.DB, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

		return
	}

	req := dbtools.UserNotificationPreferences{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if len(req) > 0 {
		tx, err := r.DB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
			return
		}

		event, err := dbtools.CreateOrUpdateNotificationPreferences(
			c.Request.Context(),
			user,
			req,
			tx,
			r.DB,
			getCtxAuditID(c),
			getCtxUser(c),
		)

		if err != nil {
			msg := "error updating user notification preferences: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)
			return
		}

		if err := updateContextWithAuditEventData(c, event); err != nil {
			msg := "error updating notification preferences (audit): " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)
			return
		}

		if err := tx.Commit(); err != nil {
			msg := "error committing user update, rolling back: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg = msg + "error rolling back transaction: " + err.Error()
			}

			sendError(c, http.StatusBadRequest, msg)
			return
		}
	}

	np, err := dbtools.GetNotificationPreferences(c.Request.Context(), user.ID, r.DB, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error fetching notification preferences: "+err.Error())
		return
	}

	// only publish events for active users
	if !isActiveUser(user) {
		c.JSON(http.StatusAccepted, np)
		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorNotificationPreferencesEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  user.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish user delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusOK, np)
}
