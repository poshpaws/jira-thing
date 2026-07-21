package main

import (
	"encoding/json"
	"testing"
)

// jsonEq marshals both values and compares the JSON — gives readable diffs
// and ignores int/float distinctions in attrs.
func jsonEq(t *testing.T, want, got any) {
	t.Helper()
	w, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	g, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if string(w) != string(g) {
		t.Errorf("ADF mismatch\nwant:\n%s\ngot:\n%s", w, g)
	}
}

// Expected-structure helpers. Deliberately independent of the converter's
// own builders so tests describe the target shape, not the implementation.

func eDoc(blocks ...any) map[string]any {
	if blocks == nil {
		blocks = []any{}
	}
	return map[string]any{"type": "doc", "version": 1, "content": blocks}
}

func eNode(typ string, children ...any) map[string]any {
	return map[string]any{"type": typ, "content": children}
}

func eText(s string, marks ...any) map[string]any {
	n := map[string]any{"type": "text", "text": s}
	if len(marks) > 0 {
		n["marks"] = marks
	}
	return n
}

func eMark(typ string) map[string]any { return map[string]any{"type": typ} }

func TestMarkdownToADF_Paragraphs(t *testing.T) {
	got := markdownToADF("first para\n\nsecond para")
	want := eDoc(
		eNode("paragraph", eText("first para")),
		eNode("paragraph", eText("second para")),
	)
	jsonEq(t, want, got)
}

func TestMarkdownToADF_HeadingLevels(t *testing.T) {
	got := markdownToADF("# One\n\n## Two\n\n### Three")
	h := func(level int, txt string) map[string]any {
		return map[string]any{
			"type":    "heading",
			"attrs":   map[string]any{"level": level},
			"content": []any{eText(txt)},
		}
	}
	jsonEq(t, eDoc(h(1, "One"), h(2, "Two"), h(3, "Three")), got)
}

func TestMarkdownToADF_InlineMarks(t *testing.T) {
	got := markdownToADF("**bold** *italic* `code` ~~gone~~")
	want := eDoc(eNode("paragraph",
		eText("bold", eMark("strong")),
		eText(" "),
		eText("italic", eMark("em")),
		eText(" "),
		eText("code", eMark("code")),
		eText(" "),
		eText("gone", eMark("strike")),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_Link(t *testing.T) {
	got := markdownToADF("a [site](https://example.com) here")
	linkMark := map[string]any{
		"type":  "link",
		"attrs": map[string]any{"href": "https://example.com"},
	}
	want := eDoc(eNode("paragraph",
		eText("a "),
		eText("site", linkMark),
		eText(" here"),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_BulletListNested(t *testing.T) {
	got := markdownToADF("- one\n- two\n  - inner")
	want := eDoc(eNode("bulletList",
		eNode("listItem", eNode("paragraph", eText("one"))),
		eNode("listItem",
			eNode("paragraph", eText("two")),
			eNode("bulletList",
				eNode("listItem", eNode("paragraph", eText("inner"))),
			),
		),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_OrderedList(t *testing.T) {
	got := markdownToADF("1. first\n2. second")
	want := eDoc(map[string]any{
		"type":  "orderedList",
		"attrs": map[string]any{"order": 1},
		"content": []any{
			eNode("listItem", eNode("paragraph", eText("first"))),
			eNode("listItem", eNode("paragraph", eText("second"))),
		},
	})
	jsonEq(t, want, got)
}

func TestMarkdownToADF_FencedCodeBlock(t *testing.T) {
	got := markdownToADF("```go\nfunc main() {}\n```")
	want := eDoc(map[string]any{
		"type":    "codeBlock",
		"attrs":   map[string]any{"language": "go"},
		"content": []any{eText("func main() {}\n")},
	})
	jsonEq(t, want, got)
}

func TestMarkdownToADF_CodeBlockNoLanguage(t *testing.T) {
	got := markdownToADF("```\nplain\n```")
	want := eDoc(eNode("codeBlock", eText("plain\n")))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_Blockquote(t *testing.T) {
	got := markdownToADF("> quoted text")
	want := eDoc(eNode("blockquote",
		eNode("paragraph", eText("quoted text")),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_ThematicBreak(t *testing.T) {
	got := markdownToADF("above\n\n---\n\nbelow")
	want := eDoc(
		eNode("paragraph", eText("above")),
		map[string]any{"type": "rule"},
		eNode("paragraph", eText("below")),
	)
	jsonEq(t, want, got)
}

func TestMarkdownToADF_Table(t *testing.T) {
	got := markdownToADF("| Col A | Col B |\n|-------|-------|\n| 1 | 2 |")
	want := eDoc(eNode("table",
		eNode("tableRow",
			eNode("tableHeader", eNode("paragraph", eText("Col A"))),
			eNode("tableHeader", eNode("paragraph", eText("Col B"))),
		),
		eNode("tableRow",
			eNode("tableCell", eNode("paragraph", eText("1"))),
			eNode("tableCell", eNode("paragraph", eText("2"))),
		),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_HardBreak(t *testing.T) {
	got := markdownToADF("line one  \nline two")
	want := eDoc(eNode("paragraph",
		eText("line one"),
		map[string]any{"type": "hardBreak"},
		eText("line two"),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_SoftBreakBecomesSpace(t *testing.T) {
	got := markdownToADF("line one\nline two")
	want := eDoc(eNode("paragraph",
		eText("line one"),
		eText(" "),
		eText("line two"),
	))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_PlainTextNoMarkdown(t *testing.T) {
	got := markdownToADF("just a plain comment")
	want := eDoc(eNode("paragraph", eText("just a plain comment")))
	jsonEq(t, want, got)
}

func TestMarkdownToADF_EmptyInput(t *testing.T) {
	jsonEq(t, eDoc(), markdownToADF(""))
}
