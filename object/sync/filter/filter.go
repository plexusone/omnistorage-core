// Package filter provides file filtering for sync operations.
//
// Filters are used to include or exclude files based on patterns,
// size, and age. They are inspired by rclone's filtering system.
//
// Basic usage:
//
//	f := filter.New(
//	    filter.Include("*.json"),
//	    filter.Exclude("*.tmp"),
//	    filter.MaxSize(100 * 1024 * 1024), // 100 MB
//	)
//
//	if f.Match(fileInfo) {
//	    // File passes filter
//	}
package filter

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Filter determines whether files should be included in sync operations.
type Filter struct {
	rules []rule
}

type ruleType int

const (
	ruleInclude ruleType = iota
	ruleExclude
	ruleMinSize
	ruleMaxSize
	ruleMinAge
	ruleMaxAge
)

type rule struct {
	ruleType ruleType
	pattern  string        // for include/exclude
	size     int64         // for min/max size
	duration time.Duration // for min/max age
}

// FileInfo contains the information needed for filtering.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// Option configures a Filter.
type Option func(*Filter)

// New creates a new Filter with the given options.
func New(opts ...Option) *Filter {
	f := &Filter{}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Include adds an include pattern.
// Files matching any include pattern are included (unless excluded).
// Patterns use filepath.Match syntax (*, ?, [...]).
func Include(pattern string) Option {
	return func(f *Filter) {
		f.rules = append(f.rules, rule{
			ruleType: ruleInclude,
			pattern:  pattern,
		})
	}
}

// Exclude adds an exclude pattern.
// Files matching any exclude pattern are excluded.
// Exclude rules take precedence over include rules.
func Exclude(pattern string) Option {
	return func(f *Filter) {
		f.rules = append(f.rules, rule{
			ruleType: ruleExclude,
			pattern:  pattern,
		})
	}
}

// MinSize sets the minimum file size filter.
// Files smaller than this are excluded.
func MinSize(size int64) Option {
	return func(f *Filter) {
		f.rules = append(f.rules, rule{
			ruleType: ruleMinSize,
			size:     size,
		})
	}
}

// MaxSize sets the maximum file size filter.
// Files larger than this are excluded.
func MaxSize(size int64) Option {
	return func(f *Filter) {
		f.rules = append(f.rules, rule{
			ruleType: ruleMaxSize,
			size:     size,
		})
	}
}

// MinAge sets the minimum file age filter.
// Files newer than this are excluded.
// Age is calculated as time since modification.
func MinAge(d time.Duration) Option {
	return func(f *Filter) {
		f.rules = append(f.rules, rule{
			ruleType: ruleMinAge,
			duration: d,
		})
	}
}

// MaxAge sets the maximum file age filter.
// Files older than this are excluded.
// Age is calculated as time since modification.
func MaxAge(d time.Duration) Option {
	return func(f *Filter) {
		f.rules = append(f.rules, rule{
			ruleType: ruleMaxAge,
			duration: d,
		})
	}
}

// FromFile loads filter rules from a file.
// Each line is a pattern. Lines starting with + are includes,
// lines starting with - are excludes. Empty lines and lines
// starting with # are ignored.
//
// Example file:
//
//	# Include JSON files
//	+ *.json
//	# Exclude temp files
//	- *.tmp
//	- *.bak
func FromFile(path string) (Option, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var opts []Option
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "+ ") {
			pattern := strings.TrimPrefix(line, "+ ")
			opts = append(opts, Include(pattern))
		} else if strings.HasPrefix(line, "- ") {
			pattern := strings.TrimPrefix(line, "- ")
			opts = append(opts, Exclude(pattern))
		} else {
			// Default to exclude
			opts = append(opts, Exclude(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return func(f *Filter) {
		for _, opt := range opts {
			opt(f)
		}
	}, nil
}

// Match returns true if the file passes the filter.
//
// The filtering logic:
//  1. If there are include patterns and the file doesn't match any, exclude it
//  2. If the file matches any exclude pattern, exclude it
//  3. If the file fails size or age constraints, exclude it
//  4. Otherwise, include the file
func (f *Filter) Match(fi FileInfo) bool {
	if f == nil || len(f.rules) == 0 {
		return true
	}

	// Check if there are any include rules
	hasIncludes := false
	matchesInclude := false
	for _, r := range f.rules {
		if r.ruleType == ruleInclude {
			hasIncludes = true
			if matchPattern(r.pattern, fi.Path) {
				matchesInclude = true
			}
		}
	}

	// If there are include patterns and none match, exclude
	if hasIncludes && !matchesInclude {
		return false
	}

	// Check exclude patterns and constraints
	for _, r := range f.rules {
		switch r.ruleType {
		case ruleExclude:
			if matchPattern(r.pattern, fi.Path) {
				return false
			}
		case ruleMinSize:
			if fi.Size < r.size {
				return false
			}
		case ruleMaxSize:
			if fi.Size > r.size {
				return false
			}
		case ruleMinAge:
			age := time.Since(fi.ModTime)
			if age < r.duration {
				return false
			}
		case ruleMaxAge:
			age := time.Since(fi.ModTime)
			if age > r.duration {
				return false
			}
		}
	}

	return true
}

// MatchPath is a convenience method that matches by path only.
func (f *Filter) MatchPath(path string) bool {
	return f.Match(FileInfo{Path: path})
}

// IsEmpty returns true if the filter has no rules.
func (f *Filter) IsEmpty() bool {
	return f == nil || len(f.rules) == 0
}

// matchPattern matches a pattern against a path.
// It tries both the full path and just the filename.
func matchPattern(pattern, path string) bool {
	// Try matching the full path
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	// Try matching just the filename
	filename := filepath.Base(path)
	if matched, _ := filepath.Match(pattern, filename); matched {
		return true
	}

	// Try matching with ** for directory wildcards
	if strings.Contains(pattern, "**") {
		// Simple ** handling: replace ** with * and try
		simplePattern := strings.ReplaceAll(pattern, "**", "*")
		if matched, _ := filepath.Match(simplePattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(simplePattern, filename); matched {
			return true
		}
	}

	return false
}

// Common file size constants for convenience.
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)
