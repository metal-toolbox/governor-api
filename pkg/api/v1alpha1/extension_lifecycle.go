package v1alpha1

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
		return extension, err
	}

	extension, err = models.Extensions(qms...).One(c.Request.Context(), exec)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExtensionNotFound
		}

		return extension, err
	}

	saveExtensionToContext(c, extension)

	return extension, err
}

type findERDConf struct {
	useERDSlugSingular bool
}

type findERDOption func(*findERDConf)

func findERDUseSlugSingular() findERDOption {
	return func(conf *findERDConf) {
		conf.useERDSlugSingular = true
	}
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
	extensionSlug, erdSlug, erdVersion string,
	opts ...findERDOption,
) (extension *models.Extension, erd *models.ExtensionResourceDefinition, err error) {
	// fetch extension
	extensionQM := qm.Where("slug = ?", extensionSlug)

	conf := findERDConf{}

	for _, opt := range opts {
		opt(&conf)
	}

	// fetch ERD
	queryMods := []qm.QueryMod{
		qm.Where("version = ?", erdVersion),
	}

	if conf.useERDSlugSingular {
		queryMods = append(queryMods, qm.Where("slug_singular = ?", erdSlug))
	} else {
		queryMods = append(queryMods, qm.Where("slug_plural = ?", erdSlug))
	}

	extension, err = fetchExtension(c, exec, extensionQM,
		qm.Load(
			models.ExtensionRels.ExtensionResourceDefinitions,
			queryMods...,
		),
	)
	if err != nil {
		return extension, erd, err
	}

	if len(extension.R.ExtensionResourceDefinitions) < 1 {
		return nil, nil, ErrERDNotFound
	}

	erd = extension.R.ExtensionResourceDefinitions[0]

	return extension, erd, err
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
	ext := getCtxExtension(c)
	erd := getCtxERD(c)

	// only check DB if extension or ERD is not loaded
	if ext == nil || erd == nil {
		var err error

		ext, erd, err = findERDForExtensionResource(
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

func mwFindERDWithURIParams(
	db *sqlx.DB,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracer.Start(c.Request.Context(), "mwFindERDWithURIParams")
		defer span.End()

		extensionSlug := c.Param("ex-slug")
		erdSlugPlural := c.Param("erd-slug-plural")
		erdVersion := c.Param("erd-version")

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

		span.SetAttributes(
			attribute.String("extensionSlug", ext.Slug),
			attribute.String("erdSlugPlural", erd.SlugPlural),
			attribute.String("erdVersion", erd.Version),
		)

		setCtxExtension(c, ext)
		setCtxERD(c, erd)
	}
}

func mwFindERDWithRequestBody(
	db *sqlx.DB,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracer.Start(c.Request.Context(), "mwFindERDWithRequestBody")
		defer span.End()

		requestBody := io.NopCloser(c.Request.Body)
		res := &ExtensionResource{}

		if err := json.NewDecoder(requestBody).Decode(res); err != nil {
			span.SetStatus(codes.Error, err.Error())
			sendError(c, http.StatusBadRequest, err.Error())

			return
		}

		setCtxExtensionResource(c, res)

		ext, erd, err := findERDForExtensionResource(
			c, db,
			res.Extension, res.Kind, res.Version,
			findERDUseSlugSingular(),
		)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())

			if errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrERDNotFound) {
				sendError(c, http.StatusNotFound, err.Error())
				return
			}

			sendError(c, http.StatusBadRequest, err.Error())

			return
		}

		span.SetAttributes(
			attribute.String("extensionSlug", ext.Slug),
			attribute.String("erdSlugPlural", erd.SlugPlural),
			attribute.String("erdVersion", erd.Version),
		)

		setCtxExtension(c, ext)
		setCtxERD(c, erd)
	}
}
