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

// createSystemExtensionResource creates a system extension resource
func createSystemExtensionResource(
	c *gin.Context,
	db *sqlx.DB, eb *eventbus.Client,
	ext *models.Extension, erd *models.ExtensionResourceDefinition,
	requestBody []byte, ownerID string,
) {
	ctx, span := tracer.Start(c.Request.Context(), "createSystemExtensionResource")
	defer span.End()

	// schema validation
	if err := validateSystemExtensionResource(
		ctx, ext.Slug, erd, db, requestBody, nil,
	); err != nil {
		span.SetStatus(codes.Error, "validation error")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, fmt.Sprintf("validation error: %s", err.Error()))

		return
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

		return
	}

	// insert
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		span.SetStatus(codes.Error, "error starting extension resource create transaction")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error starting extension resource create transaction: "+err.Error())

		return
	}

	// annotations
	annotations, err := json.Marshal(map[string]string{AnnotationLastAppliedConfig: string(requestBody)})
	if err != nil {
		span.SetStatus(codes.Error, "error marshalling annotations")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error marshalling annotations: "+err.Error())

		return
	}

	er := &models.SystemExtensionResource{
		Resource:        requestBody,
		ResourceVersion: time.Now().UnixMilli(),
		Messages:        []string{string(bytes)},
		Annotations:     annotations,
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

		return
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

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating extension resource: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource create: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return
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

		return
	}

	c.JSON(http.StatusCreated, mkSystemExtensionResource(ext, erd, er))
}

// listSystemExtensionResource lists system extension resources for an ERD
func (r *Router) listSystemExtensionResources(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

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

	resp := make([]*ExtensionResource, 0, len(ers))
	for _, er := range ers {
		resp = append(resp, mkSystemExtensionResource(ext, erd, er))
	}

	c.JSON(http.StatusOK, resp)
}

// getSystemExtensionResource fetches a system extension resources
func (r *Router) getSystemExtensionResource(c *gin.Context) {
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")
	resourceID := c.Param("resource-id")
	_, deleted := c.GetQuery("deleted")

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

	c.JSON(http.StatusOK, mkSystemExtensionResource(ext, erd, er))
}

// updateSystemExtensionResource updates a system extension resource
func updateSystemExtensionResource(
	c *gin.Context,
	db *sqlx.DB, eb *eventbus.Client,
	ext *models.Extension, erd *models.ExtensionResourceDefinition,
	resourceID string, requestBody []byte, statusMsgs []string, resourceVersion *int64,
	reqAnnotations map[string]string,
) {
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

			return
		}

		span.SetStatus(codes.Error, "error finding extension resource")
		span.RecordError(err)
		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return
	}

	// schema validator
	if err := validateSystemExtensionResource(
		ctx, ext.Slug, erd, db, requestBody, &er.ID,
	); err != nil {
		span.SetStatus(codes.Error, "validation error")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, fmt.Sprintf("validation error: %s", err.Error()))

		return
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

	// merge annotations
	annotations := map[string]string{}

	// start with current
	if err := json.Unmarshal(er.Annotations, &annotations); err != nil {
		span.SetStatus(codes.Error, "error unmarshalling current annotations")
		span.RecordError(err)

		// non-fatal error
	}

	// overwrite with requested
	if annotations != nil {
		for k, v := range reqAnnotations {
			if k == AnnotationLastAppliedConfig {
				continue
			}

			annotations[k] = v
		}
	}

	// overwrite last applied configuration, unless the request asked to keep it unchanged
	if v, ok := reqAnnotations[AnnotationLastAppliedConfig]; !ok || v != "skip" {
		annotations[AnnotationLastAppliedConfig] = string(er.Resource)
	}

	span.SetAttributes(
		attribute.String("requested-last-applied-config", string(reqAnnotations[AnnotationLastAppliedConfig])),
		attribute.String("last-applied-config", string(annotations[AnnotationLastAppliedConfig])),
	)

	annotationsBytes, err := json.Marshal(annotations)
	if err != nil {
		span.SetStatus(codes.Error, "error marshalling annotations")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error marshalling annotations: "+err.Error())

		return
	}

	// update
	original := *er

	er.Resource = requestBody
	er.Annotations = annotationsBytes
	er.ResourceVersion = time.Now().UnixMilli()

	if len(statusMsgs) > 0 {
		er.Messages = statusMsgs
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		span.SetStatus(codes.Error, "error starting extension resource update transaction")
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, "error starting extension resource update transaction: "+err.Error())

		return
	}

	const update = `
		UPDATE system_extension_resources
			SET resource = $1, resource_version = $2, messages = $3, updated_at = NOW(), annotations = $6
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
		er.Annotations,
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

		return
	}

	if !rows.Next() {
		defer rows.Close()

		msg := fmt.Sprintf("error updating %s: resource version conflict", erd.Name)
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		sendError(c, http.StatusConflict, msg)

		return
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

		return
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

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating extension resource: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing extension resource update: %s", err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
		sendError(c, http.StatusBadRequest, msg)

		return
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

		return
	}

	c.JSON(http.StatusAccepted, mkSystemExtensionResource(ext, erd, er))
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

	c.JSON(http.StatusAccepted, nil)
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

func mkSystemExtensionResource(
	ext *models.Extension, erd *models.ExtensionResourceDefinition,
	er *models.SystemExtensionResource,
) *ExtensionResource {
	res := &ExtensionResource{
		Extension: ext.Slug,
		Kind:      erd.SlugSingular,
		Version:   erd.Version,
		Spec:      json.RawMessage(er.Resource),
		Metadata: ExtensionResourceMetadata{
			CreatedAt:       er.CreatedAt.Format(time.RFC3339),
			ID:              er.ID,
			ResourceVersion: er.ResourceVersion,
		},
		Status: ExtensionResourceStatus{
			UpdatedAt: er.UpdatedAt.Format(time.RFC3339),
		},
	}

	annotations := map[string]string{}

	if err := json.Unmarshal(er.Annotations, &annotations); err == nil {
		res.Metadata.Annotations = annotations
	}

	if er.OwnerID.Valid && er.OwnerID.String != "" {
		res.Metadata.OwnerRef = ExtensionResourceMetadataOwnerRef{
			Kind: ExtensionResourceOwnerKindGroup,
			ID:   er.OwnerID.String,
		}
	}

	if len(er.Messages) > 0 {
		res.Status.Messages = make([]json.RawMessage, len(er.Messages))
		for i, msg := range er.Messages {
			res.Status.Messages[i] = json.RawMessage(msg)
		}
	}

	return res
}
