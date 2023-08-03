package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

// UserNotificationPreferences is an alias export for the same struct in
// dbtools
type UserNotificationPreferences = dbtools.UserNotificationPreferences

// UserNotificationPreferenceTargets is an alias export for the same struct in
// dbtools
type UserNotificationPreferenceTargets = dbtools.UserNotificationPreferenceTargets

// handleUpdateNotificationPreferencesRequests handles all notification preferences
// update requests, including those originated from `/users/:id` or `/user`
func handleUpdateNotificationPreferencesRequests(
	c *gin.Context,
	ex boil.ContextExecutor,
	user *models.User,
	req UserNotificationPreferences,
) (UserNotificationPreferences, int, error) {
	event, err := dbtools.CreateOrUpdateNotificationPreferences(
		c.Request.Context(),
		user,
		req,
		ex,
		getCtxAuditID(c),
		getCtxUser(c),
	)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		return nil, http.StatusBadRequest, err
	}

	np, err := dbtools.GetNotificationPreferences(c.Request.Context(), user.ID, ex, true)
	if err != nil {
		return nil, http.StatusInternalServerError, err
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
// /user/notification-preferences
func (r *Router) updateAuthenticatedUserNotificationPreferences(c *gin.Context) {
	ctxUser := getCtxUser(c)
	if ctxUser == nil {
		sendError(c, http.StatusUnauthorized, "no user in context")
		return
	}

	req := UserNotificationPreferences{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	np, status, err := handleUpdateNotificationPreferencesRequests(c, tx, ctxUser, req)
	if err != nil {
		msg := err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, status, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing notification preferences update, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += ("error rolling back transaction: " + err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	c.JSON(status, np)
}
