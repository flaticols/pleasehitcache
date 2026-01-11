// Package types defines shared types used across the analyzer.
package types

import (
	"go/ast"
	"go/token"
	"go/types"
)

// FieldInfo holds information about a struct field's layout.
type FieldInfo struct {
	Name      string
	Type      string
	Offset    int64
	Size      int64
	Alignment int64
}

// HotPathSource identifies how a struct was detected as hot.
type HotPathSource struct {
	Type     HotPathType
	Score    int
	Location token.Pos
	Details  string // e.g., "BenchmarkCounter", "worker.go:42"
}

// HotPathType categorizes hot path detection methods.
type HotPathType int

const (
	HotPathDirective HotPathType = iota
	HotPathPprof
	HotPathBenchmark
	HotPathHTTPHandler
	HotPathLoopNested
	HotPathGoroutine
	HotPathChannel
	HotPathLoopSimple
	HotPathSyncPool
	HotPathMutex
	HotPathAtomic
)

// Score returns the confidence score for this hot path type.
func (t HotPathType) Score() int {
	switch t {
	case HotPathDirective:
		return 100
	case HotPathPprof:
		return 90
	case HotPathBenchmark:
		return 85
	case HotPathHTTPHandler:
		return 80
	case HotPathLoopNested:
		return 75
	case HotPathGoroutine:
		return 70
	case HotPathChannel:
		return 65
	case HotPathLoopSimple:
		return 60
	case HotPathSyncPool:
		return 55
	case HotPathMutex:
		return 50
	case HotPathAtomic:
		return 50
	default:
		return 0
	}
}

// String returns a human-readable name for the hot path type.
func (t HotPathType) String() string {
	switch t {
	case HotPathDirective:
		return "//go:cachepad directive"
	case HotPathPprof:
		return "pprof hot function"
	case HotPathBenchmark:
		return "benchmark function"
	case HotPathHTTPHandler:
		return "HTTP handler"
	case HotPathLoopNested:
		return "nested loop"
	case HotPathGoroutine:
		return "goroutine access"
	case HotPathChannel:
		return "channel operation"
	case HotPathLoopSimple:
		return "loop iteration"
	case HotPathSyncPool:
		return "sync.Pool usage"
	case HotPathMutex:
		return "sync.Mutex field"
	case HotPathAtomic:
		return "atomic field"
	default:
		return "unknown"
	}
}

// AntiPatternType categorizes anti-patterns.
type AntiPatternType int

const (
	AntiPatternNone AntiPatternType = iota
	AntiPatternSliceElement
	AntiPatternEmbedded
	AntiPatternExceedsCacheLine
)

// String returns a human-readable name for the anti-pattern.
func (t AntiPatternType) String() string {
	switch t {
	case AntiPatternSliceElement:
		return "slice element"
	case AntiPatternEmbedded:
		return "embedded struct"
	case AntiPatternExceedsCacheLine:
		return "exceeds cache line"
	default:
		return ""
	}
}

// AntiPattern holds information about detected anti-patterns.
type AntiPattern struct {
	Type     AntiPatternType
	Location token.Pos
	Details  string
}

// StructInfo holds analyzed struct information.
type StructInfo struct {
	Name       string
	TypeSpec   *ast.TypeSpec
	StructType *types.Struct
	ASTStruct  *ast.StructType
	Pos        token.Pos
	File       *ast.File
	PkgPath    string

	// Layout
	Size      int64
	Alignment int64
	Fields    []FieldInfo

	// Hot path detection
	HotPathSources []HotPathSource
	TotalScore     int

	// Anti-patterns
	AntiPatterns []AntiPattern
}

// IsHot returns true if the struct's score meets the threshold.
func (s *StructInfo) IsHot(threshold int) bool {
	return s.TotalScore >= threshold
}

// HasAntiPattern returns true if any anti-pattern was detected.
func (s *StructInfo) HasAntiPattern() bool {
	return len(s.AntiPatterns) > 0
}

// AddHotPath adds a hot path source and updates the total score.
func (s *StructInfo) AddHotPath(source HotPathSource) {
	s.HotPathSources = append(s.HotPathSources, source)
	if source.Score > s.TotalScore {
		s.TotalScore = source.Score
	}
}

// ActionType represents the recommended action.
type ActionType int

const (
	ActionPad ActionType = iota
	ActionReorder
	ActionWarn
	ActionNoPad
)

// String returns a human-readable name for the action.
func (a ActionType) String() string {
	switch a {
	case ActionPad:
		return "pad"
	case ActionReorder:
		return "reorder"
	case ActionWarn:
		return "warn"
	case ActionNoPad:
		return "no_pad"
	default:
		return "unknown"
	}
}

// ReorderSuggestion holds field reordering information.
type ReorderSuggestion struct {
	OriginalOrder []string
	OptimalOrder  []string
	OriginalSize  int64
	OptimalSize   int64
	BytesSaved    int64
}

// Recommendation holds the analysis result for a struct.
type Recommendation struct {
	Action          ActionType
	Reason          string
	CurrentSize     int64
	OptimalSize     int64
	PaddingNeeded   int64
	CacheLineSize   int64
	ReorderSuggestion *ReorderSuggestion
}
