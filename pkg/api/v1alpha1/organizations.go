package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

// Organization is the organization response
type Organization struct {
	*models.Organization
}

// OrganizationReq is a request to create an organization
type OrganizationReq struct {
	Name string `json:"name"`
}

// listOrganizations lists the organizations as JSON
func (r *Router) listOrganizations(c *gin.Context) {
	queryMods := []qm.QueryMod{
		qm.OrderBy("name"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	orgs, err := models.Organizations(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching organizations", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing organizations: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, orgs)
}

// listOrganizationGroups lists all groups associated with the organization
func (r *Router) listOrganizationGroups(c *gin.Context) {
	queryMods := []qm.QueryMod{}

	id := c.Param("id")
	oid := id

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted organization by slug, use the id")
			return
		}

		// find the org by slug
		org, err := models.Organizations(qm.Where("slug = ?", id)).One(c.Request.Context(), r.DB)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				sendError(c, http.StatusNotFound, "organization not found: "+err.Error())
				return
			}

			sendError(c, http.StatusInternalServerError, "error getting organization"+err.Error())

			return
		}

		oid = org.ID
	}

	queryMods = append(queryMods, qm.Where("organization_id=?", oid))

	groupOrgs, err := models.GroupOrganizations(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching organization groups", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing organization groups: "+err.Error())

		return
	}

	gids := make([]interface{}, len(groupOrgs))
	for i, g := range groupOrgs {
		gids[i] = g.GroupID
	}

	groups, err := models.Groups(qm.WhereIn("id IN ?", gids...)).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching organization groups", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing organization groups: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, groups)
}

// getOrganization gets an organization and it's relationships
func (r *Router) getOrganization(c *gin.Context) {
	queryMods := []qm.QueryMod{}

	id := c.Param("id")

	deleted := false
	if _, deleted = c.GetQuery("deleted"); deleted {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		if deleted {
			sendError(c, http.StatusBadRequest, "unable to get deleted organization by slug, use the id")
			return
		}

		q = qm.Where("slug = ?", id)
	}

	queryMods = append(queryMods, q)

	org, err := models.Organizations(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "organization not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting organization"+err.Error())

		return
	}

	c.JSON(http.StatusOK, Organization{
		Organization: org,
	})
}

// createOrganization creates an org in the database
func (r *Router) createOrganization(c *gin.Context) {
	req := OrganizationReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	org := &models.Organization{
		Name: req.Name,
	}

	dbtools.SetOrganizationSlug(org)

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting organization create transaction: "+err.Error())
		return
	}

	if err := org.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error creating organization: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditOrganizationCreated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), org)
	if err != nil {
		msg := "error creating organization (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating organization (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing organization create, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	c.JSON(http.StatusAccepted, org)
}

// deleteOrganization marks an organization deleted
func (r *Router) deleteOrganization(c *gin.Context) {
	id := c.Param("id")

	q := qm.Where("id = ?", id)

	if _, err := uuid.Parse(id); err != nil {
		q = qm.Where("slug = ?", id)
	}

	org, err := models.Organizations(
		q,
		qm.Load("GroupOrganizations"),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "organization not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting organization"+err.Error())

		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	r.Logger.Debug("transaction started")

	// delete all org links
	if _, err := org.R.GroupOrganizations.DeleteAll(c.Request.Context(), tx); err != nil {
		msg := "error deleting group org link, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	r.Logger.Debug("deleted org links started")

	if _, err := org.Delete(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting organization, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	r.Logger.Debug("deleted org")

	event, err := dbtools.AuditOrganizationDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), org)
	if err != nil {
		msg := "error deleting organization (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting organization (audit): " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	r.Logger.Debug("committing started")

	if err := tx.Commit(); err != nil {
		msg := "error committing organization delete, rolling back: " + err.Error()
		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	c.JSON(http.StatusAccepted, org)
}
