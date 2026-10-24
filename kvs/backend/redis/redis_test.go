package redis

import (
	"errors"
	"path"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
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

func TestNewRefusesInsecureURL(t *testing.T) {
	tests := []string{
		"redis://cache.example.com:6379",
		"redis://:secret@cache.example.com:6379",
		"redis://:secret@10.0.0.5:6379/0",
		"rediss://cache.example.com:6380",
		"rediss://:secret@cache.example.com:6380?skip_verify=true",
		"redis://[2001:db8::1]:6379",
	}
	for _, u := range tests {
		s, err := New(Config{URL: u, ConnectTimeout: 50 * time.Millisecond})
		if err == nil {
			if cerr := s.Close(); cerr != nil {
				t.Errorf("Close: %v", cerr)
			}
			t.Fatalf("New(%q) succeeded, want ErrInsecureConnection", u)
		}
		if !errors.Is(err, ErrInsecureConnection) {
			t.Errorf("New(%q) err = %v, want ErrInsecureConnection", u, err)
		}
	}
}

func TestCheckSecure(t *testing.T) {
	tests := []struct {
		url    string
		secure bool
	}{
		{"redis://localhost:6379", true},
		{"redis://127.0.0.1:6379", true},
		{"redis://127.0.0.2:6379", true},
		{"redis://[::1]:6379", true},
		{"unix:///tmp/redis.sock", true},
		{"rediss://:secret@cache.example.com:6380", true},
		{"rediss://user:secret@cache.example.com:6380/1", true},
		{"redis://cache.example.com:6379", false},
		{"redis://:secret@cache.example.com:6379", false},
		{"rediss://cache.example.com:6380", false},
		{"rediss://:secret@cache.example.com:6380?skip_verify=true", false},
		{"redis://localhost.example.com:6379", false},
	}
	for _, tt := range tests {
		opt, err := redis.ParseURL(tt.url)
		if err != nil {
			t.Fatalf("ParseURL(%q): %v", tt.url, err)
		}
		err = checkSecure(opt)
		if tt.secure && err != nil {
			t.Errorf("checkSecure(%q) = %v, want nil", tt.url, err)
		}
		if !tt.secure && !errors.Is(err, ErrInsecureConnection) {
			t.Errorf("checkSecure(%q) = %v, want ErrInsecureConnection", tt.url, err)
		}
	}
}
