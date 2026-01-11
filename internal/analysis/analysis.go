// Package analysis provides a framework for pluggable struct analyzers.
package analysis

import (
	"go/token"

	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis"
)

// Severity represents the severity of a finding.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// Finding represents an issue found by an analyzer.
type Finding struct {
	Pos        token.Pos
	End        token.Pos
	Message    string
	Severity   Severity
	Suggestion string
	FixFunc    func() *analysis.SuggestedFix
}

// Context provides shared context for analyzers.
type Context struct {
	Pass          *analysis.Pass
	Structs       map[string]*itypes.StructInfo
	CacheLineSize int64
}

// Analyzer is the interface that all analyzers must implement.
type Analyzer interface {
	// Name returns the analyzer name.
	Name() string

	// Description returns a short description of what the analyzer detects.
	Description() string

	// Analyze runs the analysis and returns findings.
	Analyze(ctx *Context) []Finding
}

// Chain runs multiple analyzers in sequence.
type Chain struct {
	analyzers []Analyzer
}

// NewChain creates a new analyzer chain.
func NewChain(analyzers ...Analyzer) *Chain {
	return &Chain{analyzers: analyzers}
}

// Add adds an analyzer to the chain.
func (c *Chain) Add(a Analyzer) {
	c.analyzers = append(c.analyzers, a)
}

// Run executes all analyzers and collects findings.
func (c *Chain) Run(ctx *Context) []Finding {
	var findings []Finding
	for _, a := range c.analyzers {
		findings = append(findings, a.Analyze(ctx)...)
	}
	return findings
}

// Analyzers returns the list of registered analyzers.
func (c *Chain) Analyzers() []Analyzer {
	return c.analyzers
}
