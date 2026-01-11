package hotpath

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// analyzeBenchmarks detects structs used in benchmark functions.
func (a *Analyzer) analyzeBenchmarks(structs map[string]*itypes.StructInfo) {
	for _, file := range a.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Check if this is a benchmark function
			if a.isBenchmarkFunction(funcDecl) {
				a.analyzeBenchmarkBody(funcDecl, structs)
			}

			return true
		})
	}
}

// isBenchmarkFunction checks if a function is a Go benchmark.
func (a *Analyzer) isBenchmarkFunction(funcDecl *ast.FuncDecl) bool {
	// Must start with "Benchmark"
	if !strings.HasPrefix(funcDecl.Name.Name, "Benchmark") {
		return false
	}

	// Must have exactly one parameter of type *testing.B
	if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) != 1 {
		return false
	}

	param := funcDecl.Type.Params.List[0]
	tv, ok := a.pass.TypesInfo.Types[param.Type]
	if !ok {
		return false
	}

	return strings.HasSuffix(tv.Type.String(), "*testing.B")
}

// analyzeBenchmarkBody finds structs accessed in a benchmark function.
func (a *Analyzer) analyzeBenchmarkBody(funcDecl *ast.FuncDecl, structs map[string]*itypes.StructInfo) {
	if funcDecl.Body == nil {
		return
	}

	benchmarkName := funcDecl.Name.Name

	// Look for b.N loops specifically
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		// Check for the b.N pattern in for loops
		if forStmt, ok := n.(*ast.ForStmt); ok {
			if a.isBNLoop(forStmt) {
				// This is the hot loop - analyze its body carefully
				a.analyzeBenchmarkLoop(forStmt.Body, funcDecl.Pos(), benchmarkName, structs)
			}
		}

		// Also check range loops
		if rangeStmt, ok := n.(*ast.RangeStmt); ok {
			// Analyze range loop body
			a.analyzeBenchmarkLoop(rangeStmt.Body, funcDecl.Pos(), benchmarkName, structs)
		}

		return true
	})

	// Also analyze structs created/used outside the loop
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.CompositeLit:
			// Struct literal creation
			a.markStructAsBenchmark(expr, funcDecl.Pos(), benchmarkName, structs)
		case *ast.SelectorExpr:
			// Field access
			a.markStructAsBenchmarkExpr(expr.X, funcDecl.Pos(), benchmarkName, structs)
		}
		return true
	})
}

// isBNLoop checks if a for loop is iterating over b.N.
func (a *Analyzer) isBNLoop(forStmt *ast.ForStmt) bool {
	if forStmt.Cond == nil {
		return false
	}

	// Look for i < b.N pattern
	binExpr, ok := forStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}

	// Check right side for b.N
	sel, ok := binExpr.Y.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	return sel.Sel.Name == "N"
}

// analyzeBenchmarkLoop analyzes the body of a b.N loop.
func (a *Analyzer) analyzeBenchmarkLoop(body *ast.BlockStmt, benchPos token.Pos, benchName string, structs map[string]*itypes.StructInfo) {
	if body == nil {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.SelectorExpr:
			a.markStructAsBenchmarkExpr(expr.X, benchPos, benchName, structs)
		case *ast.CallExpr:
			// Check method receiver
			if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
				a.markStructAsBenchmarkExpr(sel.X, benchPos, benchName, structs)
			}
			// Check arguments
			for _, arg := range expr.Args {
				a.markStructAsBenchmarkExpr(arg, benchPos, benchName, structs)
			}
		}
		return true
	})
}

// markStructAsBenchmark marks a composite literal's type as used in a benchmark.
func (a *Analyzer) markStructAsBenchmark(lit *ast.CompositeLit, benchPos token.Pos, benchName string, structs map[string]*itypes.StructInfo) {
	tv, ok := a.pass.TypesInfo.Types[lit]
	if !ok {
		return
	}

	typ := tv.Type
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			if !hasHotPathType(info, itypes.HotPathBenchmark) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     itypes.HotPathBenchmark,
					Score:    itypes.HotPathBenchmark.Score(),
					Location: benchPos,
					Details:  benchName,
				})
			}
		}
	}
}

// markStructAsBenchmarkExpr marks an expression's type as used in a benchmark.
func (a *Analyzer) markStructAsBenchmarkExpr(expr ast.Expr, benchPos token.Pos, benchName string, structs map[string]*itypes.StructInfo) {
	tv, ok := a.pass.TypesInfo.Types[expr]
	if !ok {
		return
	}

	typ := tv.Type
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			if !hasHotPathType(info, itypes.HotPathBenchmark) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     itypes.HotPathBenchmark,
					Score:    itypes.HotPathBenchmark.Score(),
					Location: benchPos,
					Details:  benchName,
				})
			}
		}
	}
}
