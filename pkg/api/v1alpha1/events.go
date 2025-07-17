package v1alpha1

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	models "github.com/metal-toolbox/governor-api/internal/models/psql"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"go.uber.org/zap"
)

var (
	// maxPaginationSize represents the maximum number of records that can be returned per page
	maxPaginationSize = 1000
	// defaultPaginationSize represents the default number of records that are returned per page
	defaultPaginationSize = 100
)

// SerializableEvents are audit events that can be serialized into a json.RawMessage
type SerializableEvents interface {
	*models.AuditEvent | []*models.AuditEvent
}

// PaginationParams allow you to paginate the results
type PaginationParams struct {
	Limit   int    `json:"limit,omitempty"`
	Page    int    `json:"page,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Preload bool   `json:"preload,omitempty"`
	OrderBy string `json:"orderby,omitempty"`
}

func parsePagination(c *gin.Context) PaginationParams {
	limit := defaultPaginationSize
	page := 1
	query := c.Request.URL.Query()

	for key, value := range query {
		queryValue := value[len(value)-1]

		switch key {
		case "limit":
			limit, _ = strconv.Atoi(queryValue)
		case "page":
			page, _ = strconv.Atoi(queryValue)
		}
	}

	return PaginationParams{
		Limit: limit,
		Page:  page,
	}
}

func (p *PaginationParams) limitUsed() int {
	limit := p.Limit

	switch {
	case limit > maxPaginationSize:
		limit = maxPaginationSize
	case limit <= 0:
		limit = defaultPaginationSize
	}

	return limit
}

func (p *PaginationParams) offset() int {
	page := p.Page
	if page == 0 {
		page = 1
	}

	return (page - 1) * p.limitUsed()
}

// updateContextWithAuditEventData updates the context with an audit event data
func updateContextWithAuditEventData[V SerializableEvents](c *gin.Context, event V) error {
	ev, err := json.Marshal(event)
	if err != nil {
		return err
	}

	j := json.RawMessage(ev)

	c.Set(ginaudit.AuditDataContextKey, &j)

	return nil
}

// EventsResponse is the response returned from a request for audit events
type EventsResponse struct {
	PageSize         int                    `json:"page_size,omitempty"`
	Page             int                    `json:"page,omitempty"`
	PageCount        int                    `json:"page_count,omitempty"`
	TotalPages       int                    `json:"total_pages,omitempty"`
	TotalRecordCount int64                  `json:"total_record_count,omitempty"`
	Records          models.AuditEventSlice `json:"records,omitempty"`
}

// listEvents returns the audit events from the database as JSON
func (r *Router) listEvents(c *gin.Context) {
	p := parsePagination(c)

	// TODO filtering
	mods := []qm.QueryMod{}

	count, err := models.AuditEvents(mods...).Count(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching audit events", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing audit events: "+err.Error())

		return
	}

	mods = append(mods, qm.Limit(p.limitUsed()))

	if p.Page != 0 {
		mods = append(mods, qm.Offset(p.offset()))
	}

	mods = append(mods, qm.OrderBy("created_at DESC"))

	preloads := []qm.QueryMod{
		qm.Load("Actor", qm.WithDeleted()),
		qm.Load("SubjectGroup", qm.WithDeleted()),
		qm.Load("SubjectUser", qm.WithDeleted()),
		qm.Load("SubjectOrganization", qm.WithDeleted()),
		qm.Load("SubjectApplication", qm.WithDeleted()),
	}

	mods = append(mods, preloads...)

	events, err := models.AuditEvents(mods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching audit events", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing audit events: "+err.Error())

		return
	}

	d := float64(count) / float64(p.limitUsed())
	totalPages := int(math.Ceil(d))

	c.JSON(http.StatusOK, &EventsResponse{
		PageSize:         p.limitUsed(),
		PageCount:        len(events),
		TotalPages:       totalPages,
		Page:             p.Page,
		TotalRecordCount: count,
		Records:          events,
	})
}

// listGroupEvents returns the audit events from the database for a group as JSON
func (r *Router) listGroupEvents(c *gin.Context) {
	p := parsePagination(c)
	id := c.Param("id")

	// TODO filtering
	mods := []qm.QueryMod{
		qm.Where("subject_group_id = ?", id),
	}

	count, err := models.AuditEvents(mods...).Count(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching audit events", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing audit events: "+err.Error())

		return
	}

	mods = append(mods, qm.Limit(p.limitUsed()))

	if p.Page != 0 {
		mods = append(mods, qm.Offset(p.offset()))
	}

	mods = append(mods, qm.OrderBy("created_at DESC"))

	preloads := []qm.QueryMod{
		qm.Load("Actor", qm.WithDeleted()),
		qm.Load("SubjectGroup", qm.WithDeleted()),
		qm.Load("SubjectUser", qm.WithDeleted()),
		qm.Load("SubjectOrganization", qm.WithDeleted()),
		qm.Load("SubjectApplication", qm.WithDeleted()),
	}

	mods = append(mods, preloads...)

	events, err := models.AuditEvents(mods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching audit events", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error listing audit events: "+err.Error())

		return
	}

	d := float64(count) / float64(p.limitUsed())
	totalPages := int(math.Ceil(d))

	c.JSON(http.StatusOK, &EventsResponse{
		PageSize:         p.limitUsed(),
		PageCount:        len(events),
		TotalPages:       totalPages,
		Page:             p.Page,
		TotalRecordCount: count,
		Records:          events,
	})
}
