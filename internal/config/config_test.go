package config

import (
	"os"
	"path/filepath"
	"testing"

	"jira-thing/internal/profile"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`{"toil_marker":"TOIL_LABEL","toil_team":"SEC_TEAM"}`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ToilMarker != "TOIL_LABEL" {
		t.Errorf("ToilMarker = %q, want TOIL_LABEL", cfg.ToilMarker)
	}
	if cfg.ToilTeam != "SEC_TEAM" {
		t.Errorf("ToilTeam = %q, want SEC_TEAM", cfg.ToilTeam)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	old := ConfigPath
	ConfigPath = func() string { return "/nonexistent/path.json" }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.ToilMarker != "" || cfg.ToilTeam != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`not json`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	old := ConfigPath
	ConfigPath = func() string { return "" }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for empty path, got %v", err)
	}
	if cfg.ToilMarker != "" || cfg.ToilTeam != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_ConfluenceFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`{"confluence_space":"ENG","ticket_hanger":"Toil Tracker"}`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ConfluenceSpace != "ENG" {
		t.Errorf("ConfluenceSpace = %q, want ENG", cfg.ConfluenceSpace)
	}
	if cfg.TicketHanger != "Toil Tracker" {
		t.Errorf("TicketHanger = %q, want Toil Tracker", cfg.TicketHanger)
	}
}

func TestLoad_ProfileOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jira-thing.json")
	data := `{"project":"HOME","board":1,"editor":"vi","profiles":{"work":{"project":"OPS","board":7},"broken":{"board":"seven"}}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	old := ConfigPath
	ConfigPath = func() string { return path }
	t.Cleanup(func() { ConfigPath = old })

	t.Setenv(profile.EnvVar, "")
	if cfg, err := Load(); err != nil || cfg.Project != "HOME" || cfg.Board != 1 {
		t.Errorf("default: %+v %v", cfg, err)
	}
	t.Setenv(profile.EnvVar, "work")
	if cfg, err := Load(); err != nil || cfg.Project != "OPS" || cfg.Board != 7 || cfg.Editor != "vi" {
		t.Errorf("work: %+v %v", cfg, err)
	}
	t.Setenv(profile.EnvVar, "nosuch")
	if cfg, err := Load(); err != nil || cfg.Project != "HOME" {
		t.Errorf("profile without overrides: %+v %v", cfg, err)
	}
	t.Setenv(profile.EnvVar, "broken")
	if _, err := Load(); err == nil {
		t.Error("expected a parse error for a bad profile")
	}
}
