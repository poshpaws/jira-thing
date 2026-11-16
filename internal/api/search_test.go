package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"jira-thing/internal/api"
)

// pagedSearch serves total issues in pages, recording each request body.
func pagedSearch(t *testing.T, total int, bodies *[]map[string]any) *httptest.Server {
	t.Helper()
	served := 0
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*bodies = append(*bodies, body)
		n := min(int(body["maxResults"].(float64)), total-served)
		issues := make([]map[string]any, n)
		for i := range issues {
			issues[i] = map[string]any{"key": fmt.Sprintf("P-%d", served+i+1)}
		}
		served += n
		resp := map[string]any{"issues": issues, "isLast": served >= total}
		if served < total {
			resp["nextPageToken"] = fmt.Sprintf("tok%d", served)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSearchAll(t *testing.T) {
	tests := []struct {
		name             string
		total, limit     int
		wantIssues       int
		wantPages        int
		wantSecondToken  string
		wantLastPageSize float64
	}{
		{"stops at limit", 250, 150, 150, 2, "tok100", 50},
		{"stops at last page", 120, 500, 120, 2, "tok100", 100},
		{"single page", 30, 100, 30, 1, "", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodies []map[string]any
			srv := pagedSearch(t, tt.total, &bodies)
			defer srv.Close()
			res, err := api.SearchAll(conn(srv.URL), api.SearchQuery{JQL: "project = P"}, tt.limit)
			if err != nil || len(res.Issues) != tt.wantIssues || len(bodies) != tt.wantPages {
				t.Fatalf("issues=%d pages=%d err=%v", len(res.Issues), len(bodies), err)
			}
			last := bodies[len(bodies)-1]
			if tt.wantSecondToken != "" && last["nextPageToken"] != tt.wantSecondToken || last["maxResults"] != tt.wantLastPageSize {
				t.Errorf("last request = %v", last)
			}
		})
	}
}
