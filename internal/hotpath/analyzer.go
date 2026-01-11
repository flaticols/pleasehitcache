// Package hotpath provides intelligent hot path detection via AST analysis.
package hotpath

import (
	"os"

	"github.com/flaticols/pleasehitcache/internal/types"
	"github.com/google/pprof/profile"
	"golang.org/x/tools/go/analysis"
)

// Analyzer detects hot paths through various analysis methods.
type Analyzer struct {
	pass         *analysis.Pass
	pprofPath    string
	hotFunctions map[string]bool
}

// New creates a new hot path Analyzer.
func New(pass *analysis.Pass, pprofPath string) *Analyzer {
	a := &Analyzer{
		pass:      pass,
		pprofPath: pprofPath,
	}
	a.hotFunctions = a.loadPprofHotPaths()
	return a
}

// Analyze runs all hot path detection on the given structs.
func (a *Analyzer) Analyze(structs map[string]*types.StructInfo) {
	// Phase 1: pprof data (if available)
	if a.hotFunctions != nil {
		a.analyzePprofHotPaths(structs)
	}

	// Phase 2: Benchmark functions
	a.analyzeBenchmarks(structs)

	// Phase 3: HTTP handlers
	a.analyzeHTTPHandlers(structs)

	// Phase 4: Loops (nested and simple)
	a.analyzeLoops(structs)

	// Phase 5: Goroutine access
	a.analyzeGoroutines(structs)

	// Phase 6: Channel operations
	a.analyzeChannels(structs)
}

// loadPprofHotPaths loads hot functions from a pprof profile.
func (a *Analyzer) loadPprofHotPaths() map[string]bool {
	if a.pprofPath == "" {
		return nil
	}

	f, err := os.Open(a.pprofPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	prof, err := profile.Parse(f)
	if err != nil {
		return nil
	}

	hotFunctions := make(map[string]bool)
	for _, sample := range prof.Sample {
		if len(sample.Value) > 0 && sample.Value[0] > 0 {
			for _, loc := range sample.Location {
				for _, line := range loc.Line {
					if line.Function != nil {
						hotFunctions[line.Function.Name] = true
					}
				}
			}
		}
	}
	return hotFunctions
}

// analyzePprofHotPaths marks structs used in pprof hot functions.
func (a *Analyzer) analyzePprofHotPaths(structs map[string]*types.StructInfo) {
	for _, info := range structs {
		fullName := info.PkgPath + "." + info.Name
		if a.hotFunctions[fullName] {
			info.AddHotPath(types.HotPathSource{
				Type:    types.HotPathPprof,
				Score:   types.HotPathPprof.Score(),
				Details: "pprof hot function",
			})
		}
	}
}
