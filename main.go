// Package main provides a Go static analyzer for cache line optimization.
// It detects structs that may benefit from cache line padding to prevent
// false sharing, and warns against inappropriate padding that would waste memory.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/pprof/profile"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

// JSONIssue represents a single issue in JSON format for CI integration.
type JSONIssue struct {
	FromLinter string  `json:"FromLinter"`
	Text       string  `json:"Text"`
	Severity   string  `json:"Severity"`
	Pos        JSONPos `json:"Pos"`
	Struct     string  `json:"Struct"`
	Size       int64   `json:"Size"`
	CacheLine  int64   `json:"CacheLine"`
	Action     string  `json:"Action"`
	Padding    int64   `json:"PaddingNeeded,omitempty"`
	Fixable    bool    `json:"Fixable"`
}

// JSONPos represents file position in JSON format.
type JSONPos struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// FieldInfo holds information about a struct field's layout.
type FieldInfo struct {
	Name      string
	Type      string
	Offset    int64
	Size      int64
	Alignment int64
}

// StructInfo holds analyzed struct information.
type StructInfo struct {
	Name       string
	TypeSpec   *ast.TypeSpec
	StructType *types.Struct
	ASTStruct  *ast.StructType
	Pos        token.Pos
	File       *ast.File

	// Layout
	Size      int64
	Alignment int64
	Fields    []FieldInfo

	// Detection flags
	HasDirective   bool
	HasMutex       bool
	HasAtomicField bool
	InSyncPool     bool
	InHotPath      bool

	// Anti-patterns
	IsSliceElement bool
	IsEmbedded     bool
	SliceUsagePos  token.Pos
	EmbeddedPos    token.Pos
}

// ActionType represents the recommended action.
type ActionType int

const (
	ActionPad ActionType = iota
	ActionWarn
	ActionNoPad
)

// Recommendation holds the analysis result for a struct.
type Recommendation struct {
	Action        ActionType
	Reason        string
	CurrentSize   int64
	OptimalSize   int64
	PaddingNeeded int64
}

// Analyzer is the main analysis.Analyzer for cache line optimization.
var Analyzer = &analysis.Analyzer{
	Name: "cachepad",
	Doc: `detect structs that may benefit from cache line padding

This analyzer identifies structs where cache line padding could improve
performance by preventing false sharing in concurrent code. It also warns
when padding would be counterproductive (e.g., for slice elements).

Detection methods:
  - //go:cachepad directive before struct definition
  - Heuristics: sync.Mutex fields, atomic.* types, sync.Pool usage
  - pprof profiling data (via -pprof flag)

Use -cache-line-size to specify target architecture cache line size.`,
	Run: run,
}

// Flags
var (
	cacheLineSizeFlag string
	pprofPath         string
	allStructs        bool
	jsonOutput        bool
	ignorePatterns    string
)

func init() {
	Analyzer.Flags.StringVar(&cacheLineSizeFlag, "cache-line-size", "auto",
		"cache line size: 'auto' (detect from GOARCH), '64', or '128'")
	Analyzer.Flags.StringVar(&pprofPath, "pprof", "",
		"path to CPU/memory profile for hot path detection")
	Analyzer.Flags.BoolVar(&allStructs, "all", false,
		"analyze all structs, not just those with heuristic detection or directive")
	Analyzer.Flags.BoolVar(&jsonOutput, "json", false,
		"output in JSON format for CI integration")
	Analyzer.Flags.StringVar(&ignorePatterns, "ignore", "",
		"comma-separated patterns to ignore")
}

// New returns the analyzer for golangci-lint module plugin integration.
func New(conf any) ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

func main() {
	singlechecker.Main(Analyzer)
}

func run(pass *analysis.Pass) (any, error) {
	cacheLineSize := getCacheLineSize()
	hotFunctions := loadHotPaths()

	// Phase 1: Discover all structs
	structs := discoverStructs(pass, hotFunctions)

	// Phase 2: Analyze usage patterns
	analyzeUsagePatterns(pass, structs)

	// Phase 3: Calculate layouts and generate recommendations
	for _, info := range structs {
		if !shouldAnalyze(info) {
			continue
		}

		calculateLayout(pass, info, cacheLineSize)
		rec := generateRecommendation(info, cacheLineSize)
		reportDiagnostic(pass, info, rec, cacheLineSize)
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

func loadHotPaths() map[string]bool {
	if pprofPath == "" {
		return nil
	}

	f, err := os.Open(pprofPath)
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

func discoverStructs(pass *analysis.Pass, hotFunctions map[string]bool) map[string]*StructInfo {
	structs := make(map[string]*StructInfo)

	for _, file := range pass.Files {
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

				// Get types.Struct
				obj := pass.TypesInfo.Defs[typeSpec.Name]
				if obj == nil {
					continue
				}
				named, ok := obj.Type().(*types.Named)
				if !ok {
					continue
				}
				typesStruct, ok := named.Underlying().(*types.Struct)
				if !ok {
					continue
				}

				name := typeSpec.Name.Name
				if matchesIgnorePattern(name) {
					continue
				}

				info := &StructInfo{
					Name:       name,
					TypeSpec:   typeSpec,
					StructType: typesStruct,
					ASTStruct:  structType,
					Pos:        typeSpec.Pos(),
					File:       file,
				}

				// Check for directive
				info.HasDirective = hasDirective(file, genDecl, typeSpec)

				// Check for mutex/atomic fields
				info.HasMutex = hasMutexField(typesStruct)
				info.HasAtomicField = hasAtomicField(typesStruct)

				// Check if in hot path (from pprof)
				if hotFunctions != nil {
					pkgPath := pass.Pkg.Path()
					fullName := pkgPath + "." + name
					info.InHotPath = hotFunctions[fullName]
				}

				structs[name] = info
			}
			return true
		})
	}

	return structs
}

func hasDirective(file *ast.File, genDecl *ast.GenDecl, typeSpec *ast.TypeSpec) bool {
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

func hasMutexField(s *types.Struct) bool {
	for i := range s.NumFields() {
		field := s.Field(i)
		typeName := field.Type().String()
		if typeName == "sync.Mutex" || typeName == "sync.RWMutex" {
			return true
		}
	}
	return false
}

func hasAtomicField(s *types.Struct) bool {
	atomicTypes := map[string]bool{
		"sync/atomic.Int32":   true,
		"sync/atomic.Int64":   true,
		"sync/atomic.Uint32":  true,
		"sync/atomic.Uint64":  true,
		"sync/atomic.Bool":    true,
		"sync/atomic.Pointer": true,
		"sync/atomic.Value":   true,
		"atomic.Int32":        true,
		"atomic.Int64":        true,
		"atomic.Uint32":       true,
		"atomic.Uint64":       true,
		"atomic.Bool":         true,
		"atomic.Pointer":      true,
		"atomic.Value":        true,
	}
	for i := range s.NumFields() {
		field := s.Field(i)
		typeName := field.Type().String()
		if atomicTypes[typeName] {
			return true
		}
		// Check for pointer to atomic types
		if ptr, ok := field.Type().(*types.Pointer); ok {
			if atomicTypes[ptr.Elem().String()] {
				return true
			}
		}
	}
	return false
}

func analyzeUsagePatterns(pass *analysis.Pass, structs map[string]*StructInfo) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				// Check for sync.Pool.Put calls
				checkPoolPut(pass, node, structs)

			case *ast.ArrayType:
				// Check for slice types
				if node.Len == nil { // It's a slice, not array
					checkSliceElement(pass, node, structs)
				}

			case *ast.Field:
				// Check for embedded structs
				checkEmbedded(pass, node, structs)
			}
			return true
		})
	}
}

func checkPoolPut(pass *analysis.Pass, call *ast.CallExpr, structs map[string]*StructInfo) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Put" {
		return
	}

	// Check if receiver is sync.Pool
	tv, ok := pass.TypesInfo.Types[sel.X]
	if !ok {
		return
	}

	typeName := tv.Type.String()
	if !strings.Contains(typeName, "sync.Pool") {
		return
	}

	if len(call.Args) == 0 {
		return
	}

	// Get the type being stored
	argType := pass.TypesInfo.Types[call.Args[0]].Type
	if argType == nil {
		return
	}

	// Unwrap pointer
	if ptr, ok := argType.(*types.Pointer); ok {
		argType = ptr.Elem()
	}

	// Find the struct
	if named, ok := argType.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			info.InSyncPool = true
		}
	}
}

func checkSliceElement(pass *analysis.Pass, arr *ast.ArrayType, structs map[string]*StructInfo) {
	tv, ok := pass.TypesInfo.Types[arr.Elt]
	if !ok {
		return
	}

	elemType := tv.Type
	if named, ok := elemType.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			info.IsSliceElement = true
			info.SliceUsagePos = arr.Pos()
		}
	}
}

func checkEmbedded(pass *analysis.Pass, field *ast.Field, structs map[string]*StructInfo) {
	// Embedded fields have no names
	if len(field.Names) > 0 {
		return
	}

	tv, ok := pass.TypesInfo.Types[field.Type]
	if !ok {
		return
	}

	fieldType := tv.Type
	if ptr, ok := fieldType.(*types.Pointer); ok {
		fieldType = ptr.Elem()
	}

	if named, ok := fieldType.(*types.Named); ok {
		name := named.Obj().Name()
		if info, exists := structs[name]; exists {
			info.IsEmbedded = true
			info.EmbeddedPos = field.Pos()
		}
	}
}

func shouldAnalyze(info *StructInfo) bool {
	if allStructs {
		return true
	}
	return info.HasDirective || info.HasMutex || info.HasAtomicField ||
		info.InSyncPool || info.InHotPath
}

func calculateLayout(pass *analysis.Pass, info *StructInfo, cacheLineSize int64) {
	sizes := pass.TypesSizes

	info.Size = sizes.Sizeof(info.StructType)
	info.Alignment = sizes.Alignof(info.StructType)

	// Calculate field offsets
	numFields := info.StructType.NumFields()
	if numFields == 0 {
		return
	}

	fields := make([]*types.Var, numFields)
	for i := range numFields {
		fields[i] = info.StructType.Field(i)
	}
	offsets := sizes.Offsetsof(fields)

	info.Fields = make([]FieldInfo, numFields)
	for i, field := range fields {
		info.Fields[i] = FieldInfo{
			Name:      field.Name(),
			Type:      field.Type().String(),
			Offset:    offsets[i],
			Size:      sizes.Sizeof(field.Type()),
			Alignment: sizes.Alignof(field.Type()),
		}
	}
}

func generateRecommendation(info *StructInfo, cacheLineSize int64) Recommendation {
	// Anti-pattern: slice element
	if info.IsSliceElement {
		return Recommendation{
			Action:      ActionNoPad,
			Reason:      "struct is used as slice element; padding would increase total memory usage",
			CurrentSize: info.Size,
		}
	}

	// Anti-pattern: embedded struct
	if info.IsEmbedded {
		return Recommendation{
			Action:      ActionNoPad,
			Reason:      "struct is embedded in another struct; padding may cause unintended layout changes",
			CurrentSize: info.Size,
		}
	}

	// Already exceeds cache line
	if info.Size > cacheLineSize {
		return Recommendation{
			Action:      ActionNoPad,
			Reason:      fmt.Sprintf("struct size (%d bytes) already exceeds cache line (%d bytes)", info.Size, cacheLineSize),
			CurrentSize: info.Size,
		}
	}

	// Already exactly cache line size
	if info.Size == cacheLineSize {
		return Recommendation{
			Action:      ActionNoPad,
			Reason:      "struct already matches cache line size",
			CurrentSize: info.Size,
		}
	}

	// Calculate padding needed
	paddingNeeded := cacheLineSize - info.Size

	// Check if padding would more than double the size
	if paddingNeeded > info.Size {
		return Recommendation{
			Action:        ActionWarn,
			Reason:        fmt.Sprintf("padding would add %d bytes (>100%% increase)", paddingNeeded),
			CurrentSize:   info.Size,
			OptimalSize:   cacheLineSize,
			PaddingNeeded: paddingNeeded,
		}
	}

	// Padding is reasonable
	return Recommendation{
		Action:        ActionPad,
		Reason:        "false sharing risk detected; padding fits within cache line",
		CurrentSize:   info.Size,
		OptimalSize:   cacheLineSize,
		PaddingNeeded: paddingNeeded,
	}
}

func reportDiagnostic(pass *analysis.Pass, info *StructInfo, rec Recommendation, cacheLineSize int64) {
	pos := pass.Fset.Position(info.Pos)

	var message string
	var severity string

	switch rec.Action {
	case ActionPad:
		message = fmt.Sprintf("struct '%s' should add %d-byte padding for cache line alignment",
			info.Name, rec.PaddingNeeded)
		severity = "warning"
	case ActionWarn:
		message = fmt.Sprintf("struct '%s' could benefit from padding but it would significantly increase size (%d -> %d bytes)",
			info.Name, rec.CurrentSize, rec.OptimalSize)
		severity = "info"
	case ActionNoPad:
		message = fmt.Sprintf("struct '%s' - padding NOT recommended: %s",
			info.Name, rec.Reason)
		severity = "info"
	}

	// Build detection reason
	var detections []string
	if info.HasDirective {
		detections = append(detections, "directive")
	}
	if info.HasMutex {
		detections = append(detections, "sync.Mutex")
	}
	if info.HasAtomicField {
		detections = append(detections, "atomic field")
	}
	if info.InSyncPool {
		detections = append(detections, "sync.Pool")
	}
	if info.InHotPath {
		detections = append(detections, "pprof hot path")
	}

	if len(detections) > 0 {
		message += fmt.Sprintf(" [detected via: %s]", strings.Join(detections, ", "))
	}

	if jsonOutput {
		action := "none"
		switch rec.Action {
		case ActionPad:
			action = "pad"
		case ActionWarn:
			action = "warn"
		case ActionNoPad:
			action = "no_pad"
		}

		issue := JSONIssue{
			FromLinter: "cachepad",
			Text:       message,
			Severity:   severity,
			Pos: JSONPos{
				Filename: pos.Filename,
				Line:     pos.Line,
				Column:   pos.Column,
			},
			Struct:    info.Name,
			Size:      rec.CurrentSize,
			CacheLine: cacheLineSize,
			Action:    action,
			Padding:   rec.PaddingNeeded,
			Fixable:   rec.Action == ActionPad,
		}
		printJSONIssue(issue)
	}

	// Build detailed message with layout
	var detailedMsg strings.Builder
	detailedMsg.WriteString(message)
	detailedMsg.WriteString(fmt.Sprintf("\n    Size: %d bytes, Cache line: %d bytes", info.Size, cacheLineSize))

	if len(info.Fields) > 0 {
		detailedMsg.WriteString("\n    Layout:")
		for _, f := range info.Fields {
			detailedMsg.WriteString(fmt.Sprintf("\n        %-12s %-20s %3d-%3d (%d bytes)",
				f.Name, f.Type, f.Offset, f.Offset+f.Size, f.Size))
		}
	}

	if rec.Action == ActionPad {
		detailedMsg.WriteString(fmt.Sprintf("\n    Fix: add `_ [%d]byte` padding field", rec.PaddingNeeded))
	}

	diagnostic := analysis.Diagnostic{
		Pos:     info.Pos,
		End:     info.TypeSpec.End(),
		Message: detailedMsg.String(),
	}

	// Add suggested fix for padding
	if rec.Action == ActionPad && info.ASTStruct != nil && len(info.ASTStruct.Fields.List) > 0 {
		fix := createPaddingFix(pass, info, rec.PaddingNeeded, cacheLineSize)
		if fix != nil {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{*fix}
		}
	}

	pass.Report(diagnostic)
}

func createPaddingFix(pass *analysis.Pass, info *StructInfo, paddingNeeded, cacheLineSize int64) *analysis.SuggestedFix {
	if info.ASTStruct == nil || len(info.ASTStruct.Fields.List) == 0 {
		return nil
	}

	// Find position after last field
	lastField := info.ASTStruct.Fields.List[len(info.ASTStruct.Fields.List)-1]
	insertPos := lastField.End()

	paddingText := fmt.Sprintf("\n\t_ [%d]byte // cache line padding (total: %d bytes)", paddingNeeded, cacheLineSize)

	return &analysis.SuggestedFix{
		Message: fmt.Sprintf("add %d-byte cache line padding", paddingNeeded),
		TextEdits: []analysis.TextEdit{
			{
				Pos:     insertPos,
				End:     insertPos,
				NewText: []byte(paddingText),
			},
		},
	}
}

func matchesIgnorePattern(name string) bool {
	if ignorePatterns == "" {
		return false
	}

	patterns := strings.Split(ignorePatterns, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}

func printJSONIssue(issue JSONIssue) {
	data, err := json.Marshal(issue)
	if err != nil {
		return
	}
	fmt.Fprintln(os.Stderr, string(data))
}
