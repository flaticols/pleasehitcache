package hotpath

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// analyzeLoops detects structs accessed in loops.
func (a *Analyzer) analyzeLoops(structs map[string]*itypes.StructInfo) {
	for _, file := range a.pass.Files {
		a.walkLoops(file, structs, 0)
	}
}

// walkLoops walks the AST looking for loop constructs.
func (a *Analyzer) walkLoops(node ast.Node, structs map[string]*itypes.StructInfo, depth int) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.ForStmt:
			// Regular for loop
			a.analyzeLoopBody(stmt.Body, structs, depth+1, stmt.Pos())
			return false // Don't recurse, we handle body manually

		case *ast.RangeStmt:
			// Range loop - check the iterated type
			a.analyzeRangeLoop(stmt, structs, depth+1)
			return false
		}
		return true
	})
}

// analyzeLoopBody finds structs accessed within a loop body.
func (a *Analyzer) analyzeLoopBody(body *ast.BlockStmt, structs map[string]*itypes.StructInfo, depth int, loopPos token.Pos) {
	if body == nil {
		return
	}

	// Look for struct accesses in the loop body
	ast.Inspect(body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.SelectorExpr:
			// Field or method access: x.Field or x.Method()
			a.checkStructAccess(expr.X, structs, depth, loopPos)

		case *ast.CallExpr:
			// Method call or function call with struct argument
			a.checkCallInLoop(expr, structs, depth, loopPos)

		case *ast.ForStmt, *ast.RangeStmt:
			// Nested loop - recurse with increased depth
			a.walkLoops(n, structs, depth)
			return false
		}
		return true
	})
}

// analyzeRangeLoop analyzes a range statement for struct access.
func (a *Analyzer) analyzeRangeLoop(stmt *ast.RangeStmt, structs map[string]*itypes.StructInfo, depth int) {
	// Check the type being ranged over
	if tv, ok := a.pass.TypesInfo.Types[stmt.X]; ok {
		// Check if ranging over []StructType
		if slice, ok := tv.Type.Underlying().(*types.Slice); ok {
			if named, ok := slice.Elem().(*types.Named); ok {
				name := named.Obj().Name()
				if info, exists := structs[name]; exists {
					hotType := itypes.HotPathLoopSimple
					if depth > 1 {
						hotType = itypes.HotPathLoopNested
					}
					info.AddHotPath(itypes.HotPathSource{
						Type:     hotType,
						Score:    hotType.Score(),
						Location: stmt.Pos(),
						Details:  formatLoopDetails(depth),
					})
				}
			}
		}
	}

	// Also analyze the loop body
	a.analyzeLoopBody(stmt.Body, structs, depth, stmt.Pos())
}

// checkStructAccess checks if an expression accesses a struct type.
func (a *Analyzer) checkStructAccess(expr ast.Expr, structs map[string]*itypes.StructInfo, depth int, loopPos token.Pos) {
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
			hotType := itypes.HotPathLoopSimple
			if depth > 1 {
				hotType = itypes.HotPathLoopNested
			}
			// Only add if not already detected at higher score
			if !hasHotPathType(info, hotType) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     hotType,
					Score:    hotType.Score(),
					Location: loopPos,
					Details:  formatLoopDetails(depth),
				})
			}
		}
	}
}

// checkCallInLoop checks for struct usage in function calls within loops.
func (a *Analyzer) checkCallInLoop(call *ast.CallExpr, structs map[string]*itypes.StructInfo, depth int, loopPos token.Pos) {
	// Check arguments for struct types
	for _, arg := range call.Args {
		a.checkStructAccess(arg, structs, depth, loopPos)
	}

	// Check method receiver
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		a.checkStructAccess(sel.X, structs, depth, loopPos)
	}
}

// hasHotPathType checks if a struct already has a hot path source of the given type.
func hasHotPathType(info *itypes.StructInfo, t itypes.HotPathType) bool {
	for _, hp := range info.HotPathSources {
		if hp.Type == t {
			return true
		}
	}
	return false
}

func formatLoopDetails(depth int) string {
	if depth > 1 {
		return "nested loop (depth " + strconv.Itoa(depth) + ")"
	}
	return "loop iteration"
}
