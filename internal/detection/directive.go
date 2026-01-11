package detection

import (
	"go/ast"
	"strings"
)

// HasDirective checks if a struct has the //go:cachepad directive.
func HasDirective(file *ast.File, genDecl *ast.GenDecl, typeSpec *ast.TypeSpec) bool {
	// Check genDecl doc comments
	if genDecl.Doc != nil {
		for _, c := range genDecl.Doc.List {
			if strings.Contains(c.Text, "go:cachepad") {
				return true
			}
		}
	}

	// Check typeSpec doc comments
	if typeSpec.Doc != nil {
		for _, c := range typeSpec.Doc.List {
			if strings.Contains(c.Text, "go:cachepad") {
				return true
			}
		}
	}

	// Check comments in the file near the position
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "go:cachepad") {
				// Check if comment is near the type declaration
				if c.End()+2 >= typeSpec.Pos() && c.Pos() <= typeSpec.Pos() {
					return true
				}
			}
		}
	}

	return false
}
