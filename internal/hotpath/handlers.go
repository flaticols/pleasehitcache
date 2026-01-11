package hotpath

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// httpHandlerSignatures are function signatures that indicate HTTP handlers.
var httpHandlerSignatures = []struct {
	paramTypes []string
	name       string
}{
	{[]string{"http.ResponseWriter", "*http.Request"}, "http.HandlerFunc"},
	{[]string{"*gin.Context"}, "gin.HandlerFunc"},
	{[]string{"echo.Context"}, "echo.HandlerFunc"},
	{[]string{"*fiber.Ctx"}, "fiber.Handler"},
}

// analyzeHTTPHandlers detects structs used in HTTP handlers.
func (a *Analyzer) analyzeHTTPHandlers(structs map[string]*itypes.StructInfo) {
	for _, file := range a.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			// Check if this function matches an HTTP handler signature
			if a.isHTTPHandler(funcDecl) {
				a.analyzeHandlerBody(funcDecl, structs)
			}

			return true
		})
	}
}

// isHTTPHandler checks if a function declaration matches an HTTP handler signature.
func (a *Analyzer) isHTTPHandler(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil {
		return false
	}

	params := funcDecl.Type.Params.List
	if len(params) == 0 {
		return false
	}

	// Get parameter types
	var paramTypes []string
	for _, param := range params {
		tv, ok := a.pass.TypesInfo.Types[param.Type]
		if !ok {
			continue
		}
		paramTypes = append(paramTypes, tv.Type.String())
	}

	// Check against known handler signatures
	for _, sig := range httpHandlerSignatures {
		if matchesSignature(paramTypes, sig.paramTypes) {
			return true
		}
	}

	// Also check for ServeHTTP method
	if funcDecl.Recv != nil && funcDecl.Name.Name == "ServeHTTP" {
		if len(params) >= 2 {
			return true
		}
	}

	return false
}

// matchesSignature checks if parameter types match a signature.
func matchesSignature(actual, expected []string) bool {
	if len(actual) < len(expected) {
		return false
	}

	for i, exp := range expected {
		if !strings.HasSuffix(actual[i], exp) && actual[i] != exp {
			return false
		}
	}
	return true
}

// analyzeHandlerBody finds structs accessed in an HTTP handler.
func (a *Analyzer) analyzeHandlerBody(funcDecl *ast.FuncDecl, structs map[string]*itypes.StructInfo) {
	// If this is a method, the receiver is likely a hot struct
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		recvType := funcDecl.Recv.List[0].Type
		a.markStructAsHandler(recvType, funcDecl.Pos(), structs, "HTTP handler receiver")
	}

	// Walk the handler body for struct accesses
	if funcDecl.Body == nil {
		return
	}

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.SelectorExpr:
			a.markStructAsHandler(expr.X, funcDecl.Pos(), structs, "accessed in HTTP handler")
		}
		return true
	})
}

// markStructAsHandler marks a struct as used in an HTTP handler.
func (a *Analyzer) markStructAsHandler(expr ast.Expr, handlerPos token.Pos, structs map[string]*itypes.StructInfo, details string) {
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
			if !hasHotPathType(info, itypes.HotPathHTTPHandler) {
				info.AddHotPath(itypes.HotPathSource{
					Type:     itypes.HotPathHTTPHandler,
					Score:    itypes.HotPathHTTPHandler.Score(),
					Location: handlerPos,
					Details:  details,
				})
			}
		}
	}
}
