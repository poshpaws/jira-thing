package main

import (
	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
)

// menuListBoards is the menu's "Boards" action.
func menuListBoards(conn api.JiraConnection) {
	boards, err := ops.ListBoards(conn, promptLine("Project key (blank for all): "))
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	printBoards(boards)
}

// menuListProjects is the menu's "Projects" action.
func menuListProjects(conn api.JiraConnection) {
	projects, err := ops.ListProjects(conn)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	printProjects(projects)
}

// menuListVersions is the menu's "Releases" action: a project's unarchived
// releases, the configured project when the answer is blank.
func menuListVersions(conn api.JiraConnection) {
	cfg, err := config.Load()
	if err != nil {
		printMenuErr("loading config: %v", err)
		return
	}
	project := ops.ProjectOrConfig(cfg, promptLine("Project key (blank for config default): "))
	versions, err := ops.ListReleases(conn, ops.ReleaseFilter{Project: project})
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	printReleases(versions)
}
