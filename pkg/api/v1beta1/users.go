package v1beta1

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/models"
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
	permittedListUsersParams = []string{"external_id", "email", "deleted", "limit", "sort_by", "sort_order", "search", "status[]", "next_cursor", "prev_cursor", "last"}
	validStatuses            = []string{UserStatusActive, UserStatusPending, UserStatusSuspended}
	allowedSortCols          = []string{"name", "email", "id"}
)

// User is a user response
type User struct {
	*models.User
	Memberships        []string `json:"memberships,omitempty"`
	MembershipRequests []string `json:"membership_requests,omitempty"`
}

// listUsers responds with the list of all users
func (r *Router) listUsers(c *gin.Context) {
	ctx := c.Request.Context()
	queryMods := []qm.QueryMod{}

	for k, val := range c.Request.URL.Query() {
		r.Logger.Debug("checking query", zap.String("url.query.key", k), zap.Strings("url.query.value", val))

		// check for allowed parameters
		if !contains(permittedListUsersParams, k) {
			r.Logger.Warn("found illegal parameter in request", zap.String("parameter", k))
			sendError(c, http.StatusBadRequest, "illegal parameter: "+k)

			return
		}

		convertedVals := make([]interface{}, len(val))
		for i, v := range val {
			convertedVals[i] = v
		}

		switch k {
		case "email":
			// append email queries as case insensitive using LOWER()
			// Note: This does a table scan by default, but we should be able to add a functional
			// index if performance is an issue: CREATE INDEX ON users (LOWER(email));
			// alternatives are to use ILIKE which is postgres specific, and require sanitizing '%' if we want exact matches
			for _, v := range val {
				queryMods = append(queryMods, qm.Or("LOWER(email) = LOWER(?)", v))
			}
		case "external_id":
			queryMods = append(queryMods, qm.Or2(qm.WhereIn(k+" IN ?", convertedVals...)))
		}
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

	c.JSON(http.StatusOK, &PaginationResponse[*models.User]{
		TotalRecordCount: count,
		Records:          users,
		PrevCursor:       prevCursorResp,
		NextCursor:       nextCursorResp,
	})
}
