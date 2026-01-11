# pleasehitcache

A Go static analyzer for cache line optimization. Detects structs that may benefit from cache line padding to prevent false sharing, and warns against inappropriate padding.

## Installation

```bash
go install github.com/flaticols/pleasehitcache@latest
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

### 2. Heuristics (automatic)

The analyzer detects structs with:
- `sync.Mutex` or `sync.RWMutex` fields
- `atomic.*` typed fields (`atomic.Int64`, `atomic.Bool`, etc.)
- Usage in `sync.Pool`

### 3. pprof hot paths

Use `-pprof=cpu.prof` to prioritize structs appearing in hot paths from actual profiling data.

## Recommendations

### Suggests padding when:
- False sharing risk detected (concurrent access patterns)
- Struct fits in cache line with reasonable padding
- Padding wouldn't more than double struct size

### Warns against padding when:
- Struct is used as slice element (padding multiplies memory)
- Struct is embedded in another struct
- Struct already exceeds cache line size

## Cache Line Sizes

| Architecture | Size |
|--------------|------|
| x86-64 (amd64) | 64 bytes |
| ARM64 (linux) | 64 bytes |
| Apple M-series (darwin/arm64) | 128 bytes |

Auto-detected from `GOARCH`/`GOOS` or override with `-cache-line-size`.

## Example Output

```
counter.go:15:6: struct 'Counter' should add 32-byte padding for cache line alignment [detected via: sync.Mutex]
    Size: 32 bytes, Cache line: 64 bytes
    Layout:
        mu           sync.Mutex             0- 24 (24 bytes)
        value        int64                 24- 32 (8 bytes)
    Fix: add `_ [32]byte` padding field

items.go:28:6: struct 'Item' - padding NOT recommended: struct is used as slice element [detected via: sync.Mutex]
    Size: 16 bytes, Cache line: 64 bytes
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
