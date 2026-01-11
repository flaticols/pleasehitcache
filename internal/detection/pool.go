package detection

import (
	"go/ast"
	"go/types"
	"strings"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// FindPoolStoredTypes finds structs stored in sync.Pool.
func FindPoolStoredTypes(pass *analysis.Pass, structs map[string]*itypes.StructInfo) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Put" {
				return true
			}

			// Check if receiver is sync.Pool
			tv, ok := pass.TypesInfo.Types[sel.X]
			if !ok {
				return true
			}

			typeName := tv.Type.String()
			if !strings.Contains(typeName, "sync.Pool") {
				return true
			}

			if len(call.Args) == 0 {
				return true
			}

			// Get the type being stored
			argTV := pass.TypesInfo.Types[call.Args[0]]
			if argTV.Type == nil {
				return true
			}

			argType := argTV.Type
			// Unwrap pointer
			if ptr, ok := argType.(*types.Pointer); ok {
				argType = ptr.Elem()
			}

			// Find the struct
			if named, ok := argType.(*types.Named); ok {
				name := named.Obj().Name()
				if info, exists := structs[name]; exists {
					info.AddHotPath(itypes.HotPathSource{
						Type:     itypes.HotPathSyncPool,
						Score:    itypes.HotPathSyncPool.Score(),
						Location: call.Pos(),
						Details:  "sync.Pool.Put()",
					})
				}
			}
			return true
		})
	}
}
