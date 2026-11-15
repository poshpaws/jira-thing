package termsafe

import "testing"

func TestClean(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain text untouched", "Fix login – café 🎉", "Fix login – café 🎉"},
		{"tab and newline kept", "a\tb\nc", "a\tb\nc"},
		{"ANSI CSI neutralised", "x\x1b[2Jy", "x[2Jy"},
		{"OSC title and BEL removed", "\x1b]0;pwned\x07ok", "]0;pwnedok"},
		{"C1 CSI removed", "a\u009b31mb", "a31mb"},
		{"bidi override removed", "user‮gnp.exe", "usergnp.exe"},
		{"bidi isolates removed", "a⁦b⁩c", "abc"},
		{"line separators removed", "a b c", "abc"},
		{"emoji ZWJ kept", "👩‍💻", "👩‍💻"},
	}
	for _, tt := range tests {
		if got := Clean(tt.in); got != tt.want {
			t.Errorf("%s: Clean(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}
