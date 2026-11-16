package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestRunCompletion_Bash(t *testing.T) {
	out := captureStdout(func() { runCompletion([]string{"bash"}) })
	if !containsAll(out, "_jira_thing_completions", "complete -F", "template", "menu", "epic") {
		t.Errorf("bash completion missing expected content: %s", out)
	}
}

func TestRunCompletion_Zsh(t *testing.T) {
	out := captureStdout(func() { runCompletion([]string{"zsh"}) })
	if !containsAll(out, "#compdef jira-thing", "_describe", "'template'", "'menu'") {
		t.Errorf("zsh completion missing expected content: %s", out)
	}
}

func TestRunCompletion_UnsupportedShell(t *testing.T) {
	didExit := captureExit(func() { runCompletion([]string{"fish"}) })
	if !didExit {
		t.Error("expected fatal() to be called for unsupported shell")
	}
}

func TestRunCompletion_NoArgs(t *testing.T) {
	didExit := captureExit(func() { runCompletion([]string{}) })
	if !didExit {
		t.Error("expected fatal() to be called with no args")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// completionDispatchers maps each function whose switch defines commands to
// the command it completes sub-commands for ("" for top-level commands).
var completionDispatchers = map[string]string{
	"main": "", "runEpic": "epic", "runSprint": "sprint", "runBoard": "board", "runProject": "project",
	"runRelease": "release", "runWorklog": "worklog", "runConf": "conf", "runCompletion": "completion",
}

// allCaseStrings returns every string literal in fn's case clauses, aliases included.
func allCaseStrings(fn *ast.FuncDecl) []string {
	var names []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		clause, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range clause.List {
			if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				names = append(names, s)
			}
		}
		return true
	})
	return names
}

// completionFor returns the words completed after cmd ("" for the first word).
func completionFor(cmd string) []string {
	if cmd == "" {
		return completionCommands
	}
	for _, c := range completionSubcommands {
		if slices.Contains(c.names, cmd) {
			return c.subs
		}
	}
	return nil
}

func TestCompletion_CoversEveryDispatchCase(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, file := range []string{"main.go", "epic.go", "sprint.go", "catalogue.go", "worklog.go", "completion.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			cmd, isDispatcher := completionDispatchers[fn.Name.Name]
			if !isDispatcher {
				continue
			}
			seen[fn.Name.Name] = true
			for _, name := range allCaseStrings(fn) {
				if strings.HasPrefix(name, "-") { // -h, --version: flags, not commands
					continue
				}
				if !slices.Contains(completionFor(cmd), name) {
					t.Errorf("%q (handled by %s) is missing from shell completion", name, fn.Name.Name)
				}
			}
		}
	}
	if len(seen) != len(completionDispatchers) {
		t.Errorf("found dispatchers %v; has one moved file?", seen)
	}
}

func TestRunCompletion_Subcommands(t *testing.T) {
	bash := captureStdout(func() { runCompletion([]string{"bash"}) })
	zsh := captureStdout(func() { runCompletion([]string{"zsh"}) })
	if !strings.Contains(bash, `sprint|sp) COMPREPLY=( $(compgen -W "list add" -- "$cur") ) ;;`) {
		t.Errorf("bash sub-commands missing:\n%s", bash)
	}
	if !strings.Contains(zsh, "worklog|wl) compadd add ;;") {
		t.Errorf("zsh sub-commands missing:\n%s", zsh)
	}
}
