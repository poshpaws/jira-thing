package adf

import (
	"strings"
	"testing"
)

func TestAdfMention_Sanitised(t *testing.T) {
	got := adfMention(map[string]any{"attrs": map[string]any{"text": "@Ada\x1b[2J"}})
	if got != "@Ada[2J" || strings.Contains(got, "\x1b") {
		t.Errorf("mention = %q", got)
	}
}
