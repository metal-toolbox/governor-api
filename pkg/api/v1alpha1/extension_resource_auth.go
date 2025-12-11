package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// MWSystemExtensionResourceGroupAuth is a middleware that checks if the user is part of the
// extension resource definition (ERD) admin group or owner group.
// It uses the provided resourceOwnershipCheckFn to check if the user is part of the owner group.
// If the user is not part of either group, a 403 Forbidden response is returned.
// If the user is part of the gov-admins role, access is granted without further checks.
func MWSystemExtensionResourceGroupAuth(checkFn resourceOwnershipCheckFn, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "mwSystemExtensionResourceGroupAuth")
		defer span.End()

		if !contains(c.GetStringSlice("jwt.roles"), oidcScope) {
			span.AddEvent("oidc scope not found, skipping user authorization check")
			span.SetAttributes(attribute.String("oidcScope", oidcScope))

			return
		}

		user := getCtxUser(c)
		if user == nil {
			span.SetStatus(codes.Error, "user not found in context")
			sendError(c, http.StatusUnauthorized, "invalid user")

			return
		}

		// allow gov-admins
		isGovAdmin := getCtxAdmin(c)
		if isGovAdmin != nil && *isGovAdmin {
			span.SetAttributes(attribute.Bool("gov-admin", true))
			return
		}

		extensionSlug := c.Param("ex-slug")
		erdSlugPlural := c.Param("erd-slug-plural")
		erdVersion := c.Param("erd-version")

		span.SetAttributes(
			attribute.String("extensionSlug", extensionSlug),
			attribute.String("erdSlugPlural", erdSlugPlural),
			attribute.String("erdVersion", erdVersion),
		)

		// find ERD
		ext, erd, err := findERDForExtensionResource(
			c, db,
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
		enumeratedMemberships, err := dbtools.GetMembershipsForUser(ctx, db.DB, user.ID, false)
		if err != nil {
			span.SetStatus(codes.Error, "error getting enumerated groups")
			span.RecordError(err)
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
				span.SetAttributes(attribute.Bool("admin-group-member", true))
				return
			}
		}

		if err := checkFn(c, membershipSet, db); err == nil {
			span.SetAttributes(attribute.Bool("owner-group-member", true))
			return
		}

		switch {
		case errors.Is(err, sql.ErrNoRows):
			span.SetStatus(codes.Error, "resource not found")
			sendError(c, http.StatusNotFound, "resource not found")
		default:
			span.SetStatus(codes.Error, "user do not have permissions to access this resource")
			sendError(c, http.StatusForbidden, "user do not have permissions to access this resource")
		}
	}
}

// ExtResourceGroupAuthDenyAll is a resource ownership check function that denies all access
// only gov-admins and extension admin groups are allowed access
func ExtResourceGroupAuthDenyAll(
	*gin.Context, map[string]struct{}, boil.ContextExecutor,
) error {
	return errExtGroupAuthAccessDenied
}

// ExtResourceGroupAuthDBFetch is a resource ownership check function that fetches the
// extension resource from the database and checks if the user is part of the owner group
func ExtResourceGroupAuthDBFetch(
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
