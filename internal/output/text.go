// Package output provides output formatting for the analyzer.
package output

import (
	"fmt"
	"strings"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// FormatText creates a detailed text message for a struct recommendation.
func FormatText(info *itypes.StructInfo, rec *itypes.Recommendation) string {
	var sb strings.Builder

	// Header with action
	switch rec.Action {
	case itypes.ActionPad:
		sb.WriteString(fmt.Sprintf("struct '%s' should add %d-byte padding for cache line alignment",
			info.Name, rec.PaddingNeeded))
	case itypes.ActionWarn:
		sb.WriteString(fmt.Sprintf("struct '%s' could benefit from padding but it would significantly increase size (%d -> %d bytes)",
			info.Name, rec.CurrentSize, rec.OptimalSize))
	case itypes.ActionNoPad:
		sb.WriteString(fmt.Sprintf("struct '%s' - padding NOT recommended: %s",
			info.Name, rec.Reason))
	}

	// Hot path score and detection sources
	if len(info.HotPathSources) > 0 {
		sb.WriteString(fmt.Sprintf("\n    Hot path score: %d/100", info.TotalScore))
		sb.WriteString("\n    Detected via:")
		for _, hp := range info.HotPathSources {
			sb.WriteString(fmt.Sprintf("\n      - %s", hp.Type.String()))
			if hp.Details != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", hp.Details))
			}
		}
	}

	// Layout information
	sb.WriteString(fmt.Sprintf("\n    Size: %d bytes, Cache line: %d bytes", info.Size, rec.CacheLineSize))

	if len(info.Fields) > 0 {
		sb.WriteString("\n    Layout:")
		for _, f := range info.Fields {
			sb.WriteString(fmt.Sprintf("\n        %-12s %-20s %3d-%3d (%d bytes)",
				f.Name, f.Type, f.Offset, f.Offset+f.Size, f.Size))
		}
	}

	// Recommendations
	if rec.Action == itypes.ActionPad {
		sb.WriteString(fmt.Sprintf("\n    Recommendation: add `_ [%d]byte` padding field", rec.PaddingNeeded))

		// Field reorder suggestion
		if rec.ReorderSuggestion != nil {
			sb.WriteString(fmt.Sprintf("\n    Also consider reordering fields to save %d bytes:",
				rec.ReorderSuggestion.BytesSaved))
			sb.WriteString(fmt.Sprintf("\n        Current order: %s",
				strings.Join(rec.ReorderSuggestion.OriginalOrder, ", ")))
			sb.WriteString(fmt.Sprintf("\n        Optimal order: %s",
				strings.Join(rec.ReorderSuggestion.OptimalOrder, ", ")))
		}
	}

	// Anti-pattern details
	if len(info.AntiPatterns) > 0 {
		sb.WriteString("\n    Anti-patterns detected:")
		for _, ap := range info.AntiPatterns {
			sb.WriteString(fmt.Sprintf("\n      - %s: %s", ap.Type.String(), ap.Details))
		}
	}

	return sb.String()
}

// FormatShort creates a short summary message.
func FormatShort(info *itypes.StructInfo, rec *itypes.Recommendation) string {
	switch rec.Action {
	case itypes.ActionPad:
		return fmt.Sprintf("struct '%s' should add %d-byte padding", info.Name, rec.PaddingNeeded)
	case itypes.ActionWarn:
		return fmt.Sprintf("struct '%s' could benefit from padding (%d -> %d bytes)",
			info.Name, rec.CurrentSize, rec.OptimalSize)
	case itypes.ActionNoPad:
		return fmt.Sprintf("struct '%s' - padding NOT recommended: %s", info.Name, rec.Reason)
	default:
		return fmt.Sprintf("struct '%s' analyzed", info.Name)
	}
}
