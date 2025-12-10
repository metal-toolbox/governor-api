package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	"go.uber.org/zap"
)

const (
	contextKeyERD       = "gin-contextkey/extension-resource-definition"
	contextKeyExtension = "gin-contextkey/extension"
)

var errExtGroupAuthAccessDenied = errors.New("access denied")

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

type resourceOwnershipCheckFn func(
	c *gin.Context,
	groupMembershipSet map[string]struct{},
	db boil.ContextExecutor,
) error

func (r *Router) mwSystemExtensionResourceGroupAuth(checkFn resourceOwnershipCheckFn) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		// allow gov-admins
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

		// check if user is part of the admin group
		enumeratedMemberships, err := dbtools.GetMembershipsForUser(c.Request.Context(), r.DB.DB, user.ID, false)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "error getting enumerated groups: "+err.Error())
			return
		}

		membershipSet := make(map[string]struct{})
		for _, m := range enumeratedMemberships {
			membershipSet[m.GroupID] = struct{}{}
		}

		// allow if user is part of the admin group
		if erd.AdminGroup.Valid && erd.AdminGroup.String != "" {
			if _, ok := membershipSet[erd.AdminGroup.String]; ok {
				return
			}
		}

		if err := checkFn(c, membershipSet, r.DB); err == nil {
			return
		}

		switch {
		case errors.Is(err, sql.ErrNoRows):
			sendError(c, http.StatusNotFound, "resource not found")
			return
		default:
			r.Logger.Debug("resource ownership check failed", zap.Error(err))
			sendError(c, http.StatusForbidden, "user do not have permissions to access this resource")
		}
	}
}

// extResourceGroupAuthDenyAll is a resource ownership check function that denies all access
// only gov-admins and extension admin groups are allowed access
func extResourceGroupAuthDenyAll(
	*gin.Context, map[string]struct{}, boil.ContextExecutor,
) error {
	return errExtGroupAuthAccessDenied
}

// extResourceGroupAuthDBFetch is a resource ownership check function that fetches the
// extension resource from the database and checks if the user is part of the owner group
func extResourceGroupAuthDBFetch(
	c *gin.Context,
	groupMembershipSet map[string]struct{},
	exec boil.ContextExecutor,
) error {
	resourceID := c.Param("resource-id")

	er, err := models.SystemExtensionResources(qm.Where("id = ?", resourceID)).One(c.Request.Context(), exec)
	if err != nil {
		return err
	}

	if er.OwnerID.Valid && er.OwnerID.String != "" {
		if _, ok := groupMembershipSet[er.OwnerID.String]; ok {
			return nil
		}
	}

	return errExtGroupAuthAccessDenied
}
