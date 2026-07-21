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
