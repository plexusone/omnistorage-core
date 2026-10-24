package redis

import (
	"path"
	"testing"
)

func TestEscapeGlob(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"session:user:", "session:user:"},
		{"user:*", `user:\*`},
		{"a?b", `a\?b`},
		{"[abc]", `\[abc\]`},
		{`back\slash`, `back\\slash`},
		{"ünï:*", `ünï:\*`},
	}
	for _, tt := range tests {
		if got := escapeGlob(tt.in); got != tt.want {
			t.Errorf("escapeGlob(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestEscapeGlobMatching checks the escaped prefix with path.Match, whose
// metacharacters and backslash escaping agree with Redis glob patterns for
// these inputs.
func TestEscapeGlobMatching(t *testing.T) {
	tests := []struct {
		prefix, key string
		want        bool
	}{
		{"user:", "user:123", true},
		{"user:*", "user:*1", true},
		{"user:*", "user:123", false},
		{"a?", "ab", false},
		{"a?", "a?b", true},
		{"[ab]", "a", false},
		{"[ab]", "[ab]x", true},
		{`x\`, `x\y`, true},
		{`x\`, "xy", false},
	}
	for _, tt := range tests {
		got, err := path.Match(escapeGlob(tt.prefix)+"*", tt.key)
		if err != nil {
			t.Fatalf("path.Match(%q): %v", tt.prefix, err)
		}
		if got != tt.want {
			t.Errorf("prefix %q matching %q = %v, want %v", tt.prefix, tt.key, got, tt.want)
		}
	}
}
