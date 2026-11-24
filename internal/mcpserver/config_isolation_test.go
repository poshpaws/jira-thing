package mcpserver

import "jira-thing/internal/config"

func init() {
	// Keep tests off the developer's real config; tests that need settings
	// point ConfigPath at their own file.
	config.ConfigPath = func() string { return "/nonexistent/jira-thing-test/jira-thing.json" }
}
