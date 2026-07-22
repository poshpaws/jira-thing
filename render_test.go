package main

import "testing"

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
	got := adfToMarkdown(doc)
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
	got := adfToMarkdown(doc)
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
	got := adfToMarkdown(doc)
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
	got := adfToMarkdown(doc)
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
	got := adfToMarkdown(doc)
	want := "- one\n- two\n  - inner\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildDescribeMarkdown_FullIssue(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"summary":  "Fix the thing",
			"status":   map[string]any{"name": "In Progress"},
			"priority": map[string]any{"name": "High"},
			"assignee": map[string]any{"displayName": "Alice"},
			"reporter": map[string]any{"displayName": "Bob"},
			"created":  "2026-01-01T10:00:00.000+0000",
			"updated":  "2026-01-02T10:00:00.000+0000",
			"description": map[string]any{
				"type": "doc",
				"content": []any{
					map[string]any{
						"type":    "paragraph",
						"content": []any{map[string]any{"type": "text", "text": "does the thing"}},
					},
				},
			},
		},
	}
	got := buildDescribeMarkdown(issue)
	want := "# PROJ-1: Fix the thing\n\n" +
		"- **Status:** In Progress\n" +
		"- **Priority:** High\n" +
		"- **Assignee:** Alice\n" +
		"- **Reporter:** Bob\n" +
		"- **Created:** 2026-01-01\n" +
		"- **Updated:** 2026-01-02\n\n" +
		"## Description\n\n" +
		"does the thing\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildDescribeMarkdown_MissingFields(t *testing.T) {
	issue := map[string]any{"key": "PROJ-2"}
	got := buildDescribeMarkdown(issue)
	want := "# PROJ-2: \n\n" +
		"- **Status:** \n" +
		"- **Priority:** \n" +
		"- **Assignee:** Unassigned\n" +
		"- **Reporter:** Unknown\n" +
		"- **Created:** \n" +
		"- **Updated:** \n\n" +
		"## Description\n\n" +
		"_No description._\n"
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
	got := adfToMarkdown(doc)
	want := "1. first\n   1. sub\n2. second\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
