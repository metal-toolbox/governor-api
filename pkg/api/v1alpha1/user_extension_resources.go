package v1alpha1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
	"github.com/metal-toolbox/governor-api/pkg/jsonschema"
)

// UserExtensionResource is the user extension resource response
type UserExtensionResource struct {
	*models.UserExtensionResource
	ERD     string `json:"extension_resource_definition"`
	Version string `json:"version"`
}

// fetchUserAndERD is a helper function to fetch a user and ERD from the database
// simultaneously since they are not dependent on each other, and they are both
// required for most of the user extension resource endpoints.
func fetchUserAndERD(c *gin.Context, db boil.ContextExecutor) (
	user *models.User,
	extension *models.Extension, erd *models.ExtensionResourceDefinition,
	findUserErr, findERDErr error,
) {
	ctxUser := getCtxUser(c)
	userID := c.Param("id")
	extensionSlug := c.Param("ex-slug")
	erdSlugPlural := c.Param("erd-slug-plural")
	erdVersion := c.Param("erd-version")

	if userID == "" && ctxUser == nil {
		findUserErr = ErrNoUserProvided
		return
	}

	// fetch stuff
	fetchWg := sync.WaitGroup{}

	// fetch user
	if userID != "" {
		fetchWg.Add(1)

		fetchUser := func(ctx context.Context, exec boil.ContextExecutor, id string) {
			defer fetchWg.Done()

			user, findUserErr = models.FindUser(ctx, exec, id)
		}
		go fetchUser(c.Request.Context(), db, userID)
	} else {
		user = ctxUser
	}

	// find ERD
	fetchWg.Add(1)

	fetchERD := func(_ context.Context, exec boil.ContextExecutor, exSlug, erdSlug, erdVersion string) {
		defer fetchWg.Done()

		extension, erd, findERDErr = findERDForExtensionResource(c, exec, exSlug, erdSlug, erdVersion)
	}
	go fetchERD(c.Request.Context(), db, extensionSlug, erdSlugPlural, erdVersion)

	fetchWg.Wait()

	return
}

// createUserExtensionResource creates a user extension resource for a given user
func (r *Router) createUserExtensionResource(c *gin.Context) {
	defer c.Request.Body.Close()

	requestBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		sendError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, extension, erd, findUserErr, findERDErr := fetchUserAndERD(c, r.DB)
	if findUserErr != nil {
		if errors.Is(findUserErr, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, ErrUserNotFound.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user: "+findUserErr.Error())

		return
	}

	if findERDErr != nil {
		if errors.Is(findERDErr, ErrExtensionNotFound) || errors.Is(findERDErr, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, findERDErr.Error())
			return
		}

		sendError(c, http.StatusBadRequest, findERDErr.Error())

		return
	}

	if erd.Scope != ExtensionResourceDefinitionScopeUser.String() {
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
	er := &models.UserExtensionResource{Resource: requestBody, UserID: user.ID}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting extension resource create transaction: "+err.Error())
		return
	}

	if err := erd.AddUserExtensionResources(c.Request.Context(), tx, true, er); err != nil {
		msg := fmt.Sprintf("error creating %s: %s", erd.Name, err.Error())
		if err := tx.Rollback(); err != nil {
			msg += fmt.Sprintf("error rolling back transaction: %s", err.Error())
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditUserExtensionResourceCreated(
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
			UserID:                        user.ID,
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

	resp := &UserExtensionResource{
		UserExtensionResource: er,
		ERD:                   erd.SlugSingular,
		Version:               erd.Version,
	}

	c.JSON(http.StatusCreated, resp)
}

// listUserExtensionResources lists user extension resources from a given user
func (r *Router) listUserExtensionResources(c *gin.Context) {
	user, _, erd, findUserErr, findERDErr := fetchUserAndERD(c, r.DB)
	if findUserErr != nil {
		if errors.Is(findUserErr, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, ErrUserNotFound.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user: "+findUserErr.Error())

		return
	}

	if findERDErr != nil {
		if errors.Is(findERDErr, ErrExtensionNotFound) || errors.Is(findERDErr, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, findERDErr.Error())
			return
		}

		sendError(c, http.StatusBadRequest, findERDErr.Error())

		return
	}

	if erd.Scope != ExtensionResourceDefinitionScopeUser.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot list system resource for %s scoped %s/%s",
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

	extraCapacityForDeletedAndUserID := 2
	qms := make([]qm.QueryMod, 0, len(uriQueries)+extraCapacityForDeletedAndUserID)

	for k, v := range uriQueries {
		if k == "deleted" {
			qms = append(qms, qm.WithDeleted())
			continue
		}

		qms = append(qms, qm.Where("resource->>? = ?", k, v))
	}

	qms = append(qms, qm.Where("user_id = ?", user.ID))

	ers, err := erd.UserExtensionResources(qms...).All(c.Request.Context(), r.DB)
	if err != nil {
		sendError(
			c, http.StatusBadRequest,
			"error finding extension resources: "+err.Error(),
		)

		return
	}

	resp := make([]*UserExtensionResource, len(ers))
	for i, er := range ers {
		resp[i] = &UserExtensionResource{
			UserExtensionResource: er,
			ERD:                   erd.SlugSingular,
			Version:               erd.Version,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// getUserExtensionResource fetches a user extension resources from a given user
func (r *Router) getUserExtensionResource(c *gin.Context) {
	user, _, erd, findUserErr, findERDErr := fetchUserAndERD(c, r.DB)
	if findUserErr != nil {
		if errors.Is(findUserErr, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, ErrUserNotFound.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user: "+findUserErr.Error())

		return
	}

	if findERDErr != nil {
		if errors.Is(findERDErr, ErrExtensionNotFound) || errors.Is(findERDErr, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, findERDErr.Error())
			return
		}

		sendError(c, http.StatusBadRequest, findERDErr.Error())

		return
	}

	if erd.Scope != ExtensionResourceDefinitionScopeUser.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot fetch system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	resourceID := c.Param("resource-id")
	_, deleted := c.GetQuery("deleted")
	qms := []qm.QueryMod{
		qm.Where("user_id = ?", user.ID),
		qm.Where("id = ?", resourceID),
	}

	if deleted {
		qms = append(qms, qm.WithDeleted())
	}

	er, err := erd.UserExtensionResources(qms...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(
				c, http.StatusNotFound,
				fmt.Sprintf("%s: %s", ErrExtensionResourceNotFound.Error(), err.Error()),
			)
		} else {
			sendError(
				c, http.StatusBadRequest,
				"error finding extension resources: "+err.Error(),
			)
		}

		return
	}

	resp := &UserExtensionResource{
		UserExtensionResource: er,
		ERD:                   erd.SlugSingular,
		Version:               erd.Version,
	}

	c.JSON(http.StatusOK, resp)
}

// updateUserExtensionResource updates a user extension resources from a given user
func (r *Router) updateUserExtensionResource(c *gin.Context) {
	defer c.Request.Body.Close()

	requestBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		sendError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, extension, erd, findUserErr, findERDErr := fetchUserAndERD(c, r.DB)
	if findUserErr != nil {
		if errors.Is(findUserErr, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, ErrUserNotFound.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user: "+findUserErr.Error())

		return
	}

	if findERDErr != nil {
		if errors.Is(findERDErr, ErrExtensionNotFound) || errors.Is(findERDErr, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, findERDErr.Error())
			return
		}

		sendError(c, http.StatusBadRequest, findERDErr.Error())

		return
	}

	if erd.Scope != ExtensionResourceDefinitionScopeUser.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot update system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	resourceID := c.Param("resource-id")
	qms := []qm.QueryMod{
		qm.Where("user_id = ?", user.ID),
		qm.Where("id = ?", resourceID),
	}

	er, err := erd.UserExtensionResources(qms...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(
				c, http.StatusNotFound,
				fmt.Sprintf("%s: %s", ErrExtensionResourceNotFound.Error(), err.Error()),
			)
		} else {
			sendError(
				c, http.StatusBadRequest,
				"error finding extension resources: "+err.Error(),
			)
		}

		return
	}

	// schema validator
	compiler := jsonschema.NewCompiler(
		extension.Slug, erd.SlugPlural, erd.Version,
		jsonschema.WithUniqueConstraint(c.Request.Context(), erd, &resourceID, r.DB),
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

	event, err := dbtools.AuditUserExtensionResourceUpdated(
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
			UserID:                        user.ID,
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

	resp := &UserExtensionResource{
		UserExtensionResource: er,
		ERD:                   erd.SlugSingular,
		Version:               erd.Version,
	}

	c.JSON(http.StatusAccepted, resp)
}

// deleteUserExtensionResource fetches a user extension resources from a given user
func (r *Router) deleteUserExtensionResource(c *gin.Context) {
	user, extension, erd, findUserErr, findERDErr := fetchUserAndERD(c, r.DB)
	if findUserErr != nil {
		if errors.Is(findUserErr, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, ErrUserNotFound.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user: "+findUserErr.Error())

		return
	}

	if findERDErr != nil {
		if errors.Is(findERDErr, ErrExtensionNotFound) || errors.Is(findERDErr, ErrERDNotFound) {
			sendError(c, http.StatusNotFound, findERDErr.Error())
			return
		}

		sendError(c, http.StatusBadRequest, findERDErr.Error())

		return
	}

	if erd.Scope != ExtensionResourceDefinitionScopeUser.String() {
		sendError(
			c, http.StatusBadRequest,
			fmt.Sprintf(
				"cannot delete system resource for %s scoped %s/%s",
				erd.Scope, erd.SlugSingular, erd.Version,
			),
		)

		return
	}

	resourceID := c.Param("resource-id")
	qms := []qm.QueryMod{
		qm.Where("user_id = ?", user.ID),
		qm.Where("id = ?", resourceID),
	}

	er, err := erd.UserExtensionResources(qms...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(
				c, http.StatusNotFound,
				fmt.Sprintf("%s: %s", ErrExtensionResourceNotFound.Error(), err.Error()),
			)
		} else {
			sendError(
				c, http.StatusBadRequest,
				"error finding extension resources: "+err.Error(),
			)
		}

		return
	}

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

	event, err := dbtools.AuditUserExtensionResourceDeleted(
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
			UserID:                        user.ID,
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

	resp := &UserExtensionResource{
		UserExtensionResource: er,
		ERD:                   erd.SlugSingular,
		Version:               erd.Version,
	}

	c.JSON(http.StatusOK, resp)
}
