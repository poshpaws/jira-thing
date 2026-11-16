package api

import (
	"net/http"
	"net/url"
	"strconv"
)

const (
	// pageSize is the page size asked for from paginated list endpoints.
	pageSize = 50
	// maxPages bounds pagination, so a misbehaving server can't loop us forever.
	maxPages = 40
)

// page is one page of a Jira list endpoint that returns {values, isLast}.
type page[T any] struct {
	Values []T   `json:"values"`
	IsLast *bool `json:"isLast"`
}

// last reports whether this is the final page: Jira says so with isLast, and
// a short page means the same when it is missing.
func (p page[T]) last() bool {
	if p.IsLast != nil {
		return *p.IsLast
	}
	return len(p.Values) < pageSize
}

// fetchAllPages GETs endpoint page by page, adding startAt and maxResults to
// params, and returns every value. params must not be nil.
func fetchAllPages[T any](conn JiraConnection, endpoint string, params url.Values) ([]T, error) {
	var all []T
	for range maxPages {
		params.Set("startAt", strconv.Itoa(len(all)))
		params.Set("maxResults", strconv.Itoa(pageSize))
		req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint + "?" + params.Encode()})
		if err != nil {
			return nil, err
		}
		var p page[T]
		if err := executeRequest(req, &p); err != nil {
			return nil, err
		}
		all = append(all, p.Values...)
		if p.last() || len(p.Values) == 0 {
			break
		}
	}
	return all, nil
}
