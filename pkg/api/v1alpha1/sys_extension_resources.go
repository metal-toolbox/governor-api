package v1alpha1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/metal-toolbox/governor-api/pkg/jsonschema"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// SystemExtensionResource is the system extension resource response
type SystemExtensionResource struct {
	*models.SystemExtensionResource
	ERD     string `json:"extension_resource_definition"`
	Version string `json:"version"`
}

// createSystemExtensionResource creates a system extension resource
func createSystemExtensionResource(
	c *gin.Context,
	db *sqlx.DB, eb *eventbus.Client,
	ext *models.Extension, erd *models.ExtensionResourceDefinition,
	requestBody []byte, ownerID string,
) *SystemExtensionResource {
	ctx, span := tracer.Start(c.Request.Context(), "createSystemExtensionResource")
	defer span.End()

	// schema validation
	if err := validateSystemExtensionResource(
		ctx, ext.Slug, erd, db, requestBody, nil,
	); err != nil {
		span.SetStatus(codes.Error, "validation error")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, fmt.Sprintf("validation error: %s", err.Error()))

		return nil
	}

	span.SetAttributes(
		attribute.String("extension.slug", ext.Slug),
		attribute.String("erd.slug", erd.SlugSingular),
		attribute.String("erd.version", erd.Version),
	)

	bytes, err := json.Marshal(APIStatusMessage{
		Message:   "Resource created",
		Status:    "true",
		Type:      "Accepted",
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		span.SetStatus(codes.Error, "error marshalling initial status message")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error marshalling initial status message: "+err.Error())

		return nil
	}

	// insert
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		span.SetStatus(codes.Error, "error starting extension resource create transaction")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error starting extension resource create transaction: "+err.Error())

		return nil
	}

	er := &models.SystemExtensionResource{
		Resource:        requestBody,
		ResourceVersion: time.Now().UnixMilli(),
		Messages:        []string{string(bytes)},
	}

	if ownerID != "" {
		er.OwnerID = null.NewString(ownerID, true)
	}

	if err := erd.AddSystemExtensionResources(ctx, tx, true, er); err != nil {
		msg := fmt.Sprintf("error creating %s: %s", erd.Name, err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	event, err := dbtools.AuditSystemExtensionResourceCreated(
		ctx, tx, getCtxAuditID(c), getCtxUser(c), er,
	)
	if err != nil {
		msg := fmt.Sprintf("error creating extension resource (audit): %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating extension resource: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource create: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	err = eb.Publish(
		ctx,
		erd.SlugPlural,
		&events.Event{
			Version:                       erd.Version,
			Action:                        events.GovernorEventCreate,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   ext.ID,
			ExtensionResourceID:           er.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		span.SetStatus(codes.Error, "failed to publish extension resource create event")
		span.RecordError(err)
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension resource create event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return nil
	}

	return &SystemExtensionResource{
		SystemExtensionResource: er,
		ERD:                     erd.SlugSingular,
		Version:                 erd.Version,
	}
}

// listSystemExtensionResource lists system extension resources for an ERD
func (r *Router) listSystemExtensionResources(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

	// find ERD
	_, erd, err := findERDForExtensionResource(
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

// updateSystemExtensionResource updates a system extension resource
func updateSystemExtensionResource(
	c *gin.Context,
	db *sqlx.DB, eb *eventbus.Client,
	ext *models.Extension, erd *models.ExtensionResourceDefinition,
	resourceID string, requestBody []byte, statusMsgs []string, resourceVersion *int64,
) *SystemExtensionResource {
	ctx, span := tracer.Start(c.Request.Context(), "updateSystemExtensionResource")
	defer span.End()

	qms := []qm.QueryMod{
		qm.Where("id = ?", resourceID),
	}

	// fetch resource
	er, err := erd.SystemExtensionResources(qms...).One(ctx, db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Error, "resource not found")
			span.RecordError(err)
			sendError(c, http.StatusNotFound, "resource not found: "+err.Error())

			return nil
		}

		span.SetStatus(codes.Error, "error finding extension resource")
		span.RecordError(err)
		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return nil
	}

	// schema validator
	if err := validateSystemExtensionResource(
		ctx, ext.Slug, erd, db, requestBody, &er.ID,
	); err != nil {
		span.SetStatus(codes.Error, "validation error")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, fmt.Sprintf("validation error: %s", err.Error()))

		return nil
	}

	// optimistic concurrency control with resource versioning
	currentResourceVersion := er.ResourceVersion
	if resourceVersion != nil {
		currentResourceVersion = *resourceVersion
	}

	span.SetAttributes(
		attribute.String("extension.slug", ext.Slug),
		attribute.String("erd.slug", erd.SlugSingular),
		attribute.String("erd.version", erd.Version),
		attribute.String("resource.id", resourceID),
		attribute.Int64("requested-resource-version", currentResourceVersion),
	)

	// update
	original := *er

	er.Resource = requestBody
	er.ResourceVersion = time.Now().UnixMilli()

	if len(statusMsgs) > 0 {
		er.Messages = statusMsgs
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		span.SetStatus(codes.Error, "error starting extension resource update transaction")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error starting extension resource update transaction: "+err.Error())

		return nil
	}

	const update = `
		UPDATE system_extension_resources
			SET resource = $1, resource_version = $2, messages = $3, updated_at = NOW()
		WHERE 
			id = $4 AND resource_version = $5
		RETURNING
			updated_at
		;
	`

	q := queries.Raw(
		update,
		er.Resource, er.ResourceVersion, er.Messages,
		er.ID, currentResourceVersion,
	)

	rows, err := q.QueryContext(ctx, tx)
	if err != nil {
		msg := fmt.Sprintf("error updating %s: %s", erd.Name, err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	if !rows.Next() {
		defer rows.Close()

		msg := fmt.Sprintf("error updating %s: resource version conflict", erd.Name)
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		sendError(c, http.StatusConflict, msg)

		return nil
	}

	var updatedAt time.Time
	if err := rows.Scan(&updatedAt); err != nil {
		defer rows.Close()

		msg := fmt.Sprintf("error scanning updated_at for %s: %s", erd.Name, err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusInternalServerError, msg)

		return nil
	}

	er.UpdatedAt = updatedAt

	rows.Close()

	event, err := dbtools.AuditSystemExtensionResourceUpdated(
		ctx, tx, getCtxAuditID(c), getCtxUser(c), &original, er,
	)
	if err != nil {
		msg := fmt.Sprintf("error updating extension resource (audit): %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating extension resource: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource update: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return nil
	}

	err = eb.Publish(
		ctx,
		erd.SlugPlural,
		&events.Event{
			Version:                       erd.Version,
			Action:                        events.GovernorEventUpdate,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   ext.ID,
			ExtensionResourceID:           er.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		span.SetStatus(codes.Error, "failed to publish extension resource update event")
		span.RecordError(err)
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension resource update event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return nil
	}

	return &SystemExtensionResource{
		SystemExtensionResource: er,
		ERD:                     erd.SlugSingular,
		Version:                 erd.Version,
	}
}

// deleteSystemExtensionResource deletes a system extension resources
func (r *Router) deleteSystemExtensionResource(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")
	resourceID := c.Param("resource-id")

	// find ERD
	extension, erd, err := findERDForExtensionResource(
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

func validateSystemExtensionResource(
	ctx context.Context, extSlug string,
	erd *models.ExtensionResourceDefinition,
	db boil.ContextExecutor, requestBody []byte,
	excludeResourceID *string,
) error {
	// schema validator
	compiler := jsonschema.NewCompiler(
		extSlug, erd.SlugPlural, erd.Version,
		jsonschema.WithUniqueConstraint(ctx, erd, excludeResourceID, db),
	)

	schema, err := compiler.Compile(erd.Schema.String())
	if err != nil {
		return err
	}

	// validate payload
	var v interface{}
	if err := json.Unmarshal(requestBody, &v); err != nil {
		return err
	}

	if err := schema.Validate(v); err != nil {
		return err
	}

	return nil
}
