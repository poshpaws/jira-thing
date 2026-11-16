package api

// searchPageSize is the most issues Jira returns per search page with fields.
const searchPageSize = 100

// SearchAll runs q page by page until limit issues are collected or Jira
// says there are no more. q.MaxResults is ignored in favour of limit.
func SearchAll(conn JiraConnection, q SearchQuery, limit int) (SearchResult, error) {
	var all SearchResult
	for len(all.Issues) < limit {
		q.MaxResults = min(searchPageSize, limit-len(all.Issues))
		page, err := SearchIssues(conn, q)
		if err != nil {
			return SearchResult{}, err
		}
		all.Issues = append(all.Issues, page.Issues...)
		if page.IsLast || page.NextPageToken == "" || len(page.Issues) == 0 {
			break
		}
		q.NextPageToken = page.NextPageToken
	}
	return all, nil
}
