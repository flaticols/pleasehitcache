// Package recommendation provides the recommendation engine.
package recommendation

import (
	"fmt"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// Calculator defines the interface for layout calculations.
type Calculator interface {
	CacheLineSize() int64
	CalculatePadding(info *itypes.StructInfo) int64
	IsPaddingReasonable(info *itypes.StructInfo) bool
	SuggestReorder(info *itypes.StructInfo) *itypes.ReorderSuggestion
}

// Engine generates recommendations for structs.
type Engine struct {
	calculator Calculator
	threshold  int
}

// New creates a new recommendation Engine.
func New(calculator Calculator, threshold int) *Engine {
	return &Engine{
		calculator: calculator,
		threshold:  threshold,
	}
}

// Generate creates a recommendation for a struct.
func (e *Engine) Generate(info *itypes.StructInfo) *itypes.Recommendation {
	cacheLineSize := e.calculator.CacheLineSize()

	// Check anti-patterns first
	for _, ap := range info.AntiPatterns {
		switch ap.Type {
		case itypes.AntiPatternSliceElement:
			return &itypes.Recommendation{
				Action:        itypes.ActionNoPad,
				Reason:        "struct is used as slice element; padding would increase total memory usage",
				CurrentSize:   info.Size,
				CacheLineSize: cacheLineSize,
			}

		case itypes.AntiPatternEmbedded:
			return &itypes.Recommendation{
				Action:        itypes.ActionNoPad,
				Reason:        "struct is embedded in another struct; padding may cause unintended layout changes",
				CurrentSize:   info.Size,
				CacheLineSize: cacheLineSize,
			}

		case itypes.AntiPatternExceedsCacheLine:
			return &itypes.Recommendation{
				Action:        itypes.ActionNoPad,
				Reason:        fmt.Sprintf("struct size (%d bytes) already exceeds cache line (%d bytes)", info.Size, cacheLineSize),
				CurrentSize:   info.Size,
				CacheLineSize: cacheLineSize,
			}
		}
	}

	// Already at or exceeds cache line size
	if info.Size >= cacheLineSize {
		reason := "struct already matches cache line size"
		if info.Size > cacheLineSize {
			reason = fmt.Sprintf("struct size (%d bytes) already exceeds cache line (%d bytes)", info.Size, cacheLineSize)
		}
		return &itypes.Recommendation{
			Action:        itypes.ActionNoPad,
			Reason:        reason,
			CurrentSize:   info.Size,
			CacheLineSize: cacheLineSize,
		}
	}

	// Calculate padding
	paddingNeeded := e.calculator.CalculatePadding(info)

	// Check if padding would more than double the size
	if !e.calculator.IsPaddingReasonable(info) {
		return &itypes.Recommendation{
			Action:        itypes.ActionWarn,
			Reason:        fmt.Sprintf("padding would add %d bytes (>100%% increase)", paddingNeeded),
			CurrentSize:   info.Size,
			OptimalSize:   cacheLineSize,
			PaddingNeeded: paddingNeeded,
			CacheLineSize: cacheLineSize,
		}
	}

	// Check for field reordering opportunity
	reorderSuggestion := e.calculator.SuggestReorder(info)

	// Padding is reasonable
	return &itypes.Recommendation{
		Action:            itypes.ActionPad,
		Reason:            "cache line padding recommended",
		CurrentSize:       info.Size,
		OptimalSize:       cacheLineSize,
		PaddingNeeded:     paddingNeeded,
		CacheLineSize:     cacheLineSize,
		ReorderSuggestion: reorderSuggestion,
	}
}

// ShouldAnalyze determines if a struct should be analyzed based on its score.
func (e *Engine) ShouldAnalyze(info *itypes.StructInfo, analyzeAll bool) bool {
	if analyzeAll {
		return true
	}
	return info.IsHot(e.threshold)
}
