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
