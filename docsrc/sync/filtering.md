# Filtering

The filter package provides include/exclude patterns, size filters, and age filters for sync operations.

## Basic Usage

```go
import "github.com/grokify/omnistorage/sync/filter"

f := filter.New(
    filter.Include("*.json"),
    filter.Exclude("*.tmp"),
)

result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Filter: f,
})
```

## Pattern Matching

### Include Patterns

Include only files matching patterns:

```go
f := filter.New(
    filter.Include("*.json"),
    filter.Include("*.yaml"),
    filter.Include("data/**"),
)
```

### Exclude Patterns

Exclude files matching patterns:

```go
f := filter.New(
    filter.Exclude("*.tmp"),
    filter.Exclude("*.bak"),
    filter.Exclude(".git/**"),
    filter.Exclude("node_modules/**"),
)
```

### Pattern Syntax

| Pattern | Matches |
|---------|---------|
| `*.json` | Any file ending in `.json` |
| `*.{json,yaml}` | Files ending in `.json` or `.yaml` |
| `data/*` | Files directly in `data/` directory |
| `data/**` | All files under `data/` recursively |
| `??.txt` | Two-character name + `.txt` |
| `[abc].txt` | `a.txt`, `b.txt`, or `c.txt` |

### Combining Include/Exclude

When both include and exclude patterns are specified:

1. Include patterns are checked first
2. Then exclude patterns filter the results

```go
f := filter.New(
    filter.Include("*.json"),      // Include JSON files
    filter.Exclude("test_*.json"), // But exclude test files
)
```

## Size Filters

Filter by file size:

```go
f := filter.New(
    filter.MinSize(1024),           // At least 1 KB
    filter.MaxSize(100 * 1024 * 1024), // At most 100 MB
)
```

### Size Constants

```go
const (
    KB = 1024
    MB = 1024 * KB
    GB = 1024 * MB
)

f := filter.New(
    filter.MinSize(10 * KB),
    filter.MaxSize(1 * GB),
)
```

## Age Filters

Filter by modification time:

```go
f := filter.New(
    filter.MinAge(24 * time.Hour),    // Older than 1 day
    filter.MaxAge(7 * 24 * time.Hour), // Newer than 7 days
)
```

### Common Age Filters

```go
// Files modified in the last hour
filter.MaxAge(time.Hour)

// Files older than 30 days
filter.MinAge(30 * 24 * time.Hour)

// Files between 1 and 7 days old
filter.New(
    filter.MinAge(24 * time.Hour),
    filter.MaxAge(7 * 24 * time.Hour),
)
```

## Filter From File

Load filters from a file:

```go
f, err := filter.FromFile("filters.txt")
if err != nil {
    log.Fatal(err)
}
```

### Filter File Format

```
# Comments start with #
# Include patterns (prefix with +)
+ *.json
+ *.yaml
+ data/**

# Exclude patterns (prefix with -)
- *.tmp
- *.bak
- .git/**
- node_modules/**

# Size filters
--min-size 1K
--max-size 100M

# Age filters
--min-age 1d
--max-age 7d
```

### Size Units in Files

| Unit | Value |
|------|-------|
| `B` | Bytes |
| `K` | Kilobytes |
| `M` | Megabytes |
| `G` | Gigabytes |

### Age Units in Files

| Unit | Value |
|------|-------|
| `s` | Seconds |
| `m` | Minutes |
| `h` | Hours |
| `d` | Days |
| `w` | Weeks |
| `M` | Months (30 days) |
| `y` | Years (365 days) |

## Delete Excluded

Delete files in destination that match exclude patterns:

```go
result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Filter: f,
    DeleteExcluded: true, // Delete excluded files from destination
})
```

## Combined Example

```go
f := filter.New(
    // Include patterns
    filter.Include("*.json"),
    filter.Include("*.yaml"),
    filter.Include("config/**"),

    // Exclude patterns
    filter.Exclude("*.tmp"),
    filter.Exclude("*.bak"),
    filter.Exclude(".git/**"),
    filter.Exclude("test_*.json"),

    // Size limits
    filter.MinSize(100),        // At least 100 bytes
    filter.MaxSize(10 * MB),    // At most 10 MB

    // Age limits
    filter.MaxAge(30 * 24 * time.Hour), // Modified in last 30 days
)

result, err := sync.Sync(ctx, src, dst, "", "", sync.Options{
    Filter: f,
    DeleteExtra: true,
})
```

## Checking Filter Matches

```go
f := filter.New(filter.Include("*.json"))

// Check if a file passes the filter
if f.Match("data.json", info) {
    fmt.Println("File passes filter")
}
```
