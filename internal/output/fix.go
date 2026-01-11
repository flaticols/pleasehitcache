package output

import (
	"fmt"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// CreatePaddingFix creates a SuggestedFix for adding padding to a struct.
func CreatePaddingFix(info *itypes.StructInfo, rec *itypes.Recommendation) *analysis.SuggestedFix {
	if rec.Action != itypes.ActionPad {
		return nil
	}

	if info.ASTStruct == nil || len(info.ASTStruct.Fields.List) == 0 {
		return nil
	}

	// Find position after last field
	lastField := info.ASTStruct.Fields.List[len(info.ASTStruct.Fields.List)-1]
	insertPos := lastField.End()

	paddingText := fmt.Sprintf("\n\t_ [%d]byte // cache line padding (total: %d bytes)",
		rec.PaddingNeeded, rec.CacheLineSize)

	return &analysis.SuggestedFix{
		Message: fmt.Sprintf("add %d-byte cache line padding", rec.PaddingNeeded),
		TextEdits: []analysis.TextEdit{
			{
				Pos:     insertPos,
				End:     insertPos,
				NewText: []byte(paddingText),
			},
		},
	}
}

// CreateReorderFix creates a SuggestedFix for reordering struct fields.
// This is more complex and would require significant AST manipulation.
// For now, we just suggest the reorder in the output text.
func CreateReorderFix(info *itypes.StructInfo, suggestion *itypes.ReorderSuggestion) *analysis.SuggestedFix {
	// TODO: Implement field reordering fix
	// This requires:
	// 1. Reading the original source
	// 2. Extracting each field declaration
	// 3. Reordering them
	// 4. Generating new source text
	return nil
}
