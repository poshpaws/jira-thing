package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"testing"

	"jira-thing/internal/api"
	mcpsrv "jira-thing/internal/mcpserver"
)

// commandTools maps every CLI command to the MCP tools that expose the same
// feature. jira-thing is MCP-first: a feature exists as an MCP tool, and the
// CLI and TUI call the same internal/ops code.
var commandTools = map[string][]string{
	"template":       {"fetch_template", "list_templates"},
	"create":         {"create_ticket", "create_tickets_batch"},
	"my-tasks":       {"my_tasks"},
	"list":           {"search_tickets"},
	"transition":     {"list_transitions", "transition_ticket"},
	"assign":         {"assign_ticket", "search_users"},
	"update":         {"add_comment"},
	"edit":           {"update_ticket"},
	"last-comment":   {"last_comment"},
	"attach":         {"add_attachment"},
	"describe":       {"describe_ticket"},
	"link-types":     {"list_link_types"},
	"link":           {"link_tickets"},
	"unlink":         {"unlink_tickets"},
	"remote-link":    {"add_remote_link"},
	"point-check":    {"check_story_points"},
	"toil-check":     {"list_toil_tickets"},
	"toil-sync":      {"sync_toil"},
	"subtask":        {"create_subtask"},
	"diagnose":       {"diagnose_connection"},
	"epic list":      {"list_epics"},
	"epic describe":  {"list_epic_issues"},
	"epic create":    {"create_epic"},
	"epic add":       {"add_to_epic"},
	"epic remove":    {"remove_from_epic"},
	"sprint list":    {"list_sprints", "list_sprint_issues"},
	"sprint add":     {"add_to_sprint"},
	"sprint explore": {"list_sprints", "list_sprint_issues"},
	"epic explore":   {"list_epics", "list_epic_issues"},
	"board list":     {"list_boards"},
	"worklog add":    {"add_worklog"},
	"delete":         {"delete_ticket"},
	"clone":          {"clone_ticket"},
	"me":             {"whoami"},
	"watch":          {"watch_ticket"},
	"unwatch":        {"unwatch_ticket"},
	"watchers":       {"list_watchers"},
	"project list":   {"list_projects"},
	"release list":   {"list_versions"},
	"conf browse":    {"confluence_browse"},
	"conf upload":    {"confluence_create_page", "confluence_update_page"},
	"conf export":    {"confluence_export_page"},
}

// localOnly commands concern the local binary, terminal or browser, or are
// TUI front ends over features already mapped above, so need no MCP tool.
// "epic" and "conf" are dispatchers whose subcommands are checked instead.
var localOnly = map[string]bool{
	"version": true, "check-update": true, "self-update": true, "help": true,
	"completion": true, "clear-auth": true, "serve-mcp": true,
	"menu": true, "state": true, "open": true, "epic": true, "conf": true, "sprint": true, "board": true, "project": true, "release": true, "worklog": true,
}

// dispatchers names the functions whose switch statements define commands,
// and the prefix their cases are reported under.
var dispatchers = map[string]string{"main": "", "runEpic": "epic ", "runConf": "conf ", "runSprint": "sprint ", "runBoard": "board ", "runProject": "project ", "runRelease": "release ", "runWorklog": "worklog "}

func TestEveryCommandHasMCPTools(t *testing.T) {
	tools := mcpsrv.NewServer("test", api.JiraConnection{}).ListTools()
	for _, cmd := range cliCommands(t) {
		if localOnly[cmd] {
			continue
		}
		mapped, ok := commandTools[cmd]
		if !ok {
			t.Errorf("command %q has no MCP tool: add one (via internal/ops) and map it in commandTools, "+
				"or list it in localOnly if it is purely local", cmd)
			continue
		}
		for _, name := range mapped {
			if _, exists := tools[name]; !exists {
				t.Errorf("command %q maps to MCP tool %q, which is not registered", cmd, name)
			}
		}
	}
}

func TestCommandToolsHasNoStaleEntries(t *testing.T) {
	known := map[string]bool{}
	for _, cmd := range cliCommands(t) {
		known[cmd] = true
	}
	for cmd := range commandTools {
		if !known[cmd] {
			t.Errorf("commandTools maps %q, which is no longer a CLI command", cmd)
		}
	}
}

// cliCommands parses the dispatch switches and returns each command's
// primary name (the first string in its case clause), with subcommands
// prefixed by their parent command.
func cliCommands(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	var cmds []string
	for _, file := range []string{"main.go", "epic.go", "sprint.go", "catalogue.go", "worklog.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		cmds = append(cmds, dispatchCases(f)...)
	}
	sort.Strings(cmds)
	if len(cmds) == 0 {
		t.Fatal("found no commands: has the dispatch moved?")
	}
	return cmds
}

// dispatchCases collects the first string literal of every case clause in
// the dispatcher functions declared in f.
func dispatchCases(f *ast.File) []string {
	var cmds []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		prefix, isDispatcher := dispatchers[fn.Name.Name]
		if !isDispatcher {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if name, ok := firstCaseString(n); ok {
				cmds = append(cmds, prefix+name)
			}
			return true
		})
	}
	return cmds
}

// firstCaseString returns the first string literal of a case clause.
func firstCaseString(n ast.Node) (string, bool) {
	clause, ok := n.(*ast.CaseClause)
	if !ok || len(clause.List) == 0 {
		return "", false
	}
	lit, ok := clause.List[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}
