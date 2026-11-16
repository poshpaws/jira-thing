package ops

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// defaultOrderBy is the sort field when a list names none.
const defaultOrderBy = "created"

// orderByPattern matches a JQL sort field: a name or a custom field such as cf[10020].
var orderByPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]*(\[[0-9]+\])?$`)

// ErrBadOrderBy means an order-by value wasn't a plain field name.
var ErrBadOrderBy = errors.New("order-by must be a field name such as created, updated, priority or cf[10020]")

// ListFilter is a ticket list query in jira-cli's shorthand. Each set field
// becomes a clause, AND-ed together. User, priority, status, type and
// resolution values accept "x" (empty), "~x" (not empty) and "~value" (not
// value); "me" means the current user.
type ListFilter struct {
	Assignee      string
	Reporter      string
	Priority      string
	Status        string
	Type          string
	Resolution    string
	Project       string
	Components    []string
	Labels        []string
	Created       string // on or after: "-7d", "week", "2026-01-01"
	CreatedBefore string // before, same forms
	Updated       string
	Watching      bool
	JQL           string // raw JQL, AND-ed with the rest
	OrderBy       string
	Reverse       bool
}

// orderByClause finds an ORDER BY in raw JQL.
var orderByClause = regexp.MustCompile(`(?i)\border\s+by\b`)

// BuildListJQL turns a filter into JQL, newest first unless Reverse. Raw JQL
// that brings its own ORDER BY, with no other filter, is passed through as-is.
func BuildListJQL(f ListFilter) (string, error) {
	if f.JQL != "" && orderByClause.MatchString(f.JQL) && onlyRawJQL(f) {
		return f.JQL, nil
	}
	orderBy := f.OrderBy
	if orderBy == "" {
		orderBy = defaultOrderBy
	}
	if !orderByPattern.MatchString(orderBy) {
		return "", fmt.Errorf("%w: %q", ErrBadOrderBy, orderBy)
	}
	direction := "DESC"
	if f.Reverse {
		direction = "ASC"
	}
	order := fmt.Sprintf("ORDER BY %s %s", orderBy, direction)
	if clauses := listClauses(f); len(clauses) > 0 {
		return strings.Join(clauses, " AND ") + " " + order, nil
	}
	return order, nil
}

// onlyRawJQL reports whether JQL is the filter's only setting.
func onlyRawJQL(f ListFilter) bool {
	return len(listClauses(f)) == 1 && f.OrderBy == "" && !f.Reverse
}

// listClauses builds one JQL clause per set filter field, in a fixed order.
func listClauses(f ListFilter) []string {
	var clauses []string
	add := func(ok bool, clause func() string) {
		if ok {
			clauses = append(clauses, clause())
		}
	}
	add(f.Assignee != "", func() string { return negatableClause("assignee", userToken(f.Assignee)) })
	add(f.Reporter != "", func() string { return negatableClause("reporter", userToken(f.Reporter)) })
	add(f.Priority != "", func() string { return negatableClause("priority", f.Priority) })
	add(f.Status != "", func() string { return negatableClause("status", f.Status) })
	add(f.Type != "", func() string { return negatableClause("issuetype", f.Type) })
	add(f.Resolution != "", func() string { return negatableClause("resolution", f.Resolution) })
	add(f.Project != "", func() string { return "project = " + jqlQuote(f.Project) })
	add(len(f.Components) > 0, func() string { return inClause("component", f.Components) })
	add(len(f.Labels) > 0, func() string { return inClause("labels", f.Labels) })
	add(f.Created != "", func() string { return dateClause("created", ">=", f.Created) })
	add(f.CreatedBefore != "", func() string { return dateClause("created", "<", f.CreatedBefore) })
	add(f.Updated != "", func() string { return dateClause("updated", ">=", f.Updated) })
	add(f.Watching, func() string { return "watcher = currentUser()" })
	add(f.JQL != "", func() string { return "(" + f.JQL + ")" })
	return clauses
}

// userToken maps "me" and "~me" to currentUser(); other values pass through.
func userToken(v string) string {
	switch v {
	case AssigneeMe:
		return "currentUser()"
	case "~" + AssigneeMe:
		return "~currentUser()"
	}
	return v
}

// negatableClause builds field = value, with "x" for EMPTY and a leading
// "~" negating. currentUser() is the only value left unquoted.
func negatableClause(field, value string) string {
	negate := strings.HasPrefix(value, "~")
	v := strings.TrimPrefix(value, "~")
	switch {
	case v == "x" && negate:
		return field + " is not EMPTY"
	case v == "x":
		return field + " is EMPTY"
	case negate:
		return field + " != " + jqlValue(v)
	}
	return field + " = " + jqlValue(v)
}

// jqlValue quotes v, except the currentUser() function.
func jqlValue(v string) string {
	if v == "currentUser()" {
		return v
	}
	return jqlQuote(v)
}

// jqlQuote renders v as a double-quoted JQL string, escaping quotes and backslashes.
func jqlQuote(v string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
}

// inClause builds field in ("a", "b").
func inClause(field string, values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = jqlQuote(v)
	}
	return fmt.Sprintf("%s in (%s)", field, strings.Join(quoted, ", "))
}

// dateClause compares a date field. "day", "week", "month" and "year" mean
// the start of the current one; anything else ("-7d", "2026-01-01") is
// quoted, which JQL accepts for relative and absolute dates alike.
func dateClause(field, op, value string) string {
	switch value {
	case "day", "week", "month", "year":
		return fmt.Sprintf("%s %s startOf%s()", field, op, strings.ToUpper(value[:1])+value[1:])
	}
	return fmt.Sprintf("%s %s %s", field, op, jqlQuote(value))
}
