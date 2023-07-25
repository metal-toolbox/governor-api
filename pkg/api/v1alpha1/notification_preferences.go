package v1alpha1

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

// handleUpdateNotificationPreferencesRequests handles all notification preferences
// update requests, including those originated from `/users/:id` or `/user`
func handleUpdateNotificationPreferencesRequests(
	c *gin.Context,
	db *sqlx.DB,
	user *models.User,
	eb *eventbus.Client,
	req dbtools.UserNotificationPreferences,
) (dbtools.UserNotificationPreferences, int, error) {
	if len(req) > 0 {
		tx, err := db.BeginTx(c.Request.Context(), nil)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("error starting update transaction: %s", err.Error())
		}

		event, err := dbtools.CreateOrUpdateNotificationPreferences(
			c.Request.Context(),
			user,
			req,
			tx,
			db,
			getCtxAuditID(c),
			getCtxUser(c),
		)

		if err != nil {
			msg := "error updating user notification preferences: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			return nil, http.StatusBadRequest, fmt.Errorf(msg)
		}

		if err := updateContextWithAuditEventData(c, event); err != nil {
			msg := "error updating notification preferences (audit): " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg += "error rolling back transaction: " + err.Error()
			}

			return nil, http.StatusBadRequest, fmt.Errorf(msg)
		}

		if err := tx.Commit(); err != nil {
			msg := "error committing user update, rolling back: " + err.Error()

			if err := tx.Rollback(); err != nil {
				msg = msg + "error rolling back transaction: " + err.Error()
			}

			return nil, http.StatusBadRequest, fmt.Errorf(msg)
		}
	}

	np, err := dbtools.GetNotificationPreferences(c.Request.Context(), user.ID, db, true)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error fetching notification preferences: %s", err.Error())
	}

	// only publish events for active users
	if !isActiveUser(user) {
		return np, http.StatusAccepted, nil
	}

	if err := eb.Publish(c.Request.Context(), events.GovernorNotificationPreferencesEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  user.ID,
	}); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to publish user delete event, downstream changes may be delayed %s", err.Error())
	}

	return np, http.StatusAccepted, nil
}

// getUserNotificationPreferences returns the user's notification preferences
func (r *Router) getUserNotificationPreferences(c *gin.Context) {
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

	np, err := dbtools.GetNotificationPreferences(c.Request.Context(), user.ID, r.DB, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting notification preferences: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, np)
}

// getUserNotificationPreferences returns the authenticated user's notification
// preferences
func (r *Router) getAuthenticatedUserNotificationPreferences(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	np, err := dbtools.GetNotificationPreferences(c.Request.Context(), ctxUser.ID, r.DB, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting notification preferences: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, np)
}

// updateUserNotificationPreferences is the http handler for
// /users/:id/notification-preferences
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

	np, code, err := handleUpdateNotificationPreferencesRequests(c, r.DB, user, r.EventBus, req)
	if err != nil {
		sendError(c, code, err.Error())
		return
	}

	c.JSON(code, np)
}

// updateUserNotificationPreferences is the http handler for
// /user/notification-preferences
func (r *Router) updateAuthenticatedUserNotificationPreferences(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	req := dbtools.UserNotificationPreferences{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	np, code, err := handleUpdateNotificationPreferencesRequests(c, r.DB, ctxUser, r.EventBus, req)
	if err != nil {
		sendError(c, code, err.Error())
		return
	}

	c.JSON(code, np)
}
