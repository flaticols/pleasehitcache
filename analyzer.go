package main

import (
	"os"
	"runtime"
	"strconv"

	"github.com/flaticols/pleasehitcache/internal/antipattern"
	"github.com/flaticols/pleasehitcache/internal/detection"
	"github.com/flaticols/pleasehitcache/internal/hotpath"
	"github.com/flaticols/pleasehitcache/internal/layout"
	"github.com/flaticols/pleasehitcache/internal/output"
	"github.com/flaticols/pleasehitcache/internal/recommendation"
	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

const (
	outputText = "text"
	outputJSON = "json"
)

// Configuration flags
var (
	cacheLineSizeFlag string
	pprofPath         string
	analyzeAll        bool
	outputFormat      string
	ignorePatterns    string
)

// HotPathThreshold is the minimum score for a struct to be analyzed.
const HotPathThreshold = 50

// Analyzer is the main analysis.Analyzer for cache line optimization.
var Analyzer = &analysis.Analyzer{
	Name: "cachepad",
	Doc: `detect structs that may benefit from cache line padding

This analyzer identifies structs where cache line padding could improve
performance by preventing false sharing in concurrent code. It uses
intelligent hot path detection to find performance-critical structs.

Detection methods (in priority order):
  1. //go:cachepad directive - explicit marking
  2. pprof profiling data - runtime hot paths
  3. Benchmark functions - explicit performance tests
  4. HTTP handlers - request-per-second paths
  5. Loop access patterns - tight loops
  6. Goroutine access - concurrent access
  7. Channel operations - often in hot paths
  8. sync.Pool usage - allocation optimization
  9. sync.Mutex/atomic fields - concurrent access likely

Anti-patterns detected (padding NOT recommended):
  - Slice elements: padding multiplies memory usage
  - Embedded structs: padding breaks layout
  - Exceeds cache line: already too large

Use -cache-line-size to specify target architecture cache line size.
Use -pprof to provide runtime profiling data for better detection.`,
	Run: run,
}

func init() {
	Analyzer.Flags.StringVar(&cacheLineSizeFlag, "cache-line-size", "auto",
		"cache line size: 'auto' (detect from GOARCH), '64', or '128'")
	Analyzer.Flags.StringVar(&pprofPath, "pprof", "",
		"path to CPU/memory profile for hot path detection")
	Analyzer.Flags.BoolVar(&analyzeAll, "analyze-all", false,
		"analyze all structs, not just those with heuristic detection or directive")
	Analyzer.Flags.StringVar(&outputFormat, "output", outputText,
		"output format: 'text' (default) or 'json' (golangci-lint compatible)")
	Analyzer.Flags.StringVar(&ignorePatterns, "ignore", "",
		"comma-separated patterns to ignore")
}

// New returns the analyzer for golangci-lint module plugin integration.
func New(conf any) ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

func run(pass *analysis.Pass) (any, error) {
	cacheLineSize := getCacheLineSize()
	ignoreList := detection.ParseIgnorePatterns(ignorePatterns)

	// Phase 1: Discover structs
	detector := detection.New(pass, ignoreList)
	structs := detector.DiscoverStructs()

	// Phase 2: Detect sync.Pool usage
	detection.FindPoolStoredTypes(pass, structs)

	// Phase 3: Hot path analysis (AST-based + optional pprof)
	hotpathAnalyzer := hotpath.New(pass, pprofPath)
	hotpathAnalyzer.Analyze(structs)

	// Phase 4: Anti-pattern detection
	antipatternDetector := antipattern.New(pass, cacheLineSize)
	antipatternDetector.Analyze(structs)

	// Phase 5: Calculate layouts
	calculator := layout.New(pass, cacheLineSize)
	for _, info := range structs {
		calculator.Calculate(info)
	}

	// Phase 6: Generate recommendations and report
	engine := recommendation.New(calculator, HotPathThreshold)

	for _, info := range structs {
		if !engine.ShouldAnalyze(info, analyzeAll) {
			continue
		}

		rec := engine.Generate(info)
		reportDiagnostic(pass, info, rec)
	}

	return nil, nil
}

func getCacheLineSize() int64 {
	if cacheLineSizeFlag != "auto" {
		size, err := strconv.ParseInt(cacheLineSizeFlag, 10, 64)
		if err == nil {
			return size
		}
	}

	goarch := os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}

	// Apple M-series has 128-byte cache lines
	if goarch == "arm64" && goos == "darwin" {
		return 128
	}
	// Most other architectures use 64 bytes
	return 64
}

func reportDiagnostic(pass *analysis.Pass, info *itypes.StructInfo, rec *itypes.Recommendation) {
	// JSON output
	if outputFormat == outputJSON {
		output.PrintJSON(pass, info, rec)
	}

	// Create diagnostic
	message := output.FormatText(info, rec)
	diagnostic := analysis.Diagnostic{
		Pos:     info.Pos,
		End:     info.TypeSpec.End(),
		Message: message,
	}

	// Add suggested fix
	if fix := output.CreatePaddingFix(info, rec); fix != nil {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{*fix}
	}

	pass.Report(diagnostic)
}
