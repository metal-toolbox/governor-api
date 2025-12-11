package v1alpha1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	contextKeyERD               = "gin-contextkey/extension-resource-definition"
	contextKeyExtension         = "gin-contextkey/extension"
	contextKeyExtensionResource = "gin-contextkey/extension-resource"
)

var (
	errExtGroupAuthAccessDenied    = errors.New("access denied")
	errExtGroupAuthValidationError = errors.New("validation error")
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

func setCtxExtensionResource(c *gin.Context, r *ExtensionResource) {
	c.Set(contextKeyExtensionResource, r)
}

func getCtxExtensionResource(c *gin.Context) *ExtensionResource {
	val, ok := c.Get(contextKeyExtensionResource)
	if !ok {
		return nil
	}

	er, ok := val.(*ExtensionResource)
	if !ok {
		return nil
	}

	return er
}

type resourceOwnershipCheckFn func(
	c *gin.Context,
	groupMembershipSet map[string]struct{},
	db boil.ContextExecutor,
) error

// mwExtensionResourceGroupAuth is a middleware that checks if the user is part of the
// extension resource definition (ERD) admin group or owner group.
// It uses the provided resourceOwnershipCheckFn to check if the user is part of the owner group.
// If the user is not part of either group, a 403 Forbidden response is returned.
// If the user is part of the gov-admins role, access is granted without further checks.
func mwExtensionResourceGroupAuth(checkFn resourceOwnershipCheckFn, db *sqlx.DB) gin.HandlerFunc {
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

		erd := getCtxERD(c)
		if erd == nil {
			sendError(c, http.StatusInternalServerError, "extension resource definition was not set in context")
			return
		}

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

		if checkFn == nil {
			span.SetStatus(codes.Error, "no ownership check function provided")
			sendError(c, http.StatusForbidden, "user do not have permissions to access this resource")

			return
		}

		// allow if user is the owner of the resource
		if err := checkFn(c, membershipSet, db); err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				span.SetStatus(codes.Error, "resource not found")
				sendError(c, http.StatusNotFound, "resource not found")
			case errors.Is(err, errExtGroupAuthValidationError):
				span.SetStatus(codes.Error, "validation error")
				sendError(c, http.StatusBadRequest, err.Error())
			default:
				span.SetStatus(codes.Error, "user do not have permissions to access this resource")
				sendError(c, http.StatusForbidden, "user do not have permissions to access this resource")
			}
		}

		span.SetAttributes(attribute.Bool("owner-group-member", true))
	}
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

func extResourceGroupAuthOwnerRef(
	c *gin.Context,
	groupMembershipSet map[string]struct{},
	exec boil.ContextExecutor,
) error {
	res := getCtxExtensionResource(c)

	if res == nil {
		// Read the entire request body into memory
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return fmt.Errorf("%w: failed to read request body: %w", errExtGroupAuthValidationError, err)
		}

		// Reset the request body so subsequent handlers can read it
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		res = &ExtensionResource{}
		if err := json.Unmarshal(bodyBytes, res); err != nil {
			return fmt.Errorf("%w: %w", errExtGroupAuthValidationError, err)
		}

		setCtxExtensionResource(c, res)
	}

	switch res.Metadata.OwnerRef.Kind {
	case ExtensionResourceOwnerKindUser:
		user := getCtxUser(c)
		if user != nil && res.Metadata.OwnerRef.ID == user.ID {
			return nil
		}
	case ExtensionResourceOwnerKindGroup:
		if res.Metadata.OwnerRef.ID != "" {
			if _, ok := groupMembershipSet[res.Metadata.OwnerRef.ID]; ok {
				return nil
			}
		}
	}

	return errExtGroupAuthAccessDenied
}
