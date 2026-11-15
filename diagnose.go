package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/tui"
)

// runDiagnose tests Jira API connectivity using stored credentials.
func runDiagnose(args []string) {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	userID := fs.Bool("userid", false, "Print only the current user's accountId")
	teamKey := fs.String("team", "", "Fetch TICKET-KEY and print its Team custom field value")
	teamField := fs.String("team-field", "customfield_10001", "Custom field ID holding the Team value")
	findField := fs.String("find-field", "", "Search the Jira field registry by name (e.g. \"team\")")
	listFields := fs.Bool("list-fields", false, "Print every field on the Jira instance")
	_ = fs.Parse(args)

	if *listFields {
		runDiagnoseListFields()
		return
	}

	if *findField != "" {
		runDiagnoseFindField(*findField)
		return
	}

	if *userID {
		me, err := ops.WhoAmI(mustConnect())
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(me.AccountID)
		return
	}

	if *teamKey != "" {
		runDiagnoseTeam(*teamKey, *teamField)
		return
	}

	runConnectionChecks()
}

// runConnectionChecks loads the stored credentials and reports each
// connectivity check in turn.
func runConnectionChecks() {
	fmt.Println("Running diagnostics...")
	fmt.Println()
	conn, err := buildConnection()
	if err != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗ Credentials:"), err.Error())
		return
	}
	printDiagnosis(ops.Diagnose(conn))
}

// printDiagnosis reports each check in order, stopping at the first failure.
func printDiagnosis(d ops.Diagnosis) {
	fmt.Printf("  %s\n", tui.SuccessStyle.Render("✓ Credentials loaded from keyring"))
	fmt.Printf("    URL:   %s\n", d.URL)
	fmt.Printf("    Email: %s\n", d.Email)
	fmt.Printf("    Token: %s\n", d.MaskedToken)
	if d.TokenErr != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗ Token format:"), d.TokenErr.Error())
		return
	}
	fmt.Printf("  %s\n", tui.SuccessStyle.Render("✓ Token format valid"))
	fmt.Println()
	if d.APIErr != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗ API connection:"), formatDiagError(d.APIErr, d.Hint))
		return
	}
	fmt.Printf("  %s\n", tui.SuccessStyle.Render("✓ API connection successful"))
	fmt.Printf("    User:      %s\n", d.User.DisplayName)
	fmt.Printf("    AccountId: %s\n", d.User.AccountID)
	fmt.Println()
	fmt.Println(tui.SuccessStyle.Render("All checks passed."))
}

// runDiagnoseFindField looks up every field on the Jira instance and prints those
// whose name contains search (case-insensitive), so the correct customfield ID
// can be confirmed instead of guessed.
func runDiagnoseFindField(search string) {
	fields := fetchSortedFields()
	needle := strings.ToLower(search)
	matched := make([]api.Field, 0, len(fields))
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f.Name), needle) {
			matched = append(matched, f)
		}
	}
	if len(matched) == 0 {
		fmt.Printf("No fields matching %q found.\n", search)
		return
	}
	printFieldTable(matched)
	fmt.Printf("\n%d matching field(s). Use the id above in -team-field / your config filter.\n", len(matched))
}

// runDiagnoseListFields prints every field known to the Jira instance, sorted by name.
func runDiagnoseListFields() {
	fields := fetchSortedFields()
	printFieldTable(fields)
	fmt.Printf("\n%d field(s) total.\n", len(fields))
}

// fetchSortedFields retrieves all fields and sorts them alphabetically by name.
func fetchSortedFields() []api.Field {
	conn := mustConnect()
	fields, err := api.FetchFields(conn)
	if err != nil {
		fatal("fetching field list: %v", err)
	}
	sort.Slice(fields, func(i, j int) bool {
		return strings.ToLower(fields[i].Name) < strings.ToLower(fields[j].Name)
	})
	return fields
}

// printFieldTable renders fields as an aligned id/name/kind table.
func printFieldTable(fields []api.Field) {
	for _, f := range fields {
		kind := "system"
		if f.Custom {
			kind = "custom"
		}
		fmt.Printf("  %-20s %-35s (%s)\n", f.ID, f.Name, kind)
	}
}

// runDiagnoseTeam fetches issueKey and prints the raw value of teamField (the Team
// custom field), so the correct ID can be found for a JQL filter or config file.
func runDiagnoseTeam(issueKey, teamField string) {
	conn := mustConnect()
	issue, err := api.FetchIssue(conn, issueKey)
	if err != nil {
		fatal("fetching %s: %v", issueKey, err)
	}
	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		fatal("unexpected response shape for %s: no fields", issueKey)
	}
	value, present := fields[teamField]
	if !present {
		fatal("%s has no field %s", issueKey, teamField)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal("encoding %s value: %v", teamField, err)
	}
	fmt.Printf("%s (%s):\n%s\n", teamField, issueKey, raw)
	printTeamID(value)
}

// printTeamID extracts and prints the team's id/name from the common Jira Team field shapes.
func printTeamID(value any) {
	obj, ok := value.(map[string]any)
	if !ok {
		return
	}
	id, hasID := obj["id"]
	name, hasName := obj["name"]
	switch {
	case hasID && hasName:
		fmt.Printf("\n  → team ID: %v  (name: %v)\n", id, name)
	case hasID:
		fmt.Printf("\n  → team ID: %v\n", id)
	}
}

// formatDiagError appends the fix hint, if any, under the API error.
func formatDiagError(err error, hint string) string {
	if hint == "" {
		return err.Error()
	}
	return err.Error() + "\n    → " + hint
}
