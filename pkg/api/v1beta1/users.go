package v1beta1

import (
	"net/http"
	"slices"
	"strings"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	apitypes "github.com/metal-toolbox/governor-api/pkg/api/types"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

const (
	// UserStatusActive is the status for an active user
	UserStatusActive = "active"

	// UserStatusPending is the status for a pending user
	UserStatusPending = "pending"

	// UserStatusSuspended is the status for a suspended user
	UserStatusSuspended = "suspended"
)

var (
	permittedListUsersParams = []string{"external_id", "iss", "email", "deleted", "limit", "sort_by", "sort_order", "search", "status[]", "next_cursor", "prev_cursor", "last", "offset"}
	validStatuses            = []string{UserStatusActive, UserStatusPending, UserStatusSuspended}
	allowedSortCols          = []string{"name", "email", "id"}
)

// User is a user response
type User struct {
	*models.User
	// ExternalIDs are the user's identities at the identity providers. It is
	// an empty array when the user has none.
	ExternalIDs []apitypes.ExternalID `json:"external_ids"`
	// ExternalID is the user's identity at the primary identity provider, the
	// first oidc provider with identity.enabled, or null.
	//
	// Deprecated: use ExternalIDs.
	ExternalID         null.String `json:"external_id"`
	Memberships        []string    `json:"memberships,omitempty"`
	MembershipsDirect  []string    `json:"memberships_direct,omitempty"`
	MembershipRequests []string    `json:"membership_requests,omitempty"`
}

// allConfiguredIssuers reports whether every issuer in issuers is the issuer
// of a configured oidc provider.
func (r *Router) allConfiguredIssuers(issuers []string) bool {
	for _, iss := range issuers {
		if !slices.ContainsFunc(r.AuthConf, func(ac configs.Auth) bool { return ac.Issuer == iss }) {
			return false
		}
	}

	return true
}

// listUsers responds with the list of all users
func (r *Router) listUsers(c *gin.Context) {
	ctx := c.Request.Context()
	queryMods := []qm.QueryMod{}

	filterIssuers, filterIDs := c.QueryArray("iss"), c.QueryArray("external_id")
	if !r.allConfiguredIssuers(filterIssuers) {
		sendError(c, http.StatusUnprocessableEntity, "invalid iss: not a configured issuer")
		return
	}

	for k, val := range c.Request.URL.Query() {
		r.Logger.Debug("checking query", zap.String("url.query.key", k), zap.Strings("url.query.value", val))

		// check for allowed parameters
		if !contains(permittedListUsersParams, k) {
			r.Logger.Warn("found illegal parameter in request", zap.String("parameter", k))
			sendError(c, http.StatusBadRequest, "illegal parameter: "+k)

			return
		}

		if k == "email" {
			// append email queries as case insensitive using LOWER()
			// Note: This does a table scan by default, but we should be able to add a functional
			// index if performance is an issue: CREATE INDEX ON users (LOWER(email));
			// alternatives are to use ILIKE which is postgres specific, and require sanitizing '%' if we want exact matches
			for _, v := range val {
				queryMods = append(queryMods, qm.Or("LOWER(email) = LOWER(?)", v))
			}
		}
	}

	if len(filterIssuers) > 0 || len(filterIDs) > 0 {
		queryMods = append(queryMods, qm.Or2(dbtools.WhereUserHasExternalID(filterIssuers, filterIDs)))
	}

	p, err := parsePagination(c)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error parsing query parameters")
		return
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	if search, ok := c.GetQuery("search"); ok {
		search = "%" + search + "%"
		qmLikeName := qm.Where("LOWER(name) like ?", strings.ToLower(search))
		qmLikeEmail := qm.Or("LOWER(email) like ?", strings.ToLower(search))

		queryMods = append(queryMods, qm.Expr(qmLikeName, qmLikeEmail))
	}

	if statuses, ok := c.GetQueryArray("status[]"); ok {
		qmStatuses := make([]interface{}, len(statuses))

		for i, v := range statuses {
			if !contains(validStatuses, v) {
				sendError(c, http.StatusBadRequest, invalidQueryParameterValue("status, "+v).Error())
				return
			}

			qmStatuses[i] = v
		}

		queryMods = append(queryMods, qm.AndIn("status in ?", qmStatuses...))
	}

	// get count before orderby, limit and offset are set
	count, err := models.Users(queryMods...).Count(ctx, r.DB)
	if err != nil {
		r.Logger.Error("error fetching user count", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error fetching user count: "+err.Error())

		return
	}

	// retrieve limit used + 1 so we can check if there are more records
	queryMods = append(queryMods, qm.Limit(p.Limit+1))

	// allow for pagination by offset, useful for random page access
	if p.Offset > 0 {
		queryMods = append(queryMods, qm.Offset(p.Offset))
	}

	if contains(allowedSortCols, strings.ToLower(p.SortBy)) {
		queryMods = append(queryMods, qm.OrderBy(p.SortBy+" "+p.SortOrder))
	} else {
		sendError(c, http.StatusBadRequest, invalidQueryParameterValue("sortBy, "+p.SortBy).Error())
		return
	}

	var format string
	if strings.EqualFold(p.SortBy, "email") {
		format = "LOWER(%s)"
	}

	if query, param, ok := p.getCursorClause(format); ok {
		queryMods = append(queryMods, qm.Where(query, param))
	}

	users, err := models.Users(queryMods...).All(ctx, r.DB)
	if err != nil {
		r.Logger.Error("error fetching users", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error fetching users: "+err.Error())

		return
	}

	hasMoreRecords := len(users) == p.Limit+1
	if hasMoreRecords {
		users = users[:len(users)-1]
	}

	// reverse slice because we reversed ordering to get the last N records
	if p.Last || p.PrevCursor != "" {
		for i, j := 0, len(users)-1; i < j; i, j = i+1, j-1 {
			users[i], users[j] = users[j], users[i]
		}
	}

	prevCursorResp, err := getPrevCursor(users, &p, hasMoreRecords)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error getting prevCursor: "+err.Error())

		return
	}

	nextCursorResp, err := getNextCursor(users, &p, hasMoreRecords)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error getting nextCursor: "+err.Error())

		return
	}

	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}

	externalIDs, err := dbtools.ListUserExternalIDs(ctx, r.DB, ids)
	if err != nil {
		r.Logger.Error("error fetching user external ids", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error fetching users: "+err.Error())

		return
	}

	records := make([]*User, len(users))
	for i, u := range users {
		records[i] = &User{
			User:        u,
			ExternalIDs: externalIDs[u.ID],
			ExternalID:  dbtools.PrimaryExternalID(externalIDs[u.ID], r.PrimaryIdentityIssuer),
		}
	}

	c.Header("Deprecation", "true")
	c.Header("Link", apitypes.ExternalIDDeprecationLink)

	c.JSON(http.StatusOK, &PaginationResponse[*User]{
		TotalRecordCount: count,
		Records:          records,
		PrevCursor:       prevCursorResp,
		NextCursor:       nextCursorResp,
	})
}
