package template

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNamedPath_RejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"", "../x", "a/b", "a.json", ".hidden", "sp ace", `a\b`} {
		if _, err := NamedPath("/tpl", name); !errors.Is(err, ErrBadTemplateName) {
			t.Errorf("%q: err=%v", name, err)
		}
	}
	if got, err := NamedPath("/tpl", "toil_v2-x"); err != nil || got != "/tpl/toil_v2-x.json" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestNamedDir_DefaultsUnderConfig(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := NamedDir(""); got != filepath.Join(home, ".config", "jira-thing", "templates") {
		t.Errorf("got %q", got)
	}
	if got := NamedDir("~/t"); got != filepath.Join(home, "t") {
		t.Errorf("got %q", got)
	}
}

func TestListNamed(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"b.json", "a.json", "notes.txt", "bad name.json"} {
		os.WriteFile(filepath.Join(dir, f), []byte(`{}`), 0o600)
	}
	os.Mkdir(filepath.Join(dir, "sub.json"), 0o700)
	got, err := ListNamed(dir)
	if err != nil || !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("got %v err=%v", got, err)
	}
	if got, err := ListNamed(filepath.Join(dir, "missing")); err != nil || len(got) != 0 {
		t.Errorf("missing dir: got %v err=%v", got, err)
	}
}

func TestLoadNamed_StripsExcludedFields(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.json"), []byte(`{"summary":"S","customfield_1":"v"}`), 0o600)
	tmpl, err := LoadNamed(dir, "x")
	if err != nil || tmpl["summary"] != "S" || tmpl["customfield_1"] != nil {
		t.Errorf("tmpl=%v err=%v", tmpl, err)
	}
	if _, err := LoadNamed(dir, "absent"); !os.IsNotExist(err) {
		t.Errorf("absent: err=%v", err)
	}
}
