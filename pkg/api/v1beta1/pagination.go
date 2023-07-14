package v1beta1

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// maxPaginationSize represents the maximum number of records that can be returned per page
	maxPaginationSize = 1000
	// defaultPaginationSize represents the default number of records that are returned per page
	defaultPaginationSize = 100
	// desc represents the sql keyword DESC as a string
	desc = "desc"
	// asc represents the sql keyword ASC as a string
	asc = "asc"
)

// PaginationParams is the params to be parsed from query params from a gin context
type PaginationParams struct {
	Limit      int    `json:"limit,omitempty"`
	Last       bool   `json:"last,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortOrder  string `json:"sort_order,omitempty"`
	OrderBy    string `json:"orderby,omitempty"`
}

// PaginationResponse is the response given to GET requests requiring pagination
type PaginationResponse[modelType any] struct {
	TotalRecordCount int64       `json:"total_record_count,omitempty"`
	NextCursor       string      `json:"next_cursor,omitempty"`
	PrevCursor       string      `json:"prev_cursor,omitempty"`
	Records          []modelType `json:"records"`
}

// parsePagination parses relevant query params into PaginationParams object
func parsePagination(c *gin.Context) (PaginationParams, error) {
	p := PaginationParams{
		SortBy:    "id",
		SortOrder: asc,
		Limit:     defaultPaginationSize,
	}

	if _, ok := c.GetQuery("last"); ok {
		p.Last = true
	}

	if limitStr, ok := c.GetQuery("limit"); ok {
		val, err := strconv.Atoi(limitStr)

		switch {
		case err != nil:
			return PaginationParams{}, invalidQueryParameterValue("limit, " + limitStr)
		case val > maxPaginationSize:
			p.Limit = maxPaginationSize
		case val <= 0:
			p.Limit = defaultPaginationSize
		default:
			p.Limit = val
		}
	}

	if nextCursor, ok := c.GetQuery("next_cursor"); ok {
		p.NextCursor = nextCursor
	}

	if prevCursor, ok := c.GetQuery("prev_cursor"); ok {
		p.PrevCursor = prevCursor
	}

	if sortBy, ok := c.GetQuery("sort_by"); ok {
		p.SortBy = sortBy

		if sortOrder, ok := c.GetQuery(("sort_order")); ok {
			if strings.EqualFold(sortOrder, asc) && strings.EqualFold(sortOrder, desc) {
				return PaginationParams{}, invalidQueryParameterValue("sort_order, " + sortOrder)
			}

			p.SortOrder = sortOrder
		}
	}

	// if we want last page, reverse sort direction to get last N records, similar for prev cursor
	if p.Last || p.PrevCursor != "" {
		if p.SortOrder == asc {
			p.SortOrder = desc
		} else {
			p.SortOrder = asc
		}
	}

	if p.Last && (p.PrevCursor != "" || p.NextCursor != "") {
		return PaginationParams{}, invalidQueryParameterValue("get_last cannot be used with next_cursor or prev_cursor")
	}

	// both cannot be set
	if p.PrevCursor != "" && p.NextCursor != "" {
		return PaginationParams{}, invalidQueryParameterValue("prev_cursor: " + p.PrevCursor + " next_cursor: " + p.NextCursor)
	}

	return p, nil
}

// getCursorClause takes a format string as input to wrap around the column and arg and returns a where clause
func (p *PaginationParams) getCursorClause(format string) (query, param string, ok bool) {
	col := p.SortBy
	cursor := p.NextCursor

	if p.NextCursor == "" && p.PrevCursor != "" {
		cursor = p.PrevCursor
	} else if p.NextCursor == "" && p.PrevCursor == "" {
		return "", "", false
	}

	if format == "" {
		format = "%s"
	}

	if strings.EqualFold(asc, p.SortOrder) {
		return fmt.Sprintf(format, col) + " > " + fmt.Sprintf(format, "?"), cursor, true
	}

	// else p.sortOrder == desc
	return fmt.Sprintf(format, col) + " < " + fmt.Sprintf(format, "?"), cursor, true
}

func getNextCursor[recordType any](records []*recordType, p *PaginationParams, hasMoreRecords bool) (string, error) {
	if len(records) == 0 || p.Last {
		return "", nil
	}

	// if we have more records and previously called next_cursor
	if p.NextCursor != "" && !hasMoreRecords {
		return "", nil
	}

	// edge case when first call to /users is made and returned items < limit
	if p.NextCursor == "" && p.PrevCursor == "" && !hasMoreRecords {
		return "", nil
	}

	cursor, err := getStructValueByString(records[len(records)-1], p.SortBy)
	if err != nil {
		return "", err
	}

	return cursor, nil
}

func getPrevCursor[recordType any](records []*recordType, p *PaginationParams, hasMoreRecords bool) (string, error) {
	if len(records) == 0 {
		return "", nil
	}

	// if we have no more records and previously called prev_cursor
	if (p.PrevCursor != "" || p.Last) && !hasMoreRecords {
		return "", nil
	}

	// when first call to /users is made, we get the beginning of the records
	if p.PrevCursor == "" && p.NextCursor == "" && !p.Last {
		return "", nil
	}

	// requires tight coupling between column names and record fields
	cursor, err := getStructValueByString(records[0], p.SortBy)
	if err != nil {
		return "", err
	}

	return cursor, nil
}

// getStructValueByString retrieves a value from a struct using a string for the key
// s can be pointer to a struct or a struct
func getStructValueByString(s any, key string) (retval string, err error) {
	if !(reflect.TypeOf(s).Kind() == reflect.Pointer || reflect.TypeOf(s).Kind() == reflect.Struct) {
		return "", invalidFunctionParameter("cannot use interface that is not of type struct")
	}

	r := reflect.ValueOf(s)
	val := reflect.Indirect(r).FieldByNameFunc(func(n string) bool { return strings.EqualFold(key, n) })

	if !val.IsValid() {
		return "", invalidFunctionParameter("key not in struct: " + key)
	}

	// cast to string irrespective of type since sql will cast types automatically (when possible)
	return fmt.Sprint(val.Interface()), nil
}
