package v1alpha1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/metal-toolbox/governor-api/pkg/jsonschema"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// ExtensionResourceDefinition is the extension resource definition response
type ExtensionResourceDefinition struct {
	*models.ExtensionResourceDefinition
}

// ExtensionResourceDefinitionScope is an enum type for scopes in an ERD
type ExtensionResourceDefinitionScope string

// String converts an ExtensionResourceDefinitionScope to a string
func (scope ExtensionResourceDefinitionScope) String() string {
	return string(scope)
}

const (
	// ExtensionResourceDefinitionScopeUser represents the `user` scope
	ExtensionResourceDefinitionScopeUser ExtensionResourceDefinitionScope = "user"
	// ExtensionResourceDefinitionScopeSys represents the `system` scope
	ExtensionResourceDefinitionScopeSys ExtensionResourceDefinitionScope = "system"
)

// ExtensionResourceDefinitionReq is a request to create an extension resource definition
type ExtensionResourceDefinitionReq struct {
	Name         string                           `json:"name"`
	Description  string                           `json:"description"`
	SlugSingular string                           `json:"slug_singular"`
	SlugPlural   string                           `json:"slug_plural"`
	Version      string                           `json:"version"`
	Scope        ExtensionResourceDefinitionScope `json:"scope"`
	Schema       json.RawMessage                  `json:"schema"`
	Enabled      *bool                            `json:"enabled"`
}

func isValidSlug(s string) bool {
	// This regex ensures the slug is lowercase, uses hyphens to separate words,
	// does not start or end with a hyphen, and contains only alphanumeric characters or hyphens.
	pattern := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	return pattern.MatchString(s)
}

func findERD(
	ctx context.Context, exec boil.ContextExecutor,
	extensionID, erdIDOrSlug, erdVersion string, deleted bool,
) (extension *models.Extension, erd *models.ExtensionResourceDefinition, err error) {
	// fetch extension
	var extensionQM qm.QueryMod
	if _, err := uuid.Parse(extensionID); err != nil {
		extensionQM = qm.Where("slug = ?", extensionID)
	} else {
		extensionQM = qm.Where("id = ?", extensionID)
	}

	// fetch ERD
	queryMods := []qm.QueryMod{}

	// use slug if uuid is invalid
	if _, err = uuid.Parse(erdIDOrSlug); err != nil {
		if deleted {
			return nil, nil, ErrGetDeleteResourcedWithSlug
		}

		if erdVersion == "" {
			return nil, nil, err
		}

		queryMods = append(queryMods, qm.Where("slug_singular = ?", erdIDOrSlug))
		queryMods = append(queryMods, qm.Where("version = ?", erdVersion))
	} else {
		queryMods = append(queryMods, qm.Where("id = ?", erdIDOrSlug))
	}

	if deleted {
		queryMods = append(queryMods, qm.WithDeleted())
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

// listExtensionResourceDefinitions lists extension resource definitions as JSON
func (r *Router) listExtensionResourceDefinitions(c *gin.Context) {
	extensionID := c.Param("eid")
	erdQMs := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		erdQMs = append(erdQMs, qm.WithDeleted())
	}

	var extensionQM qm.QueryMod

	if _, err := uuid.Parse(extensionID); err != nil {
		extensionQM = qm.Where("slug = ?", extensionID)
	} else {
		extensionQM = qm.Where("id = ?", extensionID)
	}

	extension, err := models.Extensions(
		extensionQM, qm.Load(
			models.ExtensionRels.ExtensionResourceDefinitions,
			erdQMs...,
		),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "extension not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting ERD"+err.Error())

		return
	}

	c.JSON(http.StatusOK, extension.R.ExtensionResourceDefinitions)
}

// createExtensionResourceDefinition creates an extension resource definition in DB
func (r *Router) createExtensionResourceDefinition(c *gin.Context) {
	extensionID := c.Param("eid")

	req := &ExtensionResourceDefinitionReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "ERD name is required")
		return
	}

	if req.SlugSingular == "" || req.SlugPlural == "" {
		sendError(c, http.StatusBadRequest, "ERD slugs are required")
		return
	}

	if !isValidSlug(req.SlugSingular) || !isValidSlug(req.SlugPlural) {
		sendError(c, http.StatusBadRequest, "one or both of ERD slugs are invalid")
		return
	}

	if req.Version == "" {
		sendError(c, http.StatusBadRequest, "ERD version is required")
		return
	}

	if req.Enabled == nil {
		sendError(c, http.StatusBadRequest, "ERD enabled is required")
		return
	}

	if req.Scope == "" {
		sendError(c, http.StatusBadRequest, "ERD scope is required")
		return
	}

	if req.Scope != ExtensionResourceDefinitionScopeUser && req.Scope != ExtensionResourceDefinitionScopeSys {
		sendError(c, http.StatusBadRequest, `invalid ERD scope, "system" or "user"`)
		return
	}

	if string(req.Schema) == "" {
		sendError(c, http.StatusBadRequest, "ERD schema is required")
		return
	}

	// user may choose to upload the schema as an escaped JSON string, here uses
	// a string unmarshal to "un-escape" the JSON string.
	var schema string
	if err := json.Unmarshal(req.Schema, &schema); err != nil {
		// if the user upload the schema as an object, simply convert the bytes to
		// string should suffice
		schema = string(req.Schema)
	}

	compiler := jsonschema.NewCompiler(
		extensionID, req.SlugPlural, req.Version,
		jsonschema.WithUniqueConstraint(
			c.Request.Context(),
			&models.ExtensionResourceDefinition{},
			nil,
			nil,
		),
	)

	if _, err := compiler.Compile(schema); err != nil {
		sendError(c, http.StatusBadRequest, "ERD schema is not valid: "+err.Error())
		return
	}

	erd := &models.ExtensionResourceDefinition{
		Name:         req.Name,
		SlugSingular: req.SlugSingular,
		SlugPlural:   req.SlugPlural,
		Version:      req.Version,
		Description:  req.Description,
		Scope:        string(req.Scope),
		Schema:       []byte(schema),
		Enabled:      *req.Enabled,
	}

	var extensionQM qm.QueryMod

	if _, err := uuid.Parse(extensionID); err != nil {
		extensionQM = qm.Where("slug = ?", extensionID)
	} else {
		extensionQM = qm.Where("id = ?", extensionID)
	}

	extension, err := models.Extensions(extensionQM).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "extension not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting ERD"+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting ERD create transaction: "+err.Error())
		return
	}

	if err := extension.AddExtensionResourceDefinitions(c.Request.Context(), tx, true, erd); err != nil {
		msg := fmt.Sprintf("error creating ERD: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditExtensionResourceDefinitionCreated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		erd,
	)
	if err != nil {
		msg := fmt.Sprintf("error creating ERD (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error creating ERD: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing ERD create: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorExtensionResourceDefinitionsEventSubject,
		&events.Event{
			Version:                       events.Version,
			Action:                        events.GovernorEventCreate,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   extension.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension create event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, erd)
}

// getExtensionResourceDefinition fetch a extension from DB with given id
func (r *Router) getExtensionResourceDefinition(c *gin.Context) {
	extensionID := c.Param("eid")
	erdIDOrSlug := c.Param("erd-id-slug")
	erdVersion := c.Param("erd-version")
	_, deleted := c.GetQuery("deleted")

	_, erd, err := findERD(
		c.Request.Context(), r.DB,
		extensionID, erdIDOrSlug, erdVersion, deleted,
	)
	if err != nil {
		if errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, err.Error())
			return
		}

		sendError(c, http.StatusBadRequest, err.Error())

		return
	}

	c.JSON(http.StatusOK, erd)
}

// deleteExtensionResourceDefinition marks a extension deleted
func (r *Router) deleteExtensionResourceDefinition(c *gin.Context) {
	extensionID := c.Param("eid")
	erdIDOrSlug := c.Param("erd-id-slug")
	erdVersion := c.Param("erd-version")
	_, deleted := c.GetQuery("deleted")

	extension, erd, err := findERD(
		c.Request.Context(), r.DB,
		extensionID, erdIDOrSlug, erdVersion, deleted,
	)
	if err != nil {
		if errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, err.Error())
			return
		}

		sendError(c, http.StatusBadRequest, err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	if _, err := erd.Delete(c.Request.Context(), tx, false); err != nil {
		msg := fmt.Sprintf("error deleting ERD: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditExtensionResourceDefinitionDeleted(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		erd,
	)
	if err != nil {
		msg := fmt.Sprintf("error deleting ERD (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error deleting ERD: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing ERD delete: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorExtensionResourceDefinitionsEventSubject,
		&events.Event{
			Version:                       events.Version,
			Action:                        events.GovernorEventDelete,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   extension.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension delete event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, extension)
}

// updateExtensionResourceDefinition updates a extension in DB
func (r *Router) updateExtensionResourceDefinition(c *gin.Context) {
	extensionID := c.Param("eid")
	erdIDOrSlug := c.Param("erd-id-slug")
	erdVersion := c.Param("erd-version")
	_, deleted := c.GetQuery("deleted")

	extension, erd, err := findERD(
		c.Request.Context(), r.DB,
		extensionID, erdIDOrSlug, erdVersion, deleted,
	)
	if err != nil {
		if errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, err.Error())
			return
		}

		sendError(c, http.StatusBadRequest, err.Error())

		return
	}

	req := &ExtensionResourceDefinitionReq{}
	if err := c.BindJSON(req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if (req.SlugPlural != "" && req.SlugPlural != erd.SlugPlural) || (req.SlugSingular != "" && req.SlugSingular != erd.SlugSingular) {
		sendError(c, http.StatusBadRequest, "ERD slugs are immutable")
		return
	}

	if req.Scope != "" && req.Scope != ExtensionResourceDefinitionScope(erd.Scope) {
		sendError(c, http.StatusBadRequest, "ERD scope is immutable")
		return
	}

	if req.Version != "" && req.Version != erd.Version {
		sendError(c, http.StatusBadRequest, "ERD version is immutable")
		return
	}

	if string(req.Schema) != "" {
		sendError(c, http.StatusBadRequest, "ERD schema is immutable")
		return
	}

	original := *erd

	if req.Name != "" {
		erd.Name = req.Name
	}

	if req.Description != "" {
		erd.Description = req.Description
	}

	if req.Enabled != nil {
		erd.Enabled = *req.Enabled
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	if _, err := erd.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := fmt.Sprintf("error updating erd: %s. rolling back\n", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditExtensionResourceDefinitionUpdated(
		c.Request.Context(),
		tx,
		getCtxAuditID(c),
		getCtxUser(c),
		&original,
		erd,
	)
	if err != nil {
		msg := fmt.Sprintf("error updating ERD (audit): %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := fmt.Sprintf("error updating ERD: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := fmt.Sprintf("error committing ERD update: %s", err.Error())

		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	err = r.EventBus.Publish(
		c.Request.Context(),
		events.GovernorExtensionResourceDefinitionsEventSubject,
		&events.Event{
			Version:                       events.Version,
			Action:                        events.GovernorEventUpdate,
			AuditID:                       c.GetString(ginaudit.AuditIDContextKey),
			ActorID:                       getCtxActorID(c),
			ExtensionID:                   extension.ID,
			ExtensionResourceDefinitionID: erd.ID,
		},
	)
	if err != nil {
		sendError(
			c,
			http.StatusBadRequest,
			fmt.Sprintf(
				"failed to publish extension update event: %s\n%s",
				err.Error(),
				"downstream changes may be delayed",
			),
		)

		return
	}

	c.JSON(http.StatusAccepted, erd)
}
