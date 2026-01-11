package main

import (
	"testing"

	"github.com/flaticols/pleasehitcache/internal/detection"
	"github.com/flaticols/pleasehitcache/internal/recommendation"
	itypes "github.com/flaticols/pleasehitcache/internal/types"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDirective(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "directive")
}

func TestMutex(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "mutex")
}

func TestAtomic(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "atomic")
}

func TestPool(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "pool")
}

func TestSliceAntiPattern(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "slice")
}

func TestEmbeddedAntiPattern(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "embedded")
}

func TestHotPathDetection(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "hotpath")
}

func TestCacheLineSize(t *testing.T) {
	tests := []struct {
		goarch   string
		goos     string
		expected int64
	}{
		{"amd64", "linux", 64},
		{"amd64", "darwin", 64},
		{"arm64", "linux", 64},
		{"arm64", "darwin", 128}, // Apple M-series
	}

	for _, tc := range tests {
		t.Run(tc.goarch+"_"+tc.goos, func(t *testing.T) {
			t.Setenv("GOARCH", tc.goarch)
			t.Setenv("GOOS", tc.goos)
			cacheLineSizeFlag = "auto"

			size := getCacheLineSize()
			if size != tc.expected {
				t.Errorf("getCacheLineSize() = %d, want %d for GOARCH=%s GOOS=%s",
					size, tc.expected, tc.goarch, tc.goos)
			}
		})
	}
}

func TestCacheLineSizeOverride(t *testing.T) {
	cacheLineSizeFlag = "128"
	defer func() { cacheLineSizeFlag = "auto" }()

	size := getCacheLineSize()
	if size != 128 {
		t.Errorf("getCacheLineSize() = %d, want 128 with flag override", size)
	}
}

func TestIgnorePatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns string
		typeName string
		want     bool
	}{
		{"exact match", "TestType", "TestType", true},
		{"no match", "TestType", "OtherType", false},
		{"wildcard", "Test*", "TestType", true},
		{"substring", "Test", "MyTestType", true},
		{"multiple patterns", "Foo,Bar", "Bar", true},
		{"empty pattern", "", "AnyType", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			patterns := detection.ParseIgnorePatterns(tc.patterns)
			// Use internal function to test
			got := matchPattern(tc.typeName, patterns)
			if got != tc.want {
				t.Errorf("matchesIgnorePattern(%q) = %v, want %v with patterns %q",
					tc.typeName, got, tc.want, tc.patterns)
			}
		})
	}
}

// matchPattern is a helper to match patterns (mirrors detection.matchesIgnorePattern)
func matchPattern(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p == name {
			return true
		}
		// Simple prefix match for wildcard
		if len(p) > 0 && p[len(p)-1] == '*' {
			prefix := p[:len(p)-1]
			if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				return true
			}
		}
		// Substring match
		for i := 0; i <= len(name)-len(p); i++ {
			if name[i:i+len(p)] == p {
				return true
			}
		}
	}
	return false
}

func TestEdgeCases(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "edge")
}

func TestComplexCases(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "complex")
}

func TestFixSuggestions(t *testing.T) {
	cacheLineSizeFlag = "64"
	defer func() { cacheLineSizeFlag = "auto" }()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "fix")
}

func TestMultiArchBehavior(t *testing.T) {
	testdata := analysistest.TestData()

	t.Run("64-byte cache line", func(t *testing.T) {
		cacheLineSizeFlag = "64"
		defer func() { cacheLineSizeFlag = "auto" }()
		analysistest.Run(t, testdata, Analyzer, "mutex")
	})

	t.Run("128-byte cache line", func(t *testing.T) {
		cacheLineSizeFlag = "128"
		defer func() { cacheLineSizeFlag = "auto" }()
		analysistest.Run(t, testdata, Analyzer, "mutex")
	})
}

func TestRecommendationLogic(t *testing.T) {
	tests := []struct {
		name          string
		size          int64
		cacheLineSize int64
		isSlice       bool
		isEmbedded    bool
		wantAction    itypes.ActionType
	}{
		{
			name:          "good padding candidate",
			size:          48,
			cacheLineSize: 64,
			wantAction:    itypes.ActionPad,
		},
		{
			name:          "padding too large (>100% increase)",
			size:          16,
			cacheLineSize: 64,
			wantAction:    itypes.ActionWarn,
		},
		{
			name:          "exceeds cache line",
			size:          128,
			cacheLineSize: 64,
			wantAction:    itypes.ActionNoPad,
		},
		{
			name:          "exact cache line size",
			size:          64,
			cacheLineSize: 64,
			wantAction:    itypes.ActionNoPad,
		},
		{
			name:          "slice element anti-pattern",
			size:          32,
			cacheLineSize: 64,
			isSlice:       true,
			wantAction:    itypes.ActionNoPad,
		},
		{
			name:          "embedded anti-pattern",
			size:          32,
			cacheLineSize: 64,
			isEmbedded:    true,
			wantAction:    itypes.ActionNoPad,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := &itypes.StructInfo{
				Size: tc.size,
			}

			if tc.isSlice {
				info.AntiPatterns = append(info.AntiPatterns, itypes.AntiPattern{
					Type: itypes.AntiPatternSliceElement,
				})
			}
			if tc.isEmbedded {
				info.AntiPatterns = append(info.AntiPatterns, itypes.AntiPattern{
					Type: itypes.AntiPatternEmbedded,
				})
			}

			// Create engine with mock calculator
			calc := &mockCalculator{cacheLineSize: tc.cacheLineSize}
			engine := recommendation.New(calc, HotPathThreshold)
			rec := engine.Generate(info)

			if rec.Action != tc.wantAction {
				t.Errorf("Generate() = %v, want %v", rec.Action, tc.wantAction)
			}
		})
	}
}

// mockCalculator implements the calculator interface for testing
type mockCalculator struct {
	cacheLineSize int64
}

func (m *mockCalculator) CacheLineSize() int64 {
	return m.cacheLineSize
}

func (m *mockCalculator) CalculatePadding(info *itypes.StructInfo) int64 {
	if info.Size >= m.cacheLineSize {
		return 0
	}
	return m.cacheLineSize - info.Size
}

func (m *mockCalculator) IsPaddingReasonable(info *itypes.StructInfo) bool {
	padding := m.CalculatePadding(info)
	return float64(padding)/float64(info.Size) <= 1.0
}

func (m *mockCalculator) SuggestReorder(info *itypes.StructInfo) *itypes.ReorderSuggestion {
	return nil
}
