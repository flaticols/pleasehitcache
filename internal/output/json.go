package output

import (
	"encoding/json"
	"fmt"
	"os"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// JSONIssue represents a single issue in JSON format for CI integration.
type JSONIssue struct {
	FromLinter   string          `json:"FromLinter"`
	Text         string          `json:"Text"`
	Severity     string          `json:"Severity"`
	Pos          JSONPos         `json:"Pos"`
	Struct       string          `json:"Struct"`
	Size         int64           `json:"Size"`
	CacheLine    int64           `json:"CacheLine"`
	Action       string          `json:"Action"`
	Padding      int64           `json:"PaddingNeeded,omitempty"`
	Score        int             `json:"HotPathScore"`
	Detections   []string        `json:"Detections,omitempty"`
	AntiPatterns []string        `json:"AntiPatterns,omitempty"`
	Fixable      bool            `json:"Fixable"`
}

// JSONPos represents file position in JSON format.
type JSONPos struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// PrintJSON outputs a struct analysis in JSON format to stderr.
func PrintJSON(pass *analysis.Pass, info *itypes.StructInfo, rec *itypes.Recommendation) {
	pos := pass.Fset.Position(info.Pos)

	// Collect detection sources
	var detections []string
	for _, hp := range info.HotPathSources {
		detections = append(detections, hp.Type.String())
	}

	// Collect anti-patterns
	var antiPatterns []string
	for _, ap := range info.AntiPatterns {
		antiPatterns = append(antiPatterns, ap.Type.String())
	}

	severity := "warning"
	if rec.Action == itypes.ActionNoPad {
		severity = "info"
	}

	issue := JSONIssue{
		FromLinter:   "cachepad",
		Text:         FormatShort(info, rec),
		Severity:     severity,
		Pos: JSONPos{
			Filename: pos.Filename,
			Line:     pos.Line,
			Column:   pos.Column,
		},
		Struct:       info.Name,
		Size:         rec.CurrentSize,
		CacheLine:    rec.CacheLineSize,
		Action:       rec.Action.String(),
		Padding:      rec.PaddingNeeded,
		Score:        info.TotalScore,
		Detections:   detections,
		AntiPatterns: antiPatterns,
		Fixable:      rec.Action == itypes.ActionPad,
	}

	data, err := json.Marshal(issue)
	if err != nil {
		return
	}
	fmt.Fprintln(os.Stderr, string(data))
}
