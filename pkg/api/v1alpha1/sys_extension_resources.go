package v1alpha1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/metal-toolbox/governor-api/pkg/jsonschema"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// SystemExtensionResource is the system extension resource response
type SystemExtensionResource struct {
	*models.SystemExtensionResource
	ERD     string `json:"extension_resource_definition"`
	Version string `json:"version"`
}

func findERDForExtensionResource(
	ctx context.Context, exec boil.ContextExecutor,
	extensionSlug, erdSlugPlural, erdVersion string,
) (extension *models.Extension, erd *models.ExtensionResourceDefinition, err error) {
	// fetch extension
	extensionQM := qm.Where("slug = ?", extensionSlug)

	// fetch ERD
	queryMods := []qm.QueryMod{
		qm.Where("slug_plural = ?", erdSlugPlural),
		qm.Where("version = ?", erdVersion),
	}

	extension, err = models.Extensions(
		extensionQM,
		qm.Load(
			models.ExtensionRels.ExtensionResourceDefinitions,
			queryMods...,
		),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrExtensionNotFound
		}

		return
	}

	if len(extension.R.ExtensionResourceDefinitions) < 1 {
		return nil, nil, ErrERDNotFound
	}

	erd = extension.R.ExtensionResourceDefinitions[0]

	return
}

// createSystemExtensionResource creates a system extension resource
func (r *Router) createSystemExtensionResource(c *gin.Context) {
	defer c.Request.Body.Close()

	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

	requestBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		sendError(c, http.StatusBadRequest, err.Error())
		return
	}

	// find ERD
	extension, erd, err := findERDForExtensionResource(
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

	if erd.Scope != ExtensionResourceDefinitionScopeSys.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot create system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	// schema validator
	compiler := jsonschema.NewCompiler(
		extension.Slug, erd.SlugPlural, erd.Version,
		jsonschema.WithUniqueConstraint(c.Request.Context(), erd, nil, r.DB),
	)

	schema, err := compiler.Compile(erd.Schema.String())
	if err != nil {
		sendError(c, http.StatusBadRequest, "ERD schema is not valid: "+err.Error())
		return
	}

	// validate payload
	var v interface{}
	if err := json.Unmarshal(requestBody, &v); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if err := schema.Validate(v); err != nil {
		sendError(c, http.StatusBadRequest, err.Error())
		return
	}

	// insert
	er := &models.SystemExtensionResource{Resource: requestBody}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting extension resource create transaction: "+err.Error())
		return
	}

	if err := erd.AddSystemExtensionResources(c.Request.Context(), tx, true, er); err != nil {
		msg := fmt.Sprintf("error creating %s: %s", erd.Name, err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditSystemExtensionResourceCreated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		er,
	)
	if err != nil {
		msg := fmt.Sprintf("error creating extension resource (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating extension resource: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource create: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		erd.SlugPlural,
		&events.Event{
			Version:                       erd.Version,
			Action:                        events.GovernorEventCreate,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   extension.ID,
			ExtensionResourceID:           er.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension resource create event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	resp := &SystemExtensionResource{
		SystemExtensionResource: er,
		ERD:                     erd.SlugSingular,
		Version:                 erd.Version,
	}

	c.JSON(http.StatusCreated, resp)
}

// listSystemExtensionResource lists system extension resources for an ERD
func (r *Router) listSystemExtensionResources(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

	// find ERD
	_, erd, err := findERDForExtensionResource(
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

	if erd.Scope != ExtensionResourceDefinitionScopeSys.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot list system resources for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	uriQueries := map[string]string{}
	if err := c.BindQuery(&uriQueries); err != nil {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf("error binding uri queries: %s", err.Error()),
		)

		return
	}

	qms := make([]qm.QueryMod, 0, len(uriQueries))

	for k, v := range uriQueries {
		if k == "deleted" {
			qms = append(qms, qm.WithDeleted())
			continue
		}

		qms = append(qms, qm.Where("resource->>? = ?", k, v))
	}

	ers, err := erd.SystemExtensionResources(qms...).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return
	}

	c.JSON(http.StatusOK, ers)
}

// getSystemExtensionResource fetches a system extension resources
func (r *Router) getSystemExtensionResource(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")
	resourceID := c.Param("resource-id")
	_, deleted := c.GetQuery("deleted")

	// find ERD
	_, erd, err := findERDForExtensionResource(
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

	if erd.Scope != ExtensionResourceDefinitionScopeSys.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot get system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	qms := []qm.QueryMod{
		qm.Where("id = ?", resourceID),
	}

	if deleted {
		qms = append(qms, qm.WithDeleted())
	}

	er, err := erd.SystemExtensionResources(qms...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "resource not found: "+err.Error())
			return
		}

		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return
	}

	c.JSON(http.StatusOK, er)
}

// updateSystemExtensionResource updates a system extension resources
func (r *Router) updateSystemExtensionResource(c *gin.Context) {
	defer c.Request.Body.Close()

	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")
	resourceID := c.Param("resource-id")

	requestBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		sendError(c, http.StatusBadRequest, err.Error())
		return
	}

	// find ERD
	extension, erd, err := findERDForExtensionResource(
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

	if erd.Scope != ExtensionResourceDefinitionScopeSys.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot update system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	qms := []qm.QueryMod{
		qm.Where("id = ?", resourceID),
	}

	// fetch resource
	er, err := erd.SystemExtensionResources(qms...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "resource not found: "+err.Error())
			return
		}

		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return
	}

	// schema validator
	compiler := jsonschema.NewCompiler(
		extension.Slug, erd.SlugPlural, erd.Version,
		jsonschema.WithUniqueConstraint(c.Request.Context(), erd, &er.ID, r.DB),
	)

	schema, err := compiler.Compile(erd.Schema.String())
	if err != nil {
		sendError(c, http.StatusBadRequest, "ERD schema is not valid: "+err.Error())
		return
	}

	// validate payload
	var v interface{}
	if err := json.Unmarshal(requestBody, &v); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if err := schema.Validate(v); err != nil {
		sendError(c, http.StatusBadRequest, err.Error())
		return
	}

	// update
	original := *er
	er.Resource = requestBody

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting extension resource update transaction: "+err.Error())
		return
	}

	if _, err := er.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error updating %s: %s", erd.Name, err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditSystemExtensionResourceUpdated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		&original,
		er,
	)
	if err != nil {
		msg := fmt.Sprintf("error updating extension resource (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating extension resource: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource update: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		erd.SlugPlural,
		&events.Event{
			Version:                       erd.Version,
			Action:                        events.GovernorEventUpdate,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   extension.ID,
			ExtensionResourceID:           er.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension resource update event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	resp := &SystemExtensionResource{
		SystemExtensionResource: er,
		ERD:                     erd.SlugSingular,
		Version:                 erd.Version,
	}

	c.JSON(http.StatusAccepted, resp)
}

// deleteSystemExtensionResource deletes a system extension resources
func (r *Router) deleteSystemExtensionResource(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")
	resourceID := c.Param("resource-id")

	// find ERD
	extension, erd, err := findERDForExtensionResource(
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

	if erd.Scope != ExtensionResourceDefinitionScopeSys.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot delete system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	qms := []qm.QueryMod{
		qm.Where("id = ?", resourceID),
	}

	// fetch resource
	er, err := erd.SystemExtensionResources(qms...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "resource not found: "+err.Error())
			return
		}

		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return
	}

	// delete
	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting extension resource delete transaction: "+err.Error())
		return
	}

	if _, err := er.Delete(c.Request.Context(), tx, false); err != nil {
		msg := fmt.Sprintf("error deleting %s: %s", erd.Name, err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditSystemExtensionResourceDeleted(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		er,
	)
	if err != nil {
		msg := fmt.Sprintf("error deleting extension resource (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error deleting extension resource: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource delete: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		erd.SlugPlural,
		&events.Event{
			Version:                       erd.Version,
			Action:                        events.GovernorEventDelete,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   extension.ID,
			ExtensionResourceID:           er.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension resource delete event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	resp := &SystemExtensionResource{
		SystemExtensionResource: er,
		ERD:                     erd.SlugSingular,
		Version:                 erd.Version,
	}

	c.JSON(http.StatusAccepted, resp)
}
