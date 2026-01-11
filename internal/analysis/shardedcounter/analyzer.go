// Package shardedcounter detects atomic counters that would benefit from sharding.
package shardedcounter

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/flaticols/pleasehitcache/internal/analysis"
	itypes "github.com/flaticols/pleasehitcache/internal/types"
)

// Analyzer detects atomic counters accessed from multiple goroutines.
type Analyzer struct{}

// New creates a new sharded counter analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Name returns the analyzer name.
func (a *Analyzer) Name() string {
	return "shardedcounter"
}

// Description returns a description of the analyzer.
func (a *Analyzer) Description() string {
	return "detects atomic counters that would benefit from sharding for high-contention scenarios"
}

// atomicCounterInfo tracks information about an atomic counter field.
type atomicCounterInfo struct {
	structInfo *itypes.StructInfo
	fieldName  string
	fieldType  string
	goroutines int // number of goroutines accessing this counter
	inLoop     bool
	locations  []string // descriptions of access locations
}

// Analyze finds atomic counters that could benefit from sharding.
func (a *Analyzer) Analyze(ctx *analysis.Context) []analysis.Finding {
	var findings []analysis.Finding

	// Track atomic counter access patterns
	counters := make(map[string]*atomicCounterInfo)

	// Find all atomic counter fields in structs
	for _, info := range ctx.Structs {
		for _, field := range info.Fields {
			if isAtomicCounter(field.Type) {
				key := fmt.Sprintf("%s.%s", info.Name, field.Name)
				counters[key] = &atomicCounterInfo{
					structInfo: info,
					fieldName:  field.Name,
					fieldType:  field.Type,
				}
			}
		}
	}

	if len(counters) == 0 {
		return nil
	}

	// Walk AST to find access patterns
	for _, file := range ctx.Pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GoStmt:
				// Check for atomic access inside goroutines
				a.checkGoroutineAccess(ctx, node, counters)

			case *ast.ForStmt, *ast.RangeStmt:
				// Check for atomic access inside loops
				a.checkLoopAccess(ctx, node, counters)
			}
			return true
		})
	}

	// Generate findings for counters that would benefit from sharding
	for _, counter := range counters {
		if counter.goroutines >= 2 || (counter.goroutines >= 1 && counter.inLoop) {
			finding := a.createFinding(ctx, counter)
			findings = append(findings, finding)
		}
	}

	return findings
}

// isAtomicCounter checks if a type is an atomic counter type.
func isAtomicCounter(typeName string) bool {
	atomicTypes := []string{
		"sync/atomic.Int32",
		"sync/atomic.Int64",
		"sync/atomic.Uint32",
		"sync/atomic.Uint64",
		"atomic.Int32",
		"atomic.Int64",
		"atomic.Uint32",
		"atomic.Uint64",
	}
	for _, at := range atomicTypes {
		if strings.HasSuffix(typeName, at) || typeName == at {
			return true
		}
	}
	return false
}

// checkGoroutineAccess checks for atomic counter access inside a goroutine.
func (a *Analyzer) checkGoroutineAccess(ctx *analysis.Context, goStmt *ast.GoStmt, counters map[string]*atomicCounterInfo) {
	ast.Inspect(goStmt, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if this is an atomic operation (Add, Store, Load, Swap, CompareAndSwap)
		if !isAtomicMethod(sel.Sel.Name) {
			return true
		}

		// Try to find which counter this refers to
		counterKey := a.resolveCounterKey(ctx, sel.X, counters)
		if counterKey != "" {
			if counter, ok := counters[counterKey]; ok {
				counter.goroutines++
				pos := ctx.Pass.Fset.Position(goStmt.Pos())
				counter.locations = append(counter.locations,
					fmt.Sprintf("goroutine at %s:%d", pos.Filename, pos.Line))
			}
		}

		return true
	})
}

// checkLoopAccess checks for atomic counter access inside loops.
func (a *Analyzer) checkLoopAccess(ctx *analysis.Context, loop ast.Node, counters map[string]*atomicCounterInfo) {
	ast.Inspect(loop, func(n ast.Node) bool {
		// Skip nested goroutines (handled separately)
		if _, ok := n.(*ast.GoStmt); ok {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if !isAtomicMethod(sel.Sel.Name) {
			return true
		}

		counterKey := a.resolveCounterKey(ctx, sel.X, counters)
		if counterKey != "" {
			if counter, ok := counters[counterKey]; ok {
				counter.inLoop = true
				pos := ctx.Pass.Fset.Position(loop.Pos())
				counter.locations = append(counter.locations,
					fmt.Sprintf("loop at %s:%d", pos.Filename, pos.Line))
			}
		}

		return true
	})
}

// isAtomicMethod checks if a method name is an atomic write operation.
func isAtomicMethod(name string) bool {
	writeMethods := []string{"Add", "Store", "Swap", "CompareAndSwap"}
	for _, m := range writeMethods {
		if name == m {
			return true
		}
	}
	return false
}

// resolveCounterKey tries to resolve an expression to a counter key.
// For c.counter.Add(1), expr is c.counter (the atomic field).
// We need to find which struct 'counter' belongs to.
func (a *Analyzer) resolveCounterKey(ctx *analysis.Context, expr ast.Expr, counters map[string]*atomicCounterInfo) string {
	// expr should be a selector expression like c.counter
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	fieldName := sel.Sel.Name // "counter"

	// sel.X is the receiver (e.g., "c" or "*c")
	// Get the type of the receiver
	tv, ok := ctx.Pass.TypesInfo.Types[sel.X]
	if !ok {
		return ""
	}

	// Get the underlying struct type
	t := tv.Type
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}

	structName := named.Obj().Name()
	key := fmt.Sprintf("%s.%s", structName, fieldName)

	if _, exists := counters[key]; exists {
		return key
	}

	return ""
}

// createFinding creates a finding for a counter that needs sharding.
func (a *Analyzer) createFinding(ctx *analysis.Context, counter *atomicCounterInfo) analysis.Finding {
	var details strings.Builder
	details.WriteString(fmt.Sprintf("atomic counter '%s.%s' has high contention risk\n", counter.structInfo.Name, counter.fieldName))
	details.WriteString(fmt.Sprintf("    Detected access from %d goroutine(s)", counter.goroutines))
	if counter.inLoop {
		details.WriteString(" + loop access")
	}
	details.WriteString("\n")

	if len(counter.locations) > 0 {
		details.WriteString("    Access locations:\n")
		for _, loc := range counter.locations {
			details.WriteString(fmt.Sprintf("      - %s\n", loc))
		}
	}

	details.WriteString("\n    Suggestion: Use sharded counter pattern:\n")
	details.WriteString(fmt.Sprintf(`
    type %sSharded struct {
        shards [runtime.GOMAXPROCS(0)]struct {
            value %s
            _     [%d]byte // cache line padding
        }
    }

    func (c *%sSharded) Add(delta int64) {
        id := runtime_procPin()
        runtime_procUnpin()
        c.shards[id].value.Add(delta)
    }

    func (c *%sSharded) Load() int64 {
        var total int64
        for i := range c.shards {
            total += c.shards[i].value.Load()
        }
        return total
    }
`,
		counter.structInfo.Name,
		counter.fieldType,
		ctx.CacheLineSize-8, // padding size
		counter.structInfo.Name,
		counter.structInfo.Name,
	))

	return analysis.Finding{
		Pos:      counter.structInfo.Pos,
		End:      counter.structInfo.TypeSpec.End(),
		Message:  details.String(),
		Severity: analysis.SeverityWarning,
	}
}
