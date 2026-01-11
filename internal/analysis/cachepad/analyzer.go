// Package cachepad analyzes structs for cache line padding opportunities.
package cachepad

import (
	"fmt"
	"strings"

	"github.com/flaticols/pleasehitcache/internal/analysis"
	"github.com/flaticols/pleasehitcache/internal/layout"
	"github.com/flaticols/pleasehitcache/internal/recommendation"
	itypes "github.com/flaticols/pleasehitcache/internal/types"
	xanalysis "golang.org/x/tools/go/analysis"
)

// Analyzer detects structs that would benefit from cache line padding.
type Analyzer struct {
	threshold  int
	analyzeAll bool
}

// New creates a new cache pad analyzer.
func New(threshold int, analyzeAll bool) *Analyzer {
	return &Analyzer{
		threshold:  threshold,
		analyzeAll: analyzeAll,
	}
}

// Name returns the analyzer name.
func (a *Analyzer) Name() string {
	return "cachepad"
}

// Description returns a description of the analyzer.
func (a *Analyzer) Description() string {
	return "detects structs that would benefit from cache line padding to prevent false sharing"
}

// Analyze finds structs that need cache line padding.
func (a *Analyzer) Analyze(ctx *analysis.Context) []analysis.Finding {
	var findings []analysis.Finding

	calculator := layout.New(ctx.Pass, ctx.CacheLineSize)
	engine := recommendation.New(calculator, a.threshold)

	for _, info := range ctx.Structs {
		// Calculate layout
		calculator.Calculate(info)

		// Check if we should analyze this struct
		if !engine.ShouldAnalyze(info, a.analyzeAll) {
			continue
		}

		// Generate recommendation
		rec := engine.Generate(info)

		// Create finding
		finding := a.createFinding(ctx, info, rec)
		if finding != nil {
			findings = append(findings, *finding)
		}
	}

	return findings
}

// createFinding creates a finding from a recommendation.
func (a *Analyzer) createFinding(ctx *analysis.Context, info *itypes.StructInfo, rec *itypes.Recommendation) *analysis.Finding {
	var msg strings.Builder

	// Header
	switch rec.Action {
	case itypes.ActionPad:
		msg.WriteString(fmt.Sprintf("struct '%s' should add %d-byte padding for cache line alignment\n",
			info.Name, rec.PaddingNeeded))
	case itypes.ActionWarn:
		msg.WriteString(fmt.Sprintf("struct '%s' could benefit from padding but it would significantly increase size (%d -> %d bytes)\n",
			info.Name, rec.CurrentSize, rec.CacheLineSize))
	case itypes.ActionNoPad:
		msg.WriteString(fmt.Sprintf("struct '%s': %s\n", info.Name, rec.Reason))
	default:
		return nil
	}

	// Hot path info
	msg.WriteString(fmt.Sprintf("    Hot path score: %d/100\n", info.TotalScore))
	if len(info.HotPathSources) > 0 {
		msg.WriteString("    Detected via:\n")
		for _, hp := range info.HotPathSources {
			msg.WriteString(fmt.Sprintf("      - %s (%s)\n", hp.Type.String(), hp.Details))
		}
	}

	// Size info
	msg.WriteString(fmt.Sprintf("    Size: %d bytes, Cache line: %d bytes\n", info.Size, rec.CacheLineSize))

	// Layout
	if len(info.Fields) > 0 {
		msg.WriteString("    Layout:\n")
		for _, f := range info.Fields {
			msg.WriteString(fmt.Sprintf("        %-12s %-22s %d-%3d (%d bytes)\n",
				f.Name, f.Type, f.Offset, f.Offset+f.Size, f.Size))
		}
	}

	// Recommendation
	if rec.Action == itypes.ActionPad {
		msg.WriteString(fmt.Sprintf("    Recommendation: add `_ [%d]byte` padding field", rec.PaddingNeeded))
	}

	severity := analysis.SeverityWarning
	if rec.Action == itypes.ActionNoPad {
		severity = analysis.SeverityInfo
	}

	return &analysis.Finding{
		Pos:      info.Pos,
		End:      info.TypeSpec.End(),
		Message:  msg.String(),
		Severity: severity,
		FixFunc:  a.createFixFunc(info, rec),
	}
}

// createFixFunc creates a function that generates a suggested fix.
func (a *Analyzer) createFixFunc(info *itypes.StructInfo, rec *itypes.Recommendation) func() *xanalysis.SuggestedFix {
	if rec.Action != itypes.ActionPad {
		return nil
	}

	return func() *xanalysis.SuggestedFix {
		if info.ASTStruct == nil || len(info.ASTStruct.Fields.List) == 0 {
			return nil
		}

		lastField := info.ASTStruct.Fields.List[len(info.ASTStruct.Fields.List)-1]
		insertPos := lastField.End()

		paddingText := fmt.Sprintf("\n\t_ [%d]byte // cache line padding (total: %d bytes)",
			rec.PaddingNeeded, rec.CacheLineSize)

		return &xanalysis.SuggestedFix{
			Message: fmt.Sprintf("add %d-byte cache line padding", rec.PaddingNeeded),
			TextEdits: []xanalysis.TextEdit{
				{
					Pos:     insertPos,
					End:     insertPos,
					NewText: []byte(paddingText),
				},
			},
		}
	}
}
