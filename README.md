# pleasehitcache

A Go static analyzer for cache line optimization. Detects structs that may benefit from cache line padding to prevent false sharing, and identifies high-contention atomic counters that would benefit from sharding.

## Installation

```bash
go install github.com/flaticols/pleasehitcache@latest
```

Or via Homebrew:
```bash
brew install flaticols/apps/pleasehitcache
```

## Usage

```bash
# Analyze packages
pleasehitcache ./...

# Specify cache line size (auto-detected by default)
pleasehitcache -cache-line-size=64 ./...
pleasehitcache -cache-line-size=128 ./...

# Use pprof data to identify hot paths
pleasehitcache -pprof=cpu.prof ./...

# Analyze all structs (not just heuristically detected ones)
pleasehitcache -analyze-all ./...

# JSON output for CI
pleasehitcache -output=json ./... 2>&1

# Auto-fix (add padding fields)
pleasehitcache -fix ./...

# Ignore patterns
pleasehitcache -ignore="*Test,internal/*" ./...
```

## GitHub Action

Use in your CI pipeline:

```yaml
- uses: flaticols/pleasehitcache@latest
  with:
    path: './...'
    # cache-line-size: '64'  # optional
    # analyze-all: 'true'    # optional
```

### go vet integration

```bash
go vet -vettool=$(which pleasehitcache) ./...
```

### golangci-lint v2 integration

1. Create `.custom-gcl.yml`:
```yaml
version: v2.1.2
plugins:
  - module: 'github.com/flaticols/pleasehitcache'
    import: 'github.com/flaticols/pleasehitcache'
    version: latest
```

2. Build custom binary:
```bash
golangci-lint-custom
```

3. Configure `.golangci.yml`:
```yaml
version: "2"
linters:
  enable:
    - cachepad
```

## Analyzers

### 1. Cache Line Padding (`cachepad`)

Detects structs that would benefit from padding to prevent false sharing.

**Suggests padding when:**
- False sharing risk detected (concurrent access patterns)
- Struct fits in cache line with reasonable padding
- Padding wouldn't more than double struct size

**Warns against padding when:**
- Struct is used as slice element (padding multiplies memory)
- Struct is embedded in another struct
- Struct already exceeds cache line size

### 2. Sharded Counter Detection (`shardedcounter`)

Detects atomic counters accessed from multiple goroutines that would benefit from sharding to reduce contention.

**Problem:** Single atomic counter with many goroutines writing causes cache line contention.

**Solution:** Per-goroutine/sharded counters, sum on read:

```go
// Bad: contention on writes
type Counter struct {
    value atomic.Int64
}

// Good: no write contention
type ShardedCounter struct {
    shards [NumShards]struct {
        value atomic.Int64
        _     [56]byte // padding
    }
}

func (c *ShardedCounter) Add(delta int64) {
    id := getShardID()
    c.shards[id].value.Add(delta)
}

func (c *ShardedCounter) Load() int64 {
    var total int64
    for i := range c.shards {
        total += c.shards[i].value.Load()
    }
    return total
}
```

## Hot Path Detection

The analyzer uses multiple methods to identify performance-critical structs:

| Method | Score | Description |
|--------|-------|-------------|
| `//go:cachepad` directive | 100 | Explicit marking |
| pprof hot function | 90 | Runtime profiling data |
| Benchmark function | 85 | Used in `Benchmark*` with `b.N` |
| HTTP handler | 80 | Request-per-second paths |
| Nested loop | 75 | O(n²) or worse iteration |
| Goroutine access | 70 | Concurrent access pattern |
| Channel operation | 65 | Often in hot paths |
| Simple loop | 60 | O(n) iteration |
| sync.Pool usage | 55 | Allocation optimization |
| sync.Mutex field | 50 | Concurrent access likely |
| atomic field | 50 | Concurrent access |

**Threshold:** Structs with score >= 50 are analyzed.

## Detection Methods

### 1. Directive (explicit)

Mark structs with `//go:cachepad`:

```go
//go:cachepad
type Counter struct {
    mu    sync.Mutex
    value int64
}
```

### 2. AST-based Heuristics (automatic)

The analyzer walks the AST to detect:
- `sync.Mutex` or `sync.RWMutex` fields
- `atomic.*` typed fields
- Usage in `sync.Pool`
- Access in loops (for, range)
- Access in goroutines
- Usage in HTTP handlers
- Usage in benchmark functions
- Channel operations

### 3. pprof hot paths

Use `-pprof=cpu.prof` to prioritize structs appearing in hot paths from actual profiling data.

## Cache Line Sizes

| Architecture | Size |
|--------------|------|
| x86-64 (amd64) | 64 bytes |
| ARM64 (linux) | 64 bytes |
| Apple M-series (darwin/arm64) | 128 bytes |

Auto-detected from `GOARCH`/`GOOS` or override with `-cache-line-size`.

## Example Output

```
counter.go:15:6: struct 'Counter' should add 32-byte padding for cache line alignment
    Hot path score: 70/100
    Detected via:
      - sync.Mutex field (sync.Mutex field)
      - goroutine access (captured in goroutine closure)
    Size: 32 bytes, Cache line: 64 bytes
    Layout:
        mu           sync.Mutex             0- 24 (24 bytes)
        value        int64                 24- 32 (8 bytes)
    Recommendation: add `_ [32]byte` padding field

stats.go:8:6: atomic counter 'Stats.requests' has high contention risk
    Detected access from 3 goroutine(s) + loop access
    Access locations:
      - goroutine at worker.go:42
      - goroutine at handler.go:18
      - loop at process.go:55

    Suggestion: Use sharded counter pattern...
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-cache-line-size` | `auto` | Cache line size: `auto`, `64`, or `128` |
| `-pprof` | | Path to CPU/memory profile for hot path detection |
| `-analyze-all` | `false` | Analyze all structs, not just detected ones |
| `-output` | `text` | Output format: `text` or `json` |
| `-ignore` | | Comma-separated patterns to ignore |
| `-fix` | `false` | Auto-fix by adding padding fields |

## Architecture

```
internal/
├── analysis/              # Pluggable analyzer framework
│   ├── analysis.go        # Analyzer interface and Chain
│   ├── cachepad/          # Cache line padding analyzer
│   └── shardedcounter/    # Sharded counter detector
├── detection/             # Struct discovery
├── hotpath/               # AST-based hot path detection
├── layout/                # Struct size/alignment calculation
├── antipattern/           # Anti-pattern detection
├── recommendation/        # Recommendation engine
└── output/                # Text/JSON formatting
```

## License

MIT
