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
// which neutralises ANSI and OSC escape sequences, plus bidi embedding,
// override and isolate controls and Unicode line/paragraph separators.
// Other format characters, such as the zero-width joiner in emoji, are kept.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unsafe(r) {
			return -1
		}
		return r
	}, s)
}

// Bidi controls and separators, written as numbers so this file itself
// contains no invisible characters.
const (
	bidiEmbedFirst   = 0x202A // LEFT-TO-RIGHT EMBEDDING
	bidiOverrideLast = 0x202E // RIGHT-TO-LEFT OVERRIDE
	bidiIsolateFirst = 0x2066 // LEFT-TO-RIGHT ISOLATE
	bidiIsolateLast  = 0x2069 // POP DIRECTIONAL ISOLATE
	lineSeparator    = 0x2028
	paraSeparator    = 0x2029
)

// unsafe reports whether r must not reach the terminal.
func unsafe(r rune) bool {
	switch {
	case r == '\t' || r == '\n':
		return false
	case unicode.IsControl(r):
		return true
	case r >= bidiEmbedFirst && r <= bidiOverrideLast, r >= bidiIsolateFirst && r <= bidiIsolateLast:
		return true
	default:
		return r == lineSeparator || r == paraSeparator
	}
}
