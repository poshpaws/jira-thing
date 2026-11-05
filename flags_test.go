package main

import (
	"flag"
	"io"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// parseReordered runs args through reorderFlagsBeforeKey and a flag set with
// one bool flag (-json) and one value flag (-type), returning what was parsed.
func parseReordered(t *testing.T, args []string) (jsonOut bool, linkType string, positional []string) {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonFlag := fs.Bool("json", false, "")
	typeFlag := fs.String("type", "", "")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return *jsonFlag, *typeFlag, fs.Args()
}

func TestReorderFlagsBeforeKey_AnyPosition(t *testing.T) {
	cases := map[string][]string{
		"flags last":           {"A", "B", "-type", "T", "-json"},
		"bool first":           {"-json", "A", "B", "-type", "T"},
		"value first":          {"-type", "T", "-json", "A", "B"},
		"bool between keys":    {"A", "-json", "B", "-type", "T"},
		"value between keys":   {"A", "-type", "T", "B", "-json"},
		"double dash":          {"--json", "A", "--type", "T", "B"},
		"equals form":          {"-type=T", "A", "-json", "B"},
		"explicit bool equals": {"-json=true", "A", "B", "-type", "T"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			jsonOut, linkType, positional := parseReordered(t, args)
			if !jsonOut || linkType != "T" || !slices.Equal(positional, []string{"A", "B"}) {
				t.Errorf("json=%v type=%q positional=%v", jsonOut, linkType, positional)
			}
		})
	}
}

func TestReorderFlagsBeforeKey_ValueMayStartWithDash(t *testing.T) {
	_, linkType, positional := parseReordered(t, []string{"A", "-type", "-odd"})
	if linkType != "-odd" || !slices.Equal(positional, []string{"A"}) {
		t.Errorf("type=%q positional=%v", linkType, positional)
	}
}

func TestReorderFlagsBeforeKey_DoubleDashEndsFlags(t *testing.T) {
	jsonOut, _, positional := parseReordered(t, []string{"A", "--", "-json"})
	if jsonOut || !slices.Equal(positional, []string{"A", "-json"}) {
		t.Errorf("json=%v positional=%v", jsonOut, positional)
	}
}

func TestReorderFlagsBeforeKey_LoneDashIsPositional(t *testing.T) {
	_, _, positional := parseReordered(t, []string{"-json", "-"})
	if !slices.Equal(positional, []string{"-"}) {
		t.Errorf("positional=%v", positional)
	}
}

func TestRunLink_BoolFlagBeforeKeys(t *testing.T) {
	rec, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runLink([]string{"-dry-run", "PROJ-1", "PROJ-2", "-type", "Blocks"}) })
	if rec.linkBody != nil {
		t.Errorf("dry run must not POST, got %v", rec.linkBody)
	}
	if !strings.Contains(out, "Dry run") || !strings.Contains(out, "PROJ-2") {
		t.Errorf("expected dry-run payload naming both keys, got: %s", out)
	}
}

func TestRunTransition_BoolFlagBeforeKey(t *testing.T) {
	srv := httptest.NewTLSServer(transitionsHandler(t, func(map[string]any) {
		t.Error("dry run must not POST a transition")
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runTransition([]string{"-dry-run", "PROJ-1", "Done"}) })
	if !strings.Contains(out, "Dry run") || !strings.Contains(out, `"21"`) {
		t.Errorf("expected dry-run for transition 21, got: %s", out)
	}
}

func TestParseConfExportFlags_OutputFlagInEitherPosition(t *testing.T) {
	for _, args := range [][]string{{"123", "-o", "dir"}, {"-o", "dir", "123"}} {
		pageID, outDir := parseConfExportFlags(args)
		if pageID != "123" || outDir != "dir" {
			t.Errorf("%v: pageID=%q outDir=%q", args, pageID, outDir)
		}
	}
}
