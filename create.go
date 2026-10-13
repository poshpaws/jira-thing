package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// runCreate loads a template (or a batch file of templates) and creates one
// or more Jira tickets. With -t, summary/description/parent already present
// in the template are used as-is; missing fields are prompted for unless
// -non-interactive/-y is set, in which case a missing summary is a fatal
// error. With -f, every entry in the JSON array is created non-interactively
// and the command reports how many succeeded.
func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	batchPath := fs.String("f", "", "Path to a JSON array of ticket templates for batch create")
	descriptionFile := fs.String("description-file", "", "Path to a markdown file for the description (overrides template/prompt)")
	nonInteractive := fs.Bool("non-interactive", false, "Fail instead of prompting for missing required fields")
	fs.BoolVar(nonInteractive, "y", false, "Alias for -non-interactive")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	if *batchPath != "" {
		runBatchCreate(*batchPath)
		return
	}

	tmpl, err := template.Load(*templatePath)
	if err != nil {
		if *templatePath == "" {
			fatal("no template found in search path.\n\nCreate one first:\n  jira-thing template <TICKET-KEY> [-o ticket_template.json]\n\nThen place ticket_template.json in one of:\n  - current directory\n  - same directory as the jira-thing binary\n  - $XDG_CONFIG_HOME/jira-thing/\n  - ~/.config/jira-thing/\n\nOr specify a path directly:\n  jira-thing create -t /path/to/template.json")
		}
		fatal("loading template: %v", err)
	}
	if err := resolveCreateFields(tmpl, *descriptionFile, *nonInteractive); err != nil {
		fatal("%v", err)
	}

	conn := mustConnect()
	key, err := createTicket(conn, tmpl)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Created ticket:"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// resolveCreateFields fills in summary and description on tmpl: an explicit
// -description-file always wins for the description; otherwise an existing
// template value is used; anything still missing is prompted for unless
// nonInteractive is set.
func resolveCreateFields(tmpl map[string]any, descriptionFile string, nonInteractive bool) error {
	if descriptionFile != "" {
		data, err := os.ReadFile(descriptionFile) // #nosec G304 -- user-supplied CLI input
		if err != nil {
			return fmt.Errorf("reading description file: %w", err)
		}
		tmpl["description"] = string(data)
	}

	summary, err := resolveSummary(tmpl, nonInteractive)
	if err != nil {
		return err
	}
	description := resolveDescription(tmpl, nonInteractive)

	tmpl["summary"] = summary
	if description != "" {
		tmpl["description"] = buildDescription(description)
	} else {
		delete(tmpl, "description")
	}
	return nil
}

// resolveSummary returns the template's summary if already set, otherwise
// prompts for one; in non-interactive mode a missing summary is a fatal error.
func resolveSummary(tmpl map[string]any, nonInteractive bool) (string, error) {
	if s, ok := tmpl["summary"].(string); ok && strings.TrimSpace(s) != "" {
		return s, nil
	}
	if nonInteractive {
		return "", fmt.Errorf(`summary is required: set "summary" in the template, or omit -non-interactive to be prompted`)
	}
	summary := promptLine("Enter ticket summary: ")
	if summary == "" {
		return "", fmt.Errorf("summary is required")
	}
	return summary, nil
}

// resolveDescription returns the template's description if already set,
// otherwise prompts for one. Description is optional, so a non-interactive
// run with none given simply creates the ticket without one.
func resolveDescription(tmpl map[string]any, nonInteractive bool) string {
	if d, ok := tmpl["description"].(string); ok && d != "" {
		return d
	}
	if nonInteractive {
		return ""
	}
	return promptLine("Enter ticket description: ")
}

// createTicket strips non-updatable fields, resolves a self-assignee marker,
// extracts an optional "parent" ticket key, creates the issue, links it to
// the parent if one was given, and returns the new ticket's key.
func createTicket(conn api.JiraConnection, tmpl map[string]any) (string, error) {
	parentKey, _ := tmpl["parent"].(string)
	delete(tmpl, "parent")
	template.StripExcludedFields(tmpl)

	if tmpl["assignee"] == template.AssigneeSelf {
		me, err := api.FetchMyself(conn)
		if err != nil {
			return "", fmt.Errorf("fetching current user: %w", err)
		}
		template.ResolveAssignee(tmpl, me["accountId"].(string))
	}

	result, err := api.CreateIssue(conn, tmpl)
	if err != nil {
		return "", fmt.Errorf("creating issue: %w", err)
	}
	key := getString(result, "key")

	if parentKey != "" {
		if err := api.AddIssuesToEpic(conn, parentKey, []string{key}); err != nil {
			return key, fmt.Errorf("created %s but failed to link to parent %s: %w", key, parentKey, err)
		}
	}
	return key, nil
}

// runBatchCreate reads a JSON array of ticket templates from path and creates
// each one non-interactively, continuing past per-ticket failures and
// reporting how many of the total succeeded.
func runBatchCreate(path string) {
	data, err := os.ReadFile(path) // #nosec G304 -- user-supplied CLI input
	if err != nil {
		fatal("reading batch file: %v", err)
	}
	var tickets []map[string]any
	if err := json.Unmarshal(data, &tickets); err != nil {
		fatal("parsing batch file as a JSON array: %v", err)
	}
	if len(tickets) == 0 {
		fatal("no tickets found in %s", path)
	}

	conn := mustConnect()
	fmt.Printf("Creating %d ticket(s) from %s...\n", len(tickets), path)
	created := 0
	for i, tmpl := range tickets {
		key, err := createBatchTicket(conn, tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		fmt.Printf("  %s %s  %s\n", tui.SuccessStyle.Render("Created:"), tui.KeyStyle.Render(key), getString(tmpl, "summary"))
		created++
	}
	fmt.Printf("\n%s %d of %d ticket(s) created\n", tui.SuccessStyle.Render("Done:"), created, len(tickets))
}

// createBatchTicket resolves fields non-interactively and creates a single
// ticket from a batch entry.
func createBatchTicket(conn api.JiraConnection, tmpl map[string]any) (string, error) {
	if err := resolveCreateFields(tmpl, "", true); err != nil {
		return "", err
	}
	return createTicket(conn, tmpl)
}

// promptAssigneeChoice asks the user whether to use self or the original assignee in the template.
// If no assignee is present, it defaults to self without prompting.
func promptAssigneeChoice(tmpl map[string]any) {
	assignee, ok := tmpl["assignee"].(map[string]any)
	if !ok {
		tmpl["assignee"] = template.AssigneeSelf
		return
	}
	displayName, _ := assignee["displayName"].(string)
	if displayName == "" {
		displayName = "original user"
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Assignee: (1) current user (self) or (2) %s? [1/2]: ", displayName)
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(line) == "2" {
		fmt.Printf("Keeping assignee: %s\n", displayName)
		return
	}
	tmpl["assignee"] = template.AssigneeSelf
	fmt.Println("Assignee set to current user (resolved at ticket creation time)")
}

// buildDescription converts markdown text to the Jira Atlassian Document Format structure.
func buildDescription(text string) map[string]any {
	return markdownToADF(text)
}
