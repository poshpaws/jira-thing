package main

import (
	"strings"
	"testing"
)

func TestConfluenceStorageToMarkdown_Headings(t *testing.T) {
	got := confluenceStorageToMarkdown("<h1>Title</h1><h3>Sub</h3>", "").Markdown
	want := "# Title\n\n### Sub\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_ParagraphAndInlineFormatting(t *testing.T) {
	got := confluenceStorageToMarkdown(`<p>Hello <strong>bold</strong> and <em>italic</em> and <code>code</code>.</p>`, "").Markdown
	want := "Hello **bold** and *italic* and `code`.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_EmptyParagraphDropped(t *testing.T) {
	got := confluenceStorageToMarkdown(`<p>first</p><p> </p><p>second</p>`, "").Markdown
	if strings.Count(got, "\n\n\n") != 0 {
		t.Errorf("expected blank-line runs collapsed, got %q", got)
	}
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Errorf("expected both paragraphs present, got %q", got)
	}
}

func TestConfluenceStorageToMarkdown_Link(t *testing.T) {
	got := confluenceStorageToMarkdown(`<p><a href="https://example.com">example</a></p>`, "").Markdown
	want := "[example](https://example.com)\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_UnorderedList(t *testing.T) {
	got := confluenceStorageToMarkdown(`<ul><li>one</li><li>two</li></ul>`, "").Markdown
	want := "- one\n- two\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_OrderedListWithParagraphWrappedItems(t *testing.T) {
	// Confluence commonly wraps <li> text in a <p>.
	got := confluenceStorageToMarkdown(`<ol><li><p>first</p></li><li><p>second</p></li></ol>`, "").Markdown
	want := "1. first\n2. second\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_NestedList(t *testing.T) {
	got := confluenceStorageToMarkdown(`<ul><li>parent<ul><li>child</li></ul></li></ul>`, "").Markdown
	if !strings.Contains(got, "- parent") {
		t.Errorf("expected top-level item, got %q", got)
	}
	if !strings.Contains(got, "  - child") {
		t.Errorf("expected indented nested item, got %q", got)
	}
}

func TestConfluenceStorageToMarkdown_Table(t *testing.T) {
	storage := `<table><tbody>` +
		`<tr><th>Name</th><th>Age</th></tr>` +
		`<tr><td>Alice</td><td>30</td></tr>` +
		`</tbody></table>`
	got := confluenceStorageToMarkdown(storage, "").Markdown
	if !strings.Contains(got, "| Name | Age |") {
		t.Errorf("missing header row: %q", got)
	}
	if !strings.Contains(got, "| --- | --- |") {
		t.Errorf("missing separator row: %q", got)
	}
	if !strings.Contains(got, "| Alice | 30 |") {
		t.Errorf("missing data row: %q", got)
	}
}

func TestConfluenceStorageToMarkdown_Blockquote(t *testing.T) {
	got := confluenceStorageToMarkdown(`<blockquote><p>quoted text</p></blockquote>`, "").Markdown
	if !strings.Contains(got, "> quoted text") {
		t.Errorf("expected blockquote prefix, got %q", got)
	}
}

func TestConfluenceStorageToMarkdown_CodeMacro(t *testing.T) {
	storage := `<ac:structured-macro ac:name="code" ac:schema-version="1">` +
		`<ac:parameter ac:name="language">go</ac:parameter>` +
		`<ac:plain-text-body><![CDATA[fmt.Println("hi")]]></ac:plain-text-body>` +
		`</ac:structured-macro>`
	got := confluenceStorageToMarkdown(storage, "").Markdown
	want := "```go\nfmt.Println(\"hi\")\n```\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_PanelMacro(t *testing.T) {
	storage := `<ac:structured-macro ac:name="warning">` +
		`<ac:rich-text-body><p>be careful</p></ac:rich-text-body>` +
		`</ac:structured-macro>`
	got := confluenceStorageToMarkdown(storage, "").Markdown
	if !strings.Contains(got, "> ⚠️ **Warning:** be careful") {
		t.Errorf("expected labelled blockquote, got %q", got)
	}
}

func TestConfluenceStorageToMarkdown_ImageAttachment(t *testing.T) {
	storage := `<ac:image ac:alt="A diagram"><ri:attachment ri:filename="diagram.png"/></ac:image>`
	result := confluenceStorageToMarkdown(storage, "attachments")
	want := "![A diagram](attachments/diagram.png)\n"
	if result.Markdown != want {
		t.Errorf("got %q, want %q", result.Markdown, want)
	}
	if len(result.Attachments) != 1 || result.Attachments[0] != "diagram.png" {
		t.Errorf("Attachments = %v, want [diagram.png]", result.Attachments)
	}
}

func TestConfluenceStorageToMarkdown_ExternalImage(t *testing.T) {
	storage := `<ac:image ac:alt="Logo"><ri:url ri:value="https://example.com/logo.png"/></ac:image>`
	result := confluenceStorageToMarkdown(storage, "attachments")
	want := "![Logo](https://example.com/logo.png)\n"
	if result.Markdown != want {
		t.Errorf("got %q, want %q", result.Markdown, want)
	}
	if len(result.Attachments) != 0 {
		t.Errorf("external image should not be registered as an attachment, got %v", result.Attachments)
	}
}

func TestConfluenceStorageToMarkdown_AttachmentLink(t *testing.T) {
	storage := `<ac:link><ri:attachment ri:filename="report.pdf"/>` +
		`<ac:plain-text-link-body><![CDATA[Quarterly Report]]></ac:plain-text-link-body></ac:link>`
	result := confluenceStorageToMarkdown(storage, "attachments")
	want := "[Quarterly Report](attachments/report.pdf)\n"
	if result.Markdown != want {
		t.Errorf("got %q, want %q", result.Markdown, want)
	}
	if len(result.Attachments) != 1 || result.Attachments[0] != "report.pdf" {
		t.Errorf("Attachments = %v, want [report.pdf]", result.Attachments)
	}
}

func TestConfluenceStorageToMarkdown_ViewFileMacroEmbedsDocument(t *testing.T) {
	// The view-file macro (Office document preview embed) has no dedicated
	// handler — it should fall back to the generic ri:attachment resolution.
	storage := `<ac:structured-macro ac:name="view-file">` +
		`<ac:parameter ac:name="name"><ri:attachment ri:filename="spec.docx"/></ac:parameter>` +
		`</ac:structured-macro>`
	result := confluenceStorageToMarkdown(storage, "attachments")
	if len(result.Attachments) != 1 || result.Attachments[0] != "spec.docx" {
		t.Errorf("Attachments = %v, want [spec.docx]", result.Attachments)
	}
	if !strings.Contains(result.Markdown, "attachments/spec.docx") {
		t.Errorf("expected markdown to reference attachments/spec.docx, got %q", result.Markdown)
	}
}

func TestConfluenceStorageToMarkdown_DrawioMacro(t *testing.T) {
	storage := `<ac:structured-macro ac:name="drawio">` +
		`<ac:parameter ac:name="diagramName">architecture</ac:parameter>` +
		`</ac:structured-macro>`
	result := confluenceStorageToMarkdown(storage, "attachments")
	if len(result.Attachments) != 1 || result.Attachments[0] != "architecture.drawio" {
		t.Errorf("Attachments = %v, want [architecture.drawio]", result.Attachments)
	}
	if !strings.Contains(result.Markdown, "attachments/architecture.drawio") {
		t.Errorf("expected drawio reference, got %q", result.Markdown)
	}
}

func TestConfluenceStorageToMarkdown_DuplicateAttachmentReferencesDeduplicated(t *testing.T) {
	storage := `<ac:image><ri:attachment ri:filename="x.png"/></ac:image>` +
		`<ac:image><ri:attachment ri:filename="x.png"/></ac:image>`
	result := confluenceStorageToMarkdown(storage, "attachments")
	if len(result.Attachments) != 1 {
		t.Errorf("expected deduplicated attachments, got %v", result.Attachments)
	}
}

func TestConfluenceStorageToMarkdown_InternalPageLink(t *testing.T) {
	storage := `<ac:link><ri:page ri:content-title="Other Page"/>` +
		`<ac:plain-text-link-body><![CDATA[See Other Page]]></ac:plain-text-link-body></ac:link>`
	got := confluenceStorageToMarkdown(storage, "").Markdown
	want := "**See Other Page**\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfluenceStorageToMarkdown_HorizontalRuleAndBreak(t *testing.T) {
	got := confluenceStorageToMarkdown(`<p>a<br/>b</p><hr/><p>c</p>`, "").Markdown
	if !strings.Contains(got, "a  \nb") {
		t.Errorf("expected hard line break, got %q", got)
	}
	if !strings.Contains(got, "---") {
		t.Errorf("expected horizontal rule, got %q", got)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Toil Tracker", "toil-tracker"},
		{"  Weird__Title!! ", "weird-title"},
		{"", "confluence-page"},
		{"already-slug", "already-slug"},
	}
	for _, tt := range tests {
		if got := slugify(tt.in); got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
