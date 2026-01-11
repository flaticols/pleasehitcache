package layout

import (
	"sort"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// SuggestReorder analyzes field order and suggests optimization.
func (c *Calculator) SuggestReorder(info *itypes.StructInfo) *itypes.ReorderSuggestion {
	if len(info.Fields) < 2 {
		return nil
	}

	// Create a copy of fields for sorting
	fields := make([]itypes.FieldInfo, len(info.Fields))
	copy(fields, info.Fields)

	// Sort by alignment (descending), then by size (descending)
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].Alignment != fields[j].Alignment {
			return fields[i].Alignment > fields[j].Alignment
		}
		return fields[i].Size > fields[j].Size
	})

	// Calculate optimal size with new order
	optimalSize := c.calculatePackedSize(fields)

	// Only suggest if savings are significant (> 8 bytes)
	if info.Size-optimalSize <= 8 {
		return nil
	}

	// Build the suggestion
	originalOrder := make([]string, len(info.Fields))
	optimalOrder := make([]string, len(fields))

	for i, f := range info.Fields {
		originalOrder[i] = f.Name
	}
	for i, f := range fields {
		optimalOrder[i] = f.Name
	}

	return &itypes.ReorderSuggestion{
		OriginalOrder: originalOrder,
		OptimalOrder:  optimalOrder,
		OriginalSize:  info.Size,
		OptimalSize:   optimalSize,
		BytesSaved:    info.Size - optimalSize,
	}
}

// calculatePackedSize estimates the size with optimal field ordering.
func (c *Calculator) calculatePackedSize(fields []itypes.FieldInfo) int64 {
	if len(fields) == 0 {
		return 0
	}

	var offset int64
	var maxAlign int64

	for _, f := range fields {
		// Align offset to field alignment
		if f.Alignment > 0 {
			offset = align(offset, f.Alignment)
		}
		offset += f.Size

		if f.Alignment > maxAlign {
			maxAlign = f.Alignment
		}
	}

	// Align total size to struct alignment
	if maxAlign > 0 {
		offset = align(offset, maxAlign)
	}

	return offset
}

// align rounds up offset to the given alignment.
func align(offset, alignment int64) int64 {
	if alignment == 0 {
		return offset
	}
	return (offset + alignment - 1) / alignment * alignment
}
