// Package detection provides struct detection capabilities.
package detection

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// Detector discovers and classifies structs in a package.
type Detector struct {
	pass           *analysis.Pass
	ignorePatterns []string
}

// New creates a new Detector.
func New(pass *analysis.Pass, ignorePatterns []string) *Detector {
	return &Detector{
		pass:           pass,
		ignorePatterns: ignorePatterns,
	}
}

// DiscoverStructs finds all struct definitions in the package.
func (d *Detector) DiscoverStructs() map[string]*types.StructInfo {
	structs := make(map[string]*types.StructInfo)

	for _, file := range d.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				obj := d.pass.TypesInfo.Defs[typeSpec.Name]
				if obj == nil {
					continue
				}
				named, ok := obj.Type().(*gotypes.Named)
				if !ok {
					continue
				}
				typesStruct, ok := named.Underlying().(*gotypes.Struct)
				if !ok {
					continue
				}

				name := typeSpec.Name.Name
				if matchesIgnorePattern(name, d.ignorePatterns) {
					continue
				}

				info := &types.StructInfo{
					Name:       name,
					TypeSpec:   typeSpec,
					StructType: typesStruct,
					ASTStruct:  structType,
					Pos:        typeSpec.Pos(),
					File:       file,
					PkgPath:    d.pass.Pkg.Path(),
				}

				// Check for directive
				if HasDirective(file, genDecl, typeSpec) {
					info.AddHotPath(types.HotPathSource{
						Type:    types.HotPathDirective,
						Score:   types.HotPathDirective.Score(),
						Details: "//go:cachepad",
					})
				}

				// Check for mutex fields
				if HasMutexField(typesStruct) {
					info.AddHotPath(types.HotPathSource{
						Type:    types.HotPathMutex,
						Score:   types.HotPathMutex.Score(),
						Details: "sync.Mutex field",
					})
				}

				// Check for atomic fields
				if HasAtomicField(typesStruct) {
					info.AddHotPath(types.HotPathSource{
						Type:    types.HotPathAtomic,
						Score:   types.HotPathAtomic.Score(),
						Details: "atomic.* field",
					})
				}

				structs[name] = info
			}
			return true
		})
	}

	return structs
}
