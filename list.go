package main

import (
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/tui"
)

// stringList is a repeatable string flag (flag.Value), e.g. -l backend -l urgent.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// listFilters holds the parsed flags for the `list` command's JQL builder.
type listFilters struct {
	assignee string
	reporter string
	priority string
	status   string
	project  string
	labels   []string
	created  string
	updated  string
	watching bool
	jql      string
	orderBy  string
	reverse  bool
}

// runList searches tickets using a flag-built JQL query, mirroring jira-cli's
// `issue list` flag set. Flags combine with AND; an explicit -jql clause is
// AND-ed in as well for anything the flags don't cover.
func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	f := listFilters{}
	var labels stringList
	fs.StringVar(&f.assignee, "a", "", `Filter by assignee: "me", "x" (unassigned), "~x" (assigned), name, or "~name" (not)`)
	fs.StringVar(&f.reporter, "r", "", `Filter by reporter: "me", name, or "~name" (not)`)
	fs.StringVar(&f.priority, "y", "", `Filter by priority: name, or "~name" (not)`)
	fs.StringVar(&f.status, "s", "", `Filter by status: name, or "~name" (not)`)
	fs.StringVar(&f.project, "p", "", "Filter by project key")
	fs.Var(&labels, "l", "Filter by label (repeatable)")
	fs.StringVar(&f.created, "created", "", `Relative or absolute created-date filter, e.g. "-7d", "week", "2026-01-01"`)
	fs.StringVar(&f.updated, "updated", "", `Relative or absolute updated-date filter, e.g. "-30m", "month"`)
	fs.BoolVar(&f.watching, "w", false, "Only tickets you are watching")
	fs.StringVar(&f.jql, "jql", "", "Raw JQL, AND-ed together with the other flags")
	fs.StringVar(&f.orderBy, "order-by", "", `Sort field (default "created")`)
	fs.BoolVar(&f.reverse, "reverse", false, "Reverse the sort order (ascending instead of descending)")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	f.labels = labels

	jql := buildListJQL(f)
	conn := mustConnect()
	result, err := api.SearchIssues(conn, api.SearchQuery{
		JQL:        jql,
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	})
	if err != nil {
		fatal("searching: %v", err)
	}
	if len(result.Issues) == 0 {
		fmt.Println("No tickets found.")
		return
	}
	fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Found %d ticket(s):", len(result.Issues))))
	fmt.Println()
	printTasks(result.Issues)
}

// buildListJQL assembles a JQL query string from listFilters. Each populated
// flag becomes an AND-ed clause; an empty listFilters produces a query that
// returns all issues ordered by created date, matching jira-cli's default.
func buildListJQL(f listFilters) string {
	var clauses []string
	if f.assignee != "" {
		clauses = append(clauses, negatableClause("assignee", resolveUserToken(f.assignee)))
	}
	if f.reporter != "" {
		clauses = append(clauses, negatableClause("reporter", resolveUserToken(f.reporter)))
	}
	if f.priority != "" {
		clauses = append(clauses, negatableClause("priority", f.priority))
	}
	if f.status != "" {
		clauses = append(clauses, negatableClause("status", f.status))
	}
	if f.project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", f.project))
	}
	if len(f.labels) > 0 {
		clauses = append(clauses, labelsClause(f.labels))
	}
	if f.created != "" {
		clauses = append(clauses, dateClause("created", f.created))
	}
	if f.updated != "" {
		clauses = append(clauses, dateClause("updated", f.updated))
	}
	if f.watching {
		clauses = append(clauses, "watcher = currentUser()")
	}
	if f.jql != "" {
		clauses = append(clauses, "("+f.jql+")")
	}

	jql := strings.Join(clauses, " AND ")
	orderBy := f.orderBy
	if orderBy == "" {
		orderBy = "created"
	}
	direction := "DESC"
	if f.reverse {
		direction = "ASC"
	}
	if jql == "" {
		return fmt.Sprintf("ORDER BY %s %s", orderBy, direction)
	}
	return fmt.Sprintf("%s ORDER BY %s %s", jql, orderBy, direction)
}

// resolveUserToken translates jira-cli's shorthand user tokens: "me" -> currentUser().
// Other values pass through unchanged (handled as quoted literals by negatableClause).
func resolveUserToken(v string) string {
	trimmed := strings.TrimPrefix(v, "~")
	if trimmed != "me" {
		return v
	}
	if strings.HasPrefix(v, "~") {
		return "~currentUser()"
	}
	return "currentUser()"
}

// negatableClause builds a JQL clause supporting jira-cli's shorthand:
//   - "x"      -> field is EMPTY
//   - "~x"     -> field is not EMPTY
//   - "~value" -> field != "value"
//   - "value"  -> field = "value"
//
// "currentUser()"-style function calls (from resolveUserToken) are emitted unquoted.
func negatableClause(field, value string) string {
	negate := strings.HasPrefix(value, "~")
	v := strings.TrimPrefix(value, "~")

	switch {
	case v == "x" && negate:
		return fmt.Sprintf("%s is not EMPTY", field)
	case v == "x":
		return fmt.Sprintf("%s is EMPTY", field)
	case negate:
		return fmt.Sprintf("%s != %s", field, jqlLiteral(v))
	default:
		return fmt.Sprintf("%s = %s", field, jqlLiteral(v))
	}
}

// jqlLiteral renders v as a JQL value: unquoted for function calls like
// currentUser(), quoted otherwise.
func jqlLiteral(v string) string {
	if strings.HasSuffix(v, "()") {
		return v
	}
	return fmt.Sprintf("%q", v)
}

// labelsClause builds a JQL "labels in (...)" clause from a label list.
func labelsClause(labels []string) string {
	quoted := make([]string, len(labels))
	for i, l := range labels {
		quoted[i] = fmt.Sprintf("%q", l)
	}
	return fmt.Sprintf("labels in (%s)", strings.Join(quoted, ", "))
}

// dateClause builds a JQL date comparison. Relative shorthand (leading "-", or
// bare "day"/"week"/"month"/"year") is emitted unquoted per Jira's JQL date
// function syntax; anything else is treated as an absolute date and quoted.
func dateClause(field, value string) string {
	switch value {
	case "day", "week", "month", "year":
		return fmt.Sprintf("%s >= startOf%s()", field, strings.ToUpper(value[:1])+value[1:])
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Sprintf("%s >= %s", field, value)
	}
	return fmt.Sprintf("%s >= %q", field, value)
}
