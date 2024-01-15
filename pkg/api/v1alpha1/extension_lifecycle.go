package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"go.uber.org/zap"
)

func (r *Router) mwExtensionEnabledCheck(c *gin.Context) {
	extensionIDOrSlug := c.Param("eid")
	if extensionIDOrSlug == "" {
		extensionIDOrSlug = c.Param("ex-slug")
	}

	erdQMs := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	var extensionQM qm.QueryMod

	if _, err := uuid.Parse(extensionIDOrSlug); err != nil {
		extensionQM = qm.Where("slug = ?", extensionIDOrSlug)
	} else {
		extensionQM = qm.Where("id = ?", extensionIDOrSlug)
	}

	r.Logger.Debug("mwExtensionEnabledCheck", zap.String("extension-id-slug", extensionIDOrSlug))

	extension, err := models.Extensions(
		extensionQM, qm.Load(
			models.ExtensionRels.ExtensionResourceDefinitions,
			erdQMs...,
		),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "extension not found or deleted: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting extension"+err.Error())

		return
	}

	if !extension.Enabled {
		sendError(c, http.StatusBadRequest, "extension is disabled")
	}
}

func (r *Router) mwExtensionResourcesEnabledCheck(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

	r.Logger.Debug(
		"mwExtensionResourcesEnabledCheck",
		zap.String("extension-slug", extensionSlug),
		zap.String("erd-slug-plural", erdSlugPlural),
		zap.String("erd-version", erdVersion),
	)

	// find ERD
	ext, erd, err := findERDForExtensionResource(
		c.Request.Context(), r.DB,
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

	if !ext.Enabled {
		sendError(c, http.StatusBadRequest, "extension is disabled")
		return
	}

	if !erd.Enabled {
		sendError(c, http.StatusBadRequest, "extension resource definition is disabled")
		return
	}
}
