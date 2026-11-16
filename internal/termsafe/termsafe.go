// Package termsafe makes text written by other Jira users safe to print to a
// terminal. Ticket summaries, descriptions and comments are untrusted input:
// embedded escape sequences could drive the reader's terminal, and bidi
// overrides could make displayed text read differently from its bytes.
package termsafe

import (
	"strings"
	"unicode"
)

// Clean removes C0 and C1 control characters (keeping tab and newline),
// which neutralises ANSI and OSC escape sequences, plus every Unicode
// Bidi_Control character (marks, embeddings, overrides, isolates) and
// line/paragraph separators.
// Other format characters, such as the zero-width joiner in emoji, are kept.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unsafe(r) {
			return -1
		}
		return r
	}, s)
}

// Unicode line and paragraph separators, written as numbers so this file
// itself contains no invisible characters.
const (
	lineSeparator = 0x2028
	paraSeparator = 0x2029
)

// unsafe reports whether r must not reach the terminal.
func unsafe(r rune) bool {
	switch {
	case r == '\t' || r == '\n':
		return false
	case unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r):
		return true
	default:
		return r == lineSeparator || r == paraSeparator
	}
}

// Lines applies Line to each value.
func Lines(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = Line(v)
	}
	return out
}

// Line is Clean for a value shown on one line, such as a name in a list:
// newlines and tabs become spaces, so the value can't forge extra lines.
func Line(s string) string {
	return strings.NewReplacer("\n", " ", "\t", " ").Replace(Clean(s))
}
