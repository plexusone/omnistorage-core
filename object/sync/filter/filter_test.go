package filter

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFilterInclude(t *testing.T) {
	f := New(
		Include("*.json"),
		Include("*.txt"),
	)

	tests := []struct {
		path string
		want bool
	}{
		{"file.json", true},
		{"file.txt", true},
		{"file.xml", false},
		{"dir/file.json", true},
		{"dir/file.xml", false},
	}

	for _, tc := range tests {
		got := f.MatchPath(tc.path)
		if got != tc.want {
			t.Errorf("MatchPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterExclude(t *testing.T) {
	f := New(
		Exclude("*.tmp"),
		Exclude("*.bak"),
	)

	tests := []struct {
		path string
		want bool
	}{
		{"file.json", true},
		{"file.tmp", false},
		{"file.bak", false},
		{"dir/file.tmp", false},
	}

	for _, tc := range tests {
		got := f.MatchPath(tc.path)
		if got != tc.want {
			t.Errorf("MatchPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterIncludeExclude(t *testing.T) {
	// Include JSON but exclude backups
	f := New(
		Include("*.json"),
		Exclude("*.bak.json"),
	)

	tests := []struct {
		path string
		want bool
	}{
		{"file.json", true},
		{"file.bak.json", false},
		{"file.txt", false}, // Not included
	}

	for _, tc := range tests {
		got := f.MatchPath(tc.path)
		if got != tc.want {
			t.Errorf("MatchPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterMinSize(t *testing.T) {
	f := New(MinSize(100))

	tests := []struct {
		size int64
		want bool
	}{
		{50, false},
		{100, true},
		{150, true},
	}

	for _, tc := range tests {
		got := f.Match(FileInfo{Path: "file.txt", Size: tc.size})
		if got != tc.want {
			t.Errorf("Match(size=%d) = %v, want %v", tc.size, got, tc.want)
		}
	}
}

func TestFilterMaxSize(t *testing.T) {
	f := New(MaxSize(1 * MB))

	tests := []struct {
		size int64
		want bool
	}{
		{500 * KB, true},
		{1 * MB, true},
		{2 * MB, false},
	}

	for _, tc := range tests {
		got := f.Match(FileInfo{Path: "file.txt", Size: tc.size})
		if got != tc.want {
			t.Errorf("Match(size=%d) = %v, want %v", tc.size, got, tc.want)
		}
	}
}

func TestFilterMinAge(t *testing.T) {
	f := New(MinAge(24 * time.Hour))

	now := time.Now()
	tests := []struct {
		modTime time.Time
		want    bool
	}{
		{now.Add(-12 * time.Hour), false}, // Too new
		{now.Add(-24 * time.Hour), true},  // Exactly 24 hours
		{now.Add(-48 * time.Hour), true},  // Older
	}

	for _, tc := range tests {
		got := f.Match(FileInfo{Path: "file.txt", ModTime: tc.modTime})
		if got != tc.want {
			t.Errorf("Match(age=%v) = %v, want %v", now.Sub(tc.modTime), got, tc.want)
		}
	}
}

func TestFilterMaxAge(t *testing.T) {
	f := New(MaxAge(7 * 24 * time.Hour))

	now := time.Now()
	tests := []struct {
		modTime time.Time
		want    bool
	}{
		{now.Add(-1 * 24 * time.Hour), true},   // 1 day old
		{now.Add(-6 * 24 * time.Hour), true},   // 6 days old (within limit)
		{now.Add(-14 * 24 * time.Hour), false}, // Too old
	}

	for _, tc := range tests {
		got := f.Match(FileInfo{Path: "file.txt", ModTime: tc.modTime})
		if got != tc.want {
			t.Errorf("Match(age=%v) = %v, want %v", now.Sub(tc.modTime), got, tc.want)
		}
	}
}

func TestFilterCombined(t *testing.T) {
	f := New(
		Include("*.json"),
		Exclude("*.tmp"),
		MinSize(10),
		MaxSize(1*MB),
	)

	now := time.Now()
	tests := []struct {
		name string
		fi   FileInfo
		want bool
	}{
		{"json ok", FileInfo{Path: "file.json", Size: 100}, true},
		{"json too small", FileInfo{Path: "file.json", Size: 5}, false},
		{"json too large", FileInfo{Path: "file.json", Size: 2 * MB}, false},
		{"tmp excluded", FileInfo{Path: "file.tmp", Size: 100}, false},
		{"txt not included", FileInfo{Path: "file.txt", Size: 100}, false},
	}

	for _, tc := range tests {
		tc.fi.ModTime = now
		got := f.Match(tc.fi)
		if got != tc.want {
			t.Errorf("%s: Match() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFilterEmpty(t *testing.T) {
	f := New()

	if !f.Match(FileInfo{Path: "anything.txt"}) {
		t.Error("Empty filter should match everything")
	}
	if !f.IsEmpty() {
		t.Error("Empty filter should return IsEmpty() = true")
	}
}

func TestFilterNil(t *testing.T) {
	var f *Filter

	if !f.Match(FileInfo{Path: "anything.txt"}) {
		t.Error("Nil filter should match everything")
	}
	if !f.IsEmpty() {
		t.Error("Nil filter should return IsEmpty() = true")
	}
}

func TestFilterFromFile(t *testing.T) {
	// Create a temp filter file
	tmpDir := t.TempDir()
	filterPath := filepath.Join(tmpDir, "filters.txt")

	content := `# Include JSON
+ *.json
# Exclude temp
- *.tmp
- *.bak
`
	if err := os.WriteFile(filterPath, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write filter file: %v", err)
	}

	opt, err := FromFile(filterPath)
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	f := New(opt)

	tests := []struct {
		path string
		want bool
	}{
		{"file.json", true},
		{"file.tmp", false},
		{"file.bak", false},
		{"file.txt", false}, // Not included
	}

	for _, tc := range tests {
		got := f.MatchPath(tc.path)
		if got != tc.want {
			t.Errorf("MatchPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterFromFileNotFound(t *testing.T) {
	_, err := FromFile("/nonexistent/filter.txt")
	if err == nil {
		t.Error("FromFile should fail for nonexistent file")
	}
}

func TestSizeConstants(t *testing.T) {
	if KB != 1024 {
		t.Errorf("KB = %d, want 1024", KB)
	}
	if MB != 1024*1024 {
		t.Errorf("MB = %d, want %d", MB, 1024*1024)
	}
	if GB != 1024*1024*1024 {
		t.Errorf("GB = %d, want %d", GB, 1024*1024*1024)
	}
	if TB != 1024*1024*1024*1024 {
		t.Errorf("TB = %d, want %d", TB, 1024*1024*1024*1024)
	}
}

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.json", "file.json", true},
		{"*.json", "dir/file.json", true},
		{"data/*", "data/file.txt", true},
		{"*.txt", "file.json", false},
		{"test_*", "test_file.go", true},
		{"[a-z]*.go", "abc.go", true},
	}

	for _, tc := range tests {
		got := matchPattern(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
