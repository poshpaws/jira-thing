package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/tui"
)

// printList renders a ticket list in the format the flags ask for: JSON,
// CSV, a plain table (also chosen by -columns alone), or styled rows.
func printList(conn api.JiraConnection, issues []map[string]any, f listFlags, cols []string) {
	switch {
	case f.jsonOut && f.columns != "":
		printColumnsJSON(conn, issues, cols)
	case f.jsonOut:
		printTicketsJSON(conn, issues)
	case f.csv:
		printTicketCSV(conn, issues, cols, !f.noHeaders)
	case f.plain || f.columns != "":
		printTicketTable(conn, issues, cols, !f.noHeaders)
	case len(issues) == 0:
		fmt.Println("No tickets found.")
	default:
		fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Found %d ticket(s):", len(issues))))
		fmt.Println()
		printTasks(issues)
	}
}

// columnHeaders upper-cases column names for a header row.
func columnHeaders(cols []string) []string {
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = strings.ToUpper(c)
	}
	return headers
}

// printTicketTable prints a tab-aligned table with no styling, for reading
// or for cut and awk.
func printTicketTable(conn api.JiraConnection, issues []map[string]any, cols []string, headers bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if headers {
		fmt.Fprintln(w, strings.Join(columnHeaders(cols), "\t"))
	}
	for _, issue := range issues {
		fmt.Fprintln(w, strings.Join(ops.TicketRow(conn, issue, cols), "\t"))
	}
	if err := w.Flush(); err != nil {
		fatal("writing table: %v", err)
	}
}

// printTicketCSV prints RFC 4180 CSV. Cells that a spreadsheet would read as
// a formula are prefixed with a quote mark, since ticket text is untrusted.
func printTicketCSV(conn api.JiraConnection, issues []map[string]any, cols []string, headers bool) {
	w := csv.NewWriter(os.Stdout)
	if headers {
		_ = w.Write(columnHeaders(cols))
	}
	for _, issue := range issues {
		row := ops.TicketRow(conn, issue, cols)
		for i, cell := range row {
			row[i] = csvSafe(cell)
		}
		_ = w.Write(row)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fatal("writing CSV: %v", err)
	}
}

// csvSafe defuses spreadsheet formula injection: a cell starting with
// =, +, -, @, tab or carriage return gets a leading apostrophe.
func csvSafe(cell string) string {
	if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
		return "'" + cell
	}
	return cell
}

// printColumnsJSON prints the chosen columns as a JSON array of objects.
func printColumnsJSON(conn api.JiraConnection, issues []map[string]any, cols []string) {
	rows := make([]map[string]string, len(issues))
	for i, issue := range issues {
		row := map[string]string{}
		for j, v := range ops.TicketRow(conn, issue, cols) {
			row[cols[j]] = v
		}
		rows[i] = row
	}
	printJSONLine(rows)
}
