package ops

import (
	"fmt"
	"slices"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
)

// listColumn is one column a ticket list can show: the Jira field it needs
// (empty for key and url) and how to read it from an issue.
type listColumn struct {
	field string
	value func(conn api.JiraConnection, issue map[string]any) string
}

// dateLen is the length of the YYYY-MM-DD prefix of a Jira timestamp.
const dateLen = len("2006-01-02")

// listColumns are the columns ListTickets can return, by name.
var listColumns = map[string]listColumn{
	"key":        {"", func(_ api.JiraConnection, i map[string]any) string { return str(i, "key") }},
	"url":        {"", func(c api.JiraConnection, i map[string]any) string { return IssueURL(c, str(i, "key")) }},
	"summary":    {"summary", fieldValue("summary", "")},
	"status":     {"status", fieldValue("status", "name")},
	"priority":   {"priority", fieldValue("priority", "name")},
	"type":       {"issuetype", fieldValue("issuetype", "name")},
	"assignee":   {"assignee", fieldValue("assignee", "displayName")},
	"reporter":   {"reporter", fieldValue("reporter", "displayName")},
	"resolution": {"resolution", fieldValue("resolution", "name")},
	"labels":     {"labels", listValue("labels", "")},
	"components": {"components", listValue("components", "name")},
	"created":    {"created", dateValue("created")},
	"updated":    {"updated", dateValue("updated")},
}

// DefaultColumns are shown when no columns are asked for.
var DefaultColumns = []string{"key", "type", "status", "priority", "assignee", "updated", "summary"}

// ColumnNames lists every valid column, sorted.
func ColumnNames() []string {
	names := make([]string, 0, len(listColumns))
	for n := range listColumns {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// ParseColumns reads a comma-separated column list; empty means DefaultColumns.
func ParseColumns(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return DefaultColumns, nil
	}
	var cols []string
	for _, c := range strings.Split(s, ",") {
		c = strings.ToLower(strings.TrimSpace(c))
		if _, ok := listColumns[c]; !ok {
			return nil, fmt.Errorf("unknown column %q; choose from %s", c, strings.Join(ColumnNames(), ", "))
		}
		cols = append(cols, c)
	}
	return cols, nil
}

// columnFields returns the Jira fields the columns need, without repeats.
func columnFields(cols []string) []string {
	fields := []string{}
	for _, c := range cols {
		if f := listColumns[c].field; f != "" && !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	return fields
}

// TicketRow returns an issue's values for cols, each made safe to show on one line.
func TicketRow(conn api.JiraConnection, issue map[string]any, cols []string) []string {
	row := make([]string, len(cols))
	for i, c := range cols {
		row[i] = termsafe.Line(listColumns[c].value(conn, issue))
	}
	return row
}

// fields returns an issue's "fields" object, or an empty one.
func fields(issue map[string]any) map[string]any {
	f, _ := issue["fields"].(map[string]any)
	if f == nil {
		return map[string]any{}
	}
	return f
}

// fieldValue reads fields[name], or fields[name][sub] when sub is set.
func fieldValue(name, sub string) func(api.JiraConnection, map[string]any) string {
	return func(_ api.JiraConnection, issue map[string]any) string {
		if sub == "" {
			return str(fields(issue), name)
		}
		return nestedStr(fields(issue), name, sub)
	}
}

// listValue joins a list field with commas, reading sub from each object.
func listValue(name, sub string) func(api.JiraConnection, map[string]any) string {
	return func(_ api.JiraConnection, issue map[string]any) string {
		items, _ := fields(issue)[name].([]any)
		parts := make([]string, 0, len(items))
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				parts = append(parts, str(m, sub))
			} else if s, ok := it.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	}
}

// dateValue reads a timestamp field as YYYY-MM-DD.
func dateValue(name string) func(api.JiraConnection, map[string]any) string {
	return func(_ api.JiraConnection, issue map[string]any) string {
		v := str(fields(issue), name)
		if len(v) > dateLen {
			return v[:dateLen]
		}
		return v
	}
}
