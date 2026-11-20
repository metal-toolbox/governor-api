package v1alpha1

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

type mwAuthRole int

const (
	// AuthRoleUser indicates an authenticated user with regular user privileges
	AuthRoleUser mwAuthRole = iota
	// AuthRoleAdmin indicates an authenticated user who is a governor admin
	AuthRoleAdmin
	// AuthRoleGroupMember indicates an authenticated user who is a member of the given group
	AuthRoleGroupMember
	// AuthRoleGroupAdmin indicates an authenticated user who is an admin in the given group
	AuthRoleGroupAdmin
	// AuthRoleAdminOrGroupAdmin indicates an authenticated user who is an admin in the given group or governor admin
	AuthRoleAdminOrGroupAdmin
	// AuthRoleAdminOrGroupAdminOrGroupApprover indicates an authenticated user who is an admin in the given group or governor admin or a member of the approver group
	AuthRoleAdminOrGroupAdminOrGroupApprover
)

func (u mwAuthRole) String() string {
	return [...]string{
		"AuthRoleUser",
		"AuthRoleAdmin",
		"AuthRoleGroupMember",
		"AuthRoleGroupAdmin",
		"AuthRoleAdminOrGroupAdmin",
		"AuthRoleAdminOrGroupAdminOrGroupApprover",
	}[u]
}

const (
	contextKeyUser          = "current_user"
	contextKeyAdmin         = "is_admin"
	contextKeyGroupAdmin    = "is_group_admin"
	contextKeyGroupMember   = "is_group_member"
	contextKeyGroupApprover = "is_group_approver"
)

// oidcScope is the scope that is required for the oidcAuthRequired check
const oidcScope = "openid"

// mwUserAuthRequired checks if the current authenticated user has the required auth role and
// adds user information to the gin context. A first sign-in from a provider with identity.enabled
// may register the user (see resolveUser).
func (r *Router) mwUserAuthRequired(authRole mwAuthRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		r.Logger.Debug("mwUserAuthRequired", zap.String("role", authRole.String()))

		r.Logger.Debug("gin context",
			zap.String("subject", c.GetString("jwt.subject")),
			zap.String("user", c.GetString("jwt.user")),
			zap.Any("roles", c.GetStringSlice("jwt.roles")),
		)

		if !contains(c.GetStringSlice("jwt.roles"), oidcScope) {
			r.Logger.Debug("oidc scope not found, skipping user authorization check", zap.String("oidcScope", oidcScope))
			return
		}

		queryMods := []qm.QueryMod{
			qm.Load("GroupMemberships"),
			qm.Load("GroupMembershipRequests"),
			qm.Load("GroupMembershipRequests.User"),
			qm.Load("GroupMembershipRequests.Group"),
			qm.Load("RequesterUserGroupApplicationRequests"),
			qm.Load("RequesterUserGroupApplicationRequests.Application"),
			qm.Load("RequesterUserGroupApplicationRequests.Group"),
			qm.Load("RequesterUserGroupApplicationRequests.ApproverGroup"),
			qm.Load("RequesterUserGroupApplicationRequests.RequesterUser"),
		}

		user, ok := r.resolveUser(c, true, queryMods...)
		if !ok {
			return
		}

		isAdmin := false

		r.Logger.Debug("got authenticated user", zap.Any("user", user))

		enumeratedMemberships, err := dbtools.GetMembershipsForUser(c.Request.Context(), r.DB.DB, user.ID, false)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "error getting enumerated groups: "+err.Error())
			return
		}

		memberships := make([]string, len(enumeratedMemberships))
		for i, m := range enumeratedMemberships {
			memberships[i] = m.GroupID
		}

		ag := make([]interface{}, len(r.AdminGroups))
		for i, a := range r.AdminGroups {
			ag[i] = a
		}

		adminGroups, err := models.Groups(qm.WhereIn("slug IN ?", ag...)).All(c.Request.Context(), r.DB)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "error getting admin groups: "+err.Error())
			return
		}

		for _, g := range adminGroups {
			if contains(memberships, g.ID) {
				isAdmin = true
				break
			}
		}

		// add user to gin context
		setCtxUser(c, user)
		setCtxAdmin(c, &isAdmin)

		if authRole == AuthRoleUser {
			return
		}

		// check if the user is a governor admin
		if authRole == AuthRoleAdmin {
			if !isAdmin {
				r.Logger.Debug("user is not admin")

				sendError(c, http.StatusUnauthorized, "user not admin")

				return
			}

			return
		}

		r.Logger.Debug("unsupported auth role")
		sendError(c, http.StatusUnauthorized, "unsupported auth role")
	}
}

// mwGroupAuthRequired checks if the current authenticated user is a member or admin in the given group
// and adds user information to the gin context. It expects to find the group id (could be also the slug)
// in the id context param.
// nolint:gocyclo
func (r *Router) mwGroupAuthRequired(authRole mwAuthRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		r.Logger.Debug("mwGroupAuthRequired", zap.String("role", authRole.String()))

		if !contains(c.GetStringSlice("jwt.roles"), oidcScope) {
			r.Logger.Debug("oidc scope not found, skipping user authorization check", zap.String("oidcScope", oidcScope))
			return
		}

		if c.GetString("jwt.user") == "" {
			sendError(c, http.StatusUnauthorized, "missing jwt user")
			return
		}

		// check that the group id is passed as a request param called `id`
		id := c.Param("id")
		if id == "" {
			sendError(c, http.StatusUnauthorized, "missing group id in context")
			return
		}

		group, err := models.FindGroup(c.Request.Context(), r.DB, id)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				sendError(c, http.StatusInternalServerError, "error getting group: "+err.Error())
				return
			}

			sendError(c, http.StatusUnauthorized, "group not found")

			return
		}

		user, ok := r.resolveUser(c, false)
		if !ok {
			return
		}

		r.Logger.Debug("got authenticated user", zap.Any("user", user))

		isGroupMember := false
		isGroupAdmin := false
		isGroupApprover := false

		idIsSlug := false
		if _, err := uuid.Parse(id); err != nil {
			// assume the group id is actually a slug
			idIsSlug = true
		}

		enumeratedMemberships, err := dbtools.GetMembershipsForUser(c.Request.Context(), r.DB.DB, user.ID, true)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "error getting enumerated groups: "+err.Error())
			return
		}

		for _, m := range enumeratedMemberships {
			groupID := m.GroupID
			if idIsSlug {
				groupID = m.Group.Slug
			}

			if id == groupID {
				isGroupMember = true

				if m.AdminExpiresAt.Valid {
					if time.Now().Before(m.AdminExpiresAt.Time) {
						isGroupAdmin = m.IsAdmin
					}
				} else {
					isGroupAdmin = m.IsAdmin
				}

				break
			}

			if group.ApproverGroup.Valid && group.ApproverGroup.String == groupID {
				isGroupApprover = true
			}
		}

		// add user to gin context
		setCtxUser(c, user)
		setCtxGroupAdmin(c, &isGroupAdmin)
		setCtxGroupMember(c, &isGroupMember)
		setCtxGroupApprover(c, &isGroupApprover)

		if authRole == AuthRoleGroupMember {
			if !isGroupMember {
				r.Logger.Debug("user is not group member", zap.String("group id", id))

				sendError(c, http.StatusUnauthorized, "user not group member")

				return
			}

			return
		}

		if authRole == AuthRoleGroupAdmin {
			if !isGroupAdmin {
				r.Logger.Debug("user is not group admin", zap.String("group id", id))

				sendError(c, http.StatusUnauthorized, "user not group admin")

				return
			}

			return
		}

		if authRole == AuthRoleAdminOrGroupAdmin {
			isAdmin := false

			memberships := make(map[string]struct{})
			for _, m := range enumeratedMemberships {
				memberships[m.GroupID] = struct{}{}
			}

			ag := make([]interface{}, len(r.AdminGroups))
			for i, a := range r.AdminGroups {
				ag[i] = a
			}

			adminGroups, err := models.Groups(qm.WhereIn("slug IN ?", ag...)).All(c.Request.Context(), r.DB)
			if err != nil {
				sendError(c, http.StatusInternalServerError, "error getting admin groups: "+err.Error())
				return
			}

			for _, g := range adminGroups {
				if _, found := memberships[g.ID]; found {
					isAdmin = true
				}
			}

			if !isGroupAdmin && !isAdmin {
				r.Logger.Debug("user is not admin or group admin", zap.String("group id", id))

				sendError(c, http.StatusUnauthorized, "user not admin or group admin")

				return
			}

			return
		}

		if authRole == AuthRoleAdminOrGroupAdminOrGroupApprover {
			isAdmin := false

			memberships := make(map[string]struct{})
			for _, m := range enumeratedMemberships {
				memberships[m.GroupID] = struct{}{}
			}

			ag := make([]interface{}, len(r.AdminGroups))
			for i, a := range r.AdminGroups {
				ag[i] = a
			}

			adminGroups, err := models.Groups(qm.WhereIn("slug IN ?", ag...)).All(c.Request.Context(), r.DB)
			if err != nil {
				sendError(c, http.StatusInternalServerError, "error getting admin groups: "+err.Error())
				return
			}

			for _, g := range adminGroups {
				if _, found := memberships[g.ID]; found {
					isAdmin = true
				}
			}

			if !isGroupAdmin && !isAdmin && !isGroupApprover {
				r.Logger.Debug("user is not admin or group admin", zap.String("group id", id))

				sendError(c, http.StatusUnauthorized, "user not admin or group admin")

				return
			}

			return
		}

		r.Logger.Debug("unsupported auth role")
		sendError(c, http.StatusUnauthorized, "unsupported auth role")
	}
}

func getCtxUser(c *gin.Context) *models.User {
	cu, exists := c.Get(contextKeyUser)
	if !exists {
		return nil
	}

	user, ok := cu.(*models.User)
	if !ok {
		return nil
	}

	return user
}

func setCtxUser(c *gin.Context, u *models.User) {
	c.Set(contextKeyUser, u)
}

func getCtxAdmin(c *gin.Context) *bool {
	ca, exists := c.Get(contextKeyAdmin)
	if !exists {
		return nil
	}

	admin, ok := ca.(*bool)
	if !ok {
		return nil
	}

	return admin
}

func setCtxAdmin(c *gin.Context, a *bool) {
	c.Set(contextKeyAdmin, a)
}

func setCtxGroupAdmin(c *gin.Context, a *bool) {
	c.Set(contextKeyGroupAdmin, a)
}

func setCtxGroupMember(c *gin.Context, m *bool) {
	c.Set(contextKeyGroupMember, m)
}

func setCtxGroupApprover(c *gin.Context, a *bool) {
	c.Set(contextKeyGroupApprover, a)
}

func getCtxAuditID(c *gin.Context) string {
	ca, exists := c.Get(ginaudit.AuditIDContextKey)
	if !exists {
		return ""
	}

	id, ok := ca.(string)
	if !ok {
		return ""
	}

	return id
}

func getCtxActorID(c *gin.Context) string {
	actorUser := getCtxUser(c)
	if actorUser == nil {
		return ""
	}

	return actorUser.ID
}
