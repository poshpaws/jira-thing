package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(path, []byte(`{"toil_marker":"ECP_TOIL","toil_team":"ECP_SEC_TEAM"}`), 0o644)

	old := ConfigPath
	ConfigPath = func() string { return path }
	defer func() { ConfigPath = old }()

	cfg := Load()
	if cfg.ToilMarker != "ECP_TOIL" {
		t.Errorf("ToilMarker = %q, want ECP_TOIL", cfg.ToilMarker)
	}
	if cfg.ToilTeam != "ECP_SEC_TEAM" {
		t.Errorf("ToilTeam = %q, want ECP_SEC_TEAM", cfg.ToilTeam)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	old := ConfigPath
	ConfigPath = func() string { return "/nonexistent/path.json" }
	defer func() { ConfigPath = old }()

	cfg := Load()
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

	cfg := Load()
	if cfg.ToilMarker != "" || cfg.ToilTeam != "" {
		t.Errorf("expected empty config for invalid JSON, got %+v", cfg)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	old := ConfigPath
	ConfigPath = func() string { return "" }
	defer func() { ConfigPath = old }()

	cfg := Load()
	if cfg.ToilMarker != "" || cfg.ToilTeam != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}
