// Package antipattern detects usage patterns that make padding counterproductive.
package antipattern

import (
	"go/ast"
	"go/types"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// Detector finds anti-patterns in struct usage.
type Detector struct {
	pass          *analysis.Pass
	cacheLineSize int64
}

// New creates a new anti-pattern Detector.
func New(pass *analysis.Pass, cacheLineSize int64) *Detector {
	return &Detector{
		pass:          pass,
		cacheLineSize: cacheLineSize,
	}
}

// Analyze runs all anti-pattern detection.
func (d *Detector) Analyze(structs map[string]*itypes.StructInfo) {
	// Check for slice element usage
	d.findSliceElements(structs)

	// Check for embedded structs
	d.findEmbedded(structs)

	// Check for structs exceeding cache line
	d.checkExceedsCacheLine(structs)
}

// findSliceElements finds structs used as slice elements.
func (d *Detector) findSliceElements(structs map[string]*itypes.StructInfo) {
	for _, file := range d.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ArrayType:
				// Check for slice (no length = slice, with length = array)
				if node.Len == nil {
					d.checkSliceElement(node, structs)
				}
			case *ast.Field:
				// Also check field types
				d.checkFieldForSlice(node, structs)
			}
			return true
		})
	}
}

// checkSliceElement marks a struct as a slice element.
func (d *Detector) checkSliceElement(arr *ast.ArrayType, structs map[string]*itypes.StructInfo) {
	tv, ok := d.pass.TypesInfo.Types[arr.Elt]
	if !ok {
		return
	}

	elemType := tv.Type
	if named, ok := elemType.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			info.AntiPatterns = append(info.AntiPatterns, itypes.AntiPattern{
				Type:     itypes.AntiPatternSliceElement,
				Location: arr.Pos(),
				Details:  "used as []" + name,
			})
		}
	}
}

// checkFieldForSlice checks a field type for slice usage.
func (d *Detector) checkFieldForSlice(field *ast.Field, structs map[string]*itypes.StructInfo) {
	tv, ok := d.pass.TypesInfo.Types[field.Type]
	if !ok {
		return
	}

	if slice, ok := tv.Type.Underlying().(*types.Slice); ok {
		elemType := slice.Elem()
		if named, ok := elemType.(*types.Named); ok {
			name := named.Obj().Name()
			if info, exists := structs[name]; exists {
				info.AntiPatterns = append(info.AntiPatterns, itypes.AntiPattern{
					Type:     itypes.AntiPatternSliceElement,
					Location: field.Pos(),
					Details:  "used as []" + name + " in field",
				})
			}
		}
	}
}

// findEmbedded finds structs that are embedded in other structs.
func (d *Detector) findEmbedded(structs map[string]*itypes.StructInfo) {
	for _, file := range d.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok {
				return true
			}

			// Embedded fields have no names
			if len(field.Names) > 0 {
				return true
			}

			tv, ok := d.pass.TypesInfo.Types[field.Type]
			if !ok {
				return true
			}

			fieldType := tv.Type
			if ptr, ok := fieldType.(*types.Pointer); ok {
				fieldType = ptr.Elem()
			}

			if named, ok := fieldType.(*types.Named); ok {
				name := named.Obj().Name()
				if info, exists := structs[name]; exists {
					info.AntiPatterns = append(info.AntiPatterns, itypes.AntiPattern{
						Type:     itypes.AntiPatternEmbedded,
						Location: field.Pos(),
						Details:  "embedded in another struct",
					})
				}
			}
			return true
		})
	}
}

// checkExceedsCacheLine marks structs that exceed cache line size.
func (d *Detector) checkExceedsCacheLine(structs map[string]*itypes.StructInfo) {
	for _, info := range structs {
		if info.Size > d.cacheLineSize {
			info.AntiPatterns = append(info.AntiPatterns, itypes.AntiPattern{
				Type:    itypes.AntiPatternExceedsCacheLine,
				Details: "struct size exceeds cache line",
			})
		}
	}
}
