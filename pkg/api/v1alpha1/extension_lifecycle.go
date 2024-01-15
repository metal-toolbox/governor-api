package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/governor-api/internal/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"go.uber.org/zap"
)

const (
	extensionCtxKey = "gin-contextkey/extension"
)

func saveExtensionToContext(c *gin.Context, extension *models.Extension) {
	c.Set(extensionCtxKey, extension)
}

func extractExtensionFromContext(c *gin.Context) *models.Extension {
	if extensionAny, ok := c.Get(extensionCtxKey); ok {
		return extensionAny.(*models.Extension)
	}

	return nil
}

// fetch extension is a helper function that retrieves an extension from the
// database using the provided query modifiers. The function saves the extension
// to the gin.Context object to ensure that no unnecessary database queries are
// called after the extension is loaded through out a request.
func fetchExtension(
	c *gin.Context, exec boil.ContextExecutor, qms ...qm.QueryMod,
) (extension *models.Extension, err error) {
	// skip if extension and ERD are already loaded
	if extension = extractExtensionFromContext(c); extension != nil {
		return
	}

	extension, err = models.Extensions(qms...).One(c.Request.Context(), exec)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExtensionNotFound
		}

		return
	}

	saveExtensionToContext(c, extension)

	return
}

// findERDForExtensionResource is a function that retrieves the extension and
// extension resource definition (ERD) for a given extension slug, ERD slug plural,
// and ERD version.
// It takes a gin.Context object, a boil.ContextExecutor object, and the extensionSlug,
// erdSlugPlural, and erdVersion as parameters.
// The function returns the extension and ERD if found, along with any error
// that occurred during the retrieval process.
// If the extension or ERD is not found, specific error types are returned.
func findERDForExtensionResource(
	c *gin.Context, exec boil.ContextExecutor,
	extensionSlug, erdSlugPlural, erdVersion string,
) (extension *models.Extension, erd *models.ExtensionResourceDefinition, err error) {
	// fetch extension
	if extension == nil {
		extensionQM := qm.Where("slug = ?", extensionSlug)

		// fetch ERD
		queryMods := []qm.QueryMod{
			qm.Where("slug_plural = ?", erdSlugPlural),
			qm.Where("version = ?", erdVersion),
		}

		extension, err = fetchExtension(c, exec, extensionQM,
			qm.Load(
				models.ExtensionRels.ExtensionResourceDefinitions,
				queryMods...,
			),
		)
		if err != nil {
			return
		}
	}

	if len(extension.R.ExtensionResourceDefinitions) < 1 {
		return nil, nil, ErrERDNotFound
	}

	erd = extension.R.ExtensionResourceDefinitions[0]

	return
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

	if !ext.Enabled {
		sendError(c, http.StatusBadRequest, "extension is disabled")
		return
	}

	if !erd.Enabled {
		sendError(c, http.StatusBadRequest, "extension resource definition is disabled")
		return
	}
}
