package adf

import (
	"strings"
	"testing"
)

func TestAdfToMarkdown_LinkMark(t *testing.T) {
	doc := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": "a "},
					map[string]any{
						"type": "text",
						"text": "site",
						"marks": []any{map[string]any{
							"type":  "link",
							"attrs": map[string]any{"href": "https://example.com"},
						}},
					},
				},
			},
		},
	}
	got := ToMarkdown(doc)
	want := "a [site](https://example.com)\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToMarkdown_Rule(t *testing.T) {
	doc := map[string]any{
		"type":    "doc",
		"content": []any{map[string]any{"type": "rule"}},
	}
	got := ToMarkdown(doc)
	want := "---\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToMarkdown_Table(t *testing.T) {
	cell := func(typ, txt string) map[string]any {
		return map[string]any{
			"type": typ,
			"content": []any{map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": txt}},
			}},
		}
	}
	row := func(cells ...any) map[string]any {
		return map[string]any{"type": "tableRow", "content": cells}
	}
	doc := map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type": "table",
			"content": []any{
				row(cell("tableHeader", "Col A"), cell("tableHeader", "Col B")),
				row(cell("tableCell", "1"), cell("tableCell", "2")),
			},
		}},
	}
	got := ToMarkdown(doc)
	want := "| Col A | Col B |\n| --- | --- |\n| 1 | 2 |\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToMarkdown_BlockquoteSeparatedFromNextBlock(t *testing.T) {
	para := func(txt string) map[string]any {
		return map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]any{"type": "text", "text": txt}},
		}
	}
	doc := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{"type": "blockquote", "content": []any{para("quoted")}},
			para("after"),
		},
	}
	got := ToMarkdown(doc)
	want := "> quoted\n\nafter\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToMarkdown_NestedBulletListIndented(t *testing.T) {
	para := func(txt string) map[string]any {
		return map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]any{"type": "text", "text": txt}},
		}
	}
	item := func(children ...any) map[string]any {
		return map[string]any{"type": "listItem", "content": children}
	}
	doc := map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type": "bulletList",
			"content": []any{
				item(para("one")),
				item(para("two"), map[string]any{
					"type":    "bulletList",
					"content": []any{item(para("inner"))},
				}),
			},
		}},
	}
	got := ToMarkdown(doc)
	want := "- one\n- two\n  - inner\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToMarkdown_NestedOrderedListIndented(t *testing.T) {
	para := func(txt string) map[string]any {
		return map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]any{"type": "text", "text": txt}},
		}
	}
	item := func(children ...any) map[string]any {
		return map[string]any{"type": "listItem", "content": children}
	}
	doc := map[string]any{
		"type": "doc",
		"content": []any{map[string]any{
			"type": "orderedList",
			"content": []any{
				item(para("first"), map[string]any{
					"type":    "orderedList",
					"content": []any{item(para("sub"))},
				}),
				item(para("second")),
			},
		}},
	}
	got := ToMarkdown(doc)
	want := "1. first\n   1. sub\n2. second\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToMarkdown_StripsTerminalControlCharacters(t *testing.T) {
	doc := map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": "safe\x1b[2J\x1b]0;pwned\x07 text\u009b31m\tend",
				"marks": []any{map[string]any{"type": "link", "attrs": map[string]any{"href": "https://x\x1b[8m.io"}}}},
		}},
	}}
	got := ToMarkdown(doc)
	for _, bad := range []string{"\x1b", "\x07", "\u009b"} {
		if strings.Contains(got, bad) {
			t.Errorf("control character %q survived: %q", bad, got)
		}
	}
	if !strings.Contains(got, "\tend") || !strings.Contains(got, "https://x") {
		t.Errorf("printable text and tabs must survive: %q", got)
	}
}
