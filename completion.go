package main

import (
	"fmt"
	"strings"
)

// completionCommands lists every top-level subcommand and alias, used to build
// shell completion scripts. Kept in sync with the switch in main() by hand —
// there's no cobra-style command registry to generate it from.
var completionCommands = []string{
	"template", "te", "create", "cr", "my-tasks", "mt", "list", "ls", "state", "sta",
	"menu", "m", "epic", "open", "o", "update", "up", "last-comment", "lc", "attach", "at",
	"describe", "de", "toil-check", "tc", "toil", "point-check", "pc", "toil-sync", "ts",
	"conf", "subtask", "st", "serve-mcp", "diagnose", "diag", "clear-auth",
	"check-update", "cu", "self-update", "su", "version", "completion", "help",
}

const bashCompletionTemplate = `_jira_thing_completions() {
    local cur
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "%s" -- "$cur") )
    fi
}
complete -F _jira_thing_completions jira-thing
`

const zshCompletionTemplate = `#compdef jira-thing
_jira_thing() {
    local -a commands
    commands=(%s)
    _describe 'command' commands
}
_jira_thing
`

// runCompletion prints a shell completion script for the requested shell.
func runCompletion(args []string) {
	if len(args) < 1 {
		fatal("usage: jira-thing completion <bash|zsh>")
	}
	switch args[0] {
	case "bash":
		fmt.Printf(bashCompletionTemplate, strings.Join(completionCommands, " "))
	case "zsh":
		fmt.Printf(zshCompletionTemplate, strings.Join(quoteAll(completionCommands), " "))
	default:
		fatal("unsupported shell: %s (supported: bash, zsh)", args[0])
	}
}

// quoteAll wraps each string in single quotes, for embedding in a zsh array literal.
func quoteAll(items []string) []string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + s + "'"
	}
	return quoted
}
