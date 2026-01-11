package hotpath

import (
	"go/ast"
	"go/token"
	"go/types"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// analyzeGoroutines detects structs accessed in goroutines.
func (a *Analyzer) analyzeGoroutines(structs map[string]*itypes.StructInfo) {
	for _, file := range a.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			goStmt, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}

			// Analyze the goroutine call
			a.analyzeGoStatement(goStmt, structs)
			return true
		})
	}
}

// analyzeGoStatement analyzes a go statement for struct access.
func (a *Analyzer) analyzeGoStatement(goStmt *ast.GoStmt, structs map[string]*itypes.StructInfo) {
	// Check if it's a function literal (closure)
	if funcLit, ok := goStmt.Call.Fun.(*ast.FuncLit); ok {
		// Find captured variables in the closure
		a.analyzeClosure(funcLit, goStmt.Pos(), structs)
	}

	// Check arguments passed to the goroutine function
	for _, arg := range goStmt.Call.Args {
		a.markStructAsGoroutineAccess(arg, goStmt.Pos(), structs)
	}

	// If it's a method call, check the receiver
	if sel, ok := goStmt.Call.Fun.(*ast.SelectorExpr); ok {
		a.markStructAsGoroutineAccess(sel.X, goStmt.Pos(), structs)
	}
}

// analyzeClosure finds structs captured in a closure.
func (a *Analyzer) analyzeClosure(funcLit *ast.FuncLit, goPos token.Pos, structs map[string]*itypes.StructInfo) {
	// Walk the closure body looking for struct accesses
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.SelectorExpr:
			// Check if accessing a struct field
			a.markStructAsGoroutineAccess(expr.X, goPos, structs)

		case *ast.Ident:
			// Check if it's a captured variable
			if obj := a.pass.TypesInfo.Uses[expr]; obj != nil {
				typ := obj.Type()
				// Unwrap pointer
				if ptr, ok := typ.(*types.Pointer); ok {
					typ = ptr.Elem()
				}
				if named, ok := typ.(*types.Named); ok {
					name := named.Obj().Name()
					if info, exists := structs[name]; exists {
						info.AddHotPath(itypes.HotPathSource{
							Type:     itypes.HotPathGoroutine,
							Score:    itypes.HotPathGoroutine.Score(),
							Location: goPos,
							Details:  "captured in goroutine closure",
						})
					}
				}
			}
		}
		return true
	})
}

// markStructAsGoroutineAccess marks a struct as accessed from a goroutine.
func (a *Analyzer) markStructAsGoroutineAccess(expr ast.Expr, goPos token.Pos, structs map[string]*itypes.StructInfo) {
	tv, ok := a.pass.TypesInfo.Types[expr]
	if !ok {
		return
	}

	typ := tv.Type
	// Unwrap pointer
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			if !hasHotPathType(info, itypes.HotPathGoroutine) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     itypes.HotPathGoroutine,
					Score:    itypes.HotPathGoroutine.Score(),
					Location: goPos,
					Details:  "passed to goroutine",
				})
			}
		}
	}
}
