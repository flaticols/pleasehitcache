package hotpath

import (
	"go/ast"
	"go/token"
	"go/types"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// analyzeChannels detects structs used in channel operations.
func (a *Analyzer) analyzeChannels(structs map[string]*itypes.StructInfo) {
	for _, file := range a.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.SelectStmt:
				a.analyzeSelect(stmt, structs)

			case *ast.SendStmt:
				a.analyzeSend(stmt, structs)

			case *ast.UnaryExpr:
				// Check for channel receive: <-ch
				if stmt.Op == token.ARROW {
					a.analyzeReceive(stmt, structs)
				}
			}
			return true
		})
	}
}

// analyzeSelect analyzes a select statement for struct usage.
func (a *Analyzer) analyzeSelect(stmt *ast.SelectStmt, structs map[string]*itypes.StructInfo) {
	if stmt.Body == nil {
		return
	}

	for _, clause := range stmt.Body.List {
		commClause, ok := clause.(*ast.CommClause)
		if !ok {
			continue
		}

		// Check the communication statement
		switch comm := commClause.Comm.(type) {
		case *ast.SendStmt:
			// ch <- value
			a.markChannelStruct(comm.Value, stmt.Pos(), structs, "sent on channel in select")

		case *ast.AssignStmt:
			// value := <-ch or value, ok := <-ch
			if len(comm.Rhs) > 0 {
				if unary, ok := comm.Rhs[0].(*ast.UnaryExpr); ok && unary.Op == token.ARROW {
					// Mark the received type
					a.markChannelReceiveType(unary, stmt.Pos(), structs)
				}
			}

		case *ast.ExprStmt:
			// <-ch (receive without assignment)
			if unary, ok := comm.X.(*ast.UnaryExpr); ok && unary.Op == token.ARROW {
				a.markChannelReceiveType(unary, stmt.Pos(), structs)
			}
		}

		// Analyze the case body
		for _, bodyStmt := range commClause.Body {
			ast.Inspect(bodyStmt, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					a.markChannelStruct(sel.X, stmt.Pos(), structs, "accessed in select case")
				}
				return true
			})
		}
	}
}

// analyzeSend analyzes a channel send operation.
func (a *Analyzer) analyzeSend(stmt *ast.SendStmt, structs map[string]*itypes.StructInfo) {
	a.markChannelStruct(stmt.Value, stmt.Pos(), structs, "sent on channel")
}

// analyzeReceive analyzes a channel receive operation.
func (a *Analyzer) analyzeReceive(expr *ast.UnaryExpr, structs map[string]*itypes.StructInfo) {
	a.markChannelReceiveType(expr, expr.Pos(), structs)
}

// markChannelStruct marks a struct as used in a channel operation.
func (a *Analyzer) markChannelStruct(expr ast.Expr, pos token.Pos, structs map[string]*itypes.StructInfo, details string) {
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
			if !hasHotPathType(info, itypes.HotPathChannel) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     itypes.HotPathChannel,
					Score:    itypes.HotPathChannel.Score(),
					Location: pos,
					Details:  details,
				})
			}
		}
	}
}

// markChannelReceiveType marks the element type of a channel as used.
func (a *Analyzer) markChannelReceiveType(expr *ast.UnaryExpr, pos token.Pos, structs map[string]*itypes.StructInfo) {
	tv, ok := a.pass.TypesInfo.Types[expr.X]
	if !ok {
		return
	}

	// Get the channel's element type
	chanType, ok := tv.Type.Underlying().(*types.Chan)
	if !ok {
		return
	}

	elemType := chanType.Elem()
	if ptr, ok := elemType.(*types.Pointer); ok {
		elemType = ptr.Elem()
	}

	if named, ok := elemType.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			if !hasHotPathType(info, itypes.HotPathChannel) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     itypes.HotPathChannel,
					Score:    itypes.HotPathChannel.Score(),
					Location: pos,
					Details:  "received from channel",
				})
			}
		}
	}
}
