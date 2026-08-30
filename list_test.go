package main

import (
	"strings"
	"testing"
)

func TestBuildListJQL_Empty(t *testing.T) {
	got := buildListJQL(listFilters{})
	want := "ORDER BY created DESC"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildListJQL_AssigneeMe(t *testing.T) {
	got := buildListJQL(listFilters{assignee: "me"})
	if !strings.Contains(got, "assignee = currentUser()") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_AssigneeUnassigned(t *testing.T) {
	got := buildListJQL(listFilters{assignee: "x"})
	if !strings.Contains(got, "assignee is EMPTY") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_AssigneeNotUnassigned(t *testing.T) {
	got := buildListJQL(listFilters{assignee: "~x"})
	if !strings.Contains(got, "assignee is not EMPTY") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_AssigneeName(t *testing.T) {
	got := buildListJQL(listFilters{assignee: "Jon Doe"})
	if !strings.Contains(got, `assignee = "Jon Doe"`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_AssigneeNegatedName(t *testing.T) {
	got := buildListJQL(listFilters{assignee: "~Jon Doe"})
	if !strings.Contains(got, `assignee != "Jon Doe"`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_ReporterMe(t *testing.T) {
	got := buildListJQL(listFilters{reporter: "me"})
	if !strings.Contains(got, "reporter = currentUser()") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_PriorityAndStatus(t *testing.T) {
	got := buildListJQL(listFilters{priority: "High", status: "In Progress"})
	if !strings.Contains(got, `priority = "High"`) || !strings.Contains(got, `status = "In Progress"`) {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, " AND ") {
		t.Errorf("expected clauses AND-ed together, got %q", got)
	}
}

func TestBuildListJQL_NegatedStatus(t *testing.T) {
	got := buildListJQL(listFilters{status: "~Done"})
	if !strings.Contains(got, `status != "Done"`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_Project(t *testing.T) {
	got := buildListJQL(listFilters{project: "PROJ"})
	if !strings.Contains(got, `project = "PROJ"`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_Labels(t *testing.T) {
	got := buildListJQL(listFilters{labels: []string{"backend", "urgent"}})
	if !strings.Contains(got, `labels in ("backend", "urgent")`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_CreatedRelative(t *testing.T) {
	got := buildListJQL(listFilters{created: "-7d"})
	if !strings.Contains(got, "created >= -7d") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_CreatedKeyword(t *testing.T) {
	got := buildListJQL(listFilters{created: "week"})
	if !strings.Contains(got, "created >= startOfWeek()") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_CreatedAbsolute(t *testing.T) {
	got := buildListJQL(listFilters{created: "2026-01-01"})
	if !strings.Contains(got, `created >= "2026-01-01"`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_Updated(t *testing.T) {
	got := buildListJQL(listFilters{updated: "-30m"})
	if !strings.Contains(got, "updated >= -30m") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_Watching(t *testing.T) {
	got := buildListJQL(listFilters{watching: true})
	if !strings.Contains(got, "watcher = currentUser()") {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_RawJQLAppended(t *testing.T) {
	got := buildListJQL(listFilters{jql: `summary ~ "cli"`})
	if !strings.Contains(got, `(summary ~ "cli")`) {
		t.Errorf("got %q", got)
	}
}

func TestBuildListJQL_OrderByAndReverse(t *testing.T) {
	got := buildListJQL(listFilters{orderBy: "rank", reverse: true})
	want := "ORDER BY rank ASC"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildListJQL_AllFiltersCombined(t *testing.T) {
	got := buildListJQL(listFilters{
		assignee: "me",
		priority: "High",
		status:   "In Progress",
		labels:   []string{"backend"},
	})
	for _, want := range []string{
		"assignee = currentUser()",
		`priority = "High"`,
		`status = "In Progress"`,
		`labels in ("backend")`,
		"ORDER BY created DESC",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}
