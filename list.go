package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
)

// stringList is a repeatable string flag (flag.Value), e.g. -l backend -l urgent.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// listFlags holds the parsed flags for the list command.
type listFlags struct {
	filter     ops.ListFilter
	limit      int
	columns    string
	jsonOut    bool
	plain      bool
	csv        bool
	noHeaders  bool
	labels     stringList
	components stringList
}

// defineListFilterFlags registers the JQL filter flags on fs.
func defineListFilterFlags(fs *flag.FlagSet, f *listFlags) {
	fs.StringVar(&f.filter.Assignee, "a", "", `Filter by assignee: "me", "x" (unassigned), "~x" (assigned), name, or "~name" (not)`)
	fs.StringVar(&f.filter.Reporter, "r", "", `Filter by reporter: "me", name, or "~name" (not)`)
	fs.StringVar(&f.filter.Priority, "y", "", `Filter by priority: name, or "~name" (not)`)
	fs.StringVar(&f.filter.Status, "s", "", `Filter by status: name, or "~name" (not)`)
	fs.StringVar(&f.filter.Type, "t", "", `Filter by issue type: name, or "~name" (not)`)
	fs.StringVar(&f.filter.Resolution, "R", "", `Filter by resolution: name, "x" (unresolved), "~x" (resolved)`)
	fs.StringVar(&f.filter.Project, "p", "", "Filter by project key")
	fs.Var(&f.components, "C", "Filter by component (repeatable)")
	fs.Var(&f.labels, "l", "Filter by label (repeatable)")
	fs.StringVar(&f.filter.Created, "created", "", `Created on or after: "-7d", "week", "2026-01-01"`)
	fs.StringVar(&f.filter.CreatedBefore, "created-before", "", `Created before: "-30d", "month", "2026-01-01"`)
	fs.StringVar(&f.filter.Updated, "updated", "", `Updated on or after: "-30m", "month"`)
	fs.BoolVar(&f.filter.Watching, "w", false, "Only tickets you are watching")
	fs.StringVar(&f.filter.JQL, "jql", "", "Raw JQL, AND-ed together with the other flags")
	fs.StringVar(&f.filter.OrderBy, "order-by", "", `Sort field (default "created")`)
	fs.BoolVar(&f.filter.Reverse, "reverse", false, "Reverse the sort order (ascending instead of descending)")
}

// defineListOutputFlags registers the output format flags on fs.
func defineListOutputFlags(fs *flag.FlagSet, f *listFlags) {
	fs.IntVar(&f.limit, "limit", ops.DefaultListLimit, fmt.Sprintf("Most tickets to return (max %d)", ops.MaxListLimit))
	fs.StringVar(&f.columns, "columns", "", "Comma-separated columns for -plain/-csv/-json: "+strings.Join(ops.ColumnNames(), ","))
	fs.BoolVar(&f.jsonOut, "json", false, "Print matching tickets as a JSON array")
	fs.BoolVar(&f.plain, "plain", false, "Print a plain tab-aligned table, no colours")
	fs.BoolVar(&f.csv, "csv", false, "Print CSV")
	fs.BoolVar(&f.noHeaders, "no-headers", false, "Leave out the header row in -plain and -csv")
}

// parseListFlags parses runList's flag set, exiting on a parse error or
// conflicting output flags.
func parseListFlags(args []string) listFlags {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	var f listFlags
	defineListFilterFlags(fs, &f)
	defineListOutputFlags(fs, &f)
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal("usage: jira-thing list [filters] [-limit N] [-plain|-csv|-json] [-columns a,b] [-no-headers]")
	}
	f.filter.Labels, f.filter.Components = f.labels, f.components
	if countTrue(f.jsonOut, f.plain, f.csv) > 1 {
		fatal("pick one of -json, -plain or -csv")
	}
	return f
}

// countTrue counts the set flags.
func countTrue(flags ...bool) int {
	n := 0
	for _, b := range flags {
		if b {
			n++
		}
	}
	return n
}

// runList searches tickets using a flag-built JQL query, mirroring jira-cli's
// `issue list` flag set. Flags combine with AND; an explicit -jql clause is
// AND-ed in as well for anything the flags don't cover.
func runList(args []string) {
	f := parseListFlags(args)
	cols, err := ops.ParseColumns(f.columns)
	if err != nil {
		fatal("%v", err)
	}
	conn := mustConnect()
	res, err := ops.ListTickets(conn, ops.ListRequest{Filter: f.filter, Columns: cols, Limit: f.limit})
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	printList(conn, res.Issues, f, cols)
}

// listedTicket is the JSON shape -json prints for each matching ticket.
type listedTicket struct {
	Key      string `json:"key"`
	URL      string `json:"url"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Updated  string `json:"updated"`
}

// printTicketsJSON prints issues as a JSON array, "[]" when there are none —
// scripts can always parse the output as an array, with no prose fallback.
func printTicketsJSON(conn api.JiraConnection, issues []map[string]any) {
	tickets := issuesToTickets(issues)
	listed := make([]listedTicket, len(tickets))
	for i, t := range tickets {
		listed[i] = listedTicket{
			Key: t.Key, URL: conn.BaseURL + "/browse/" + t.Key,
			Summary: t.Summary, Status: t.Status, Priority: t.Priority, Updated: t.Updated,
		}
	}
	body, err := json.MarshalIndent(listed, "", "  ")
	if err != nil {
		fatal("rendering JSON result: %v", err)
	}
	fmt.Println(string(body))
}
