package main

import (
	"fmt"
	"strings"
)

// completionCommands lists every top-level subcommand and alias, used to build
// shell completion scripts. Kept in sync with the switch in main() by hand —
// there's no cobra-style command registry to generate it from.
var completionCommands = []string{
	"template", "te", "create", "cr", "my-tasks", "mt", "list", "ls", "state", "sta", "transition", "tr",
	"assign", "as",
	"menu", "m", "epic", "sprint", "sp", "board", "project", "release", "worklog", "wl", "delete", "clone", "me", "watch", "unwatch", "watchers", "open", "o", "update", "up", "edit", "ed", "last-comment", "lc", "attach", "at",
	"describe", "de", "link-types", "lt", "link", "ln", "unlink", "ul", "remote-link", "rl",
	"toil-check", "tc", "toil", "point-check", "pc", "toil-sync", "ts",
	"conf", "subtask", "st", "serve-mcp", "diagnose", "diag", "clear-auth",
	"check-update", "cu", "self-update", "su", "version", "completion", "help",
}

// completionSubcommands lists the sub-commands of each dispatching command,
// completed as the second word. names holds the command and its aliases.
var completionSubcommands = []struct{ names, subs []string }{
	{[]string{"template", "te"}, []string{"list"}},
	{[]string{"epic"}, []string{"list", "describe", "explore", "create", "add", "remove"}},
	{[]string{"sprint", "sp"}, []string{"list", "add", "explore"}},
	{[]string{"board"}, []string{"list"}},
	{[]string{"project"}, []string{"list"}},
	{[]string{"release"}, []string{"list"}},
	{[]string{"worklog", "wl"}, []string{"add"}},
	{[]string{"conf"}, []string{"browse", "br", "upload", "up", "export", "exp"}},
	{[]string{"completion"}, []string{"bash", "zsh"}},
}

const bashCompletionTemplate = `_jira_thing_completions() {
    local cur
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "%s" -- "$cur") )
    elif [ "$COMP_CWORD" -eq 2 ]; then
        case "${COMP_WORDS[1]}" in
%s        esac
    fi
}
complete -F _jira_thing_completions jira-thing
`

const zshCompletionTemplate = `#compdef jira-thing
_jira_thing() {
    local -a commands
    commands=(%s)
    if (( CURRENT == 2 )); then
        _describe 'command' commands
    elif (( CURRENT == 3 )); then
        case $words[2] in
%s        esac
    fi
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
		fmt.Printf(bashCompletionTemplate, strings.Join(completionCommands, " "), subcommandCases(bashSubcommandCase))
	case "zsh":
		fmt.Printf(zshCompletionTemplate, strings.Join(quoteAll(completionCommands), " "), subcommandCases(zshSubcommandCase))
	default:
		fatal("unsupported shell: %s (supported: bash, zsh)", args[0])
	}
}

// subcommandCases renders one shell case arm per dispatching command.
func subcommandCases(arm func(pattern, words string) string) string {
	var sb strings.Builder
	for _, c := range completionSubcommands {
		sb.WriteString(arm(strings.Join(c.names, "|"), strings.Join(c.subs, " ")))
	}
	return sb.String()
}

// bashSubcommandCase is a bash case arm completing words.
func bashSubcommandCase(pattern, words string) string {
	return fmt.Sprintf("            %s) COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") ) ;;\n", pattern, words)
}

// zshSubcommandCase is a zsh case arm completing words.
func zshSubcommandCase(pattern, words string) string {
	return fmt.Sprintf("            %s) compadd %s ;;\n", pattern, words)
}

// quoteAll wraps each string in single quotes, for embedding in a zsh array literal.
func quoteAll(items []string) []string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + s + "'"
	}
	return quoted
}
