package v1alpha1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	"go.uber.org/zap"
)

const (
	contextKeyERD       = "gin-contextkey/extension-resource-definition"
	contextKeyExtension = "gin-contextkey/extension"
)

func setCtxERD(c *gin.Context, u *models.ExtensionResourceDefinition) {
	c.Set(contextKeyERD, u)
}

func getCtxERD(c *gin.Context) *models.ExtensionResourceDefinition {
	val, ok := c.Get(contextKeyERD)
	if !ok {
		return nil
	}

	erd, ok := val.(*models.ExtensionResourceDefinition)
	if !ok {
		return nil
	}

	return erd
}

func setCtxExtension(c *gin.Context, u *models.Extension) {
	c.Set(contextKeyExtension, u)
}

func getCtxExtension(c *gin.Context) *models.Extension {
	val, ok := c.Get(contextKeyExtension)
	if !ok {
		return nil
	}

	ext, ok := val.(*models.Extension)
	if !ok {
		return nil
	}

	return ext
}

func (r *Router) mwSystemExtensionResourceGroupAuth(c *gin.Context) {
	if !contains(c.GetStringSlice("jwt.roles"), oidcScope) {
		r.Logger.Debug("oidc scope not found, skipping user authorization check", zap.String("oidcScope", oidcScope))
		return
	}

	user := getCtxUser(c)
	if user == nil {
		r.Logger.Error("user not found in context")
		sendError(c, http.StatusUnauthorized, "invalid user")

		return
	}

	isGovAdmin := getCtxAdmin(c)
	if isGovAdmin != nil && *isGovAdmin {
		r.Logger.Debug("user is gov admin")
		return
	}

	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

	r.Logger.Debug(
		"mwSystemExtensionResourceGroupAuth",
		zap.String("extension-slug", extensionSlug),
		zap.String("erd-slug-plural", erdSlugPlural),
		zap.String("erd-version", erdVersion),
	)

	// find ERD
	ext, erd, err := findERDForExtensionResource(
		c, r.DB,
		extensionSlug, erdSlugPlural, erdVersion,
	)
	if err != nil {
		if errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, err.Error())
			return
		}

		sendError(c, http.StatusBadRequest, err.Error())

		return
	}

	setCtxExtension(c, ext)
	setCtxERD(c, erd)

	// if user is not gov-admin and there's no admin group set for the ERD
	if !erd.AdminGroup.Valid || erd.AdminGroup.String == "" {
		sendError(c, http.StatusForbidden, "user do not have permissions to access this resource")

		return
	}

	adminGroupID := erd.AdminGroup.String

	// check if user is part of the admin group
	enumeratedMemberships, err := dbtools.GetMembershipsForUser(c.Request.Context(), r.DB.DB, user.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting enumerated groups: "+err.Error())
		return
	}

	for _, m := range enumeratedMemberships {
		if m.GroupID == adminGroupID {
			return
		}
	}

	sendError(c, http.StatusForbidden, "user do not have permissions to access this resource")
}
