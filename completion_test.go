package main

import (
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
