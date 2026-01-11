package main

import (
	"testing"

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
			ignorePatterns = tc.patterns
			defer func() { ignorePatterns = "" }()

			got := matchesIgnorePattern(tc.typeName)
			if got != tc.want {
				t.Errorf("matchesIgnorePattern(%q) = %v, want %v with patterns %q",
					tc.typeName, got, tc.want, tc.patterns)
			}
		})
	}
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
	// Use 64-byte cache line for predictable fix suggestions
	cacheLineSizeFlag = "64"
	defer func() { cacheLineSizeFlag = "auto" }()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "fix")
}

func TestMultiArchBehavior(t *testing.T) {
	testdata := analysistest.TestData()

	// Test with 64-byte cache line
	t.Run("64-byte cache line", func(t *testing.T) {
		cacheLineSizeFlag = "64"
		defer func() { cacheLineSizeFlag = "auto" }()
		analysistest.Run(t, testdata, Analyzer, "mutex")
	})

	// Test with 128-byte cache line
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
		wantAction    ActionType
	}{
		{
			name:          "good padding candidate",
			size:          48,
			cacheLineSize: 64,
			wantAction:    ActionPad,
		},
		{
			name:          "padding too large (>100% increase)",
			size:          16,
			cacheLineSize: 64,
			wantAction:    ActionWarn,
		},
		{
			name:          "exceeds cache line",
			size:          128,
			cacheLineSize: 64,
			wantAction:    ActionNoPad,
		},
		{
			name:          "exact cache line size",
			size:          64,
			cacheLineSize: 64,
			wantAction:    ActionNoPad,
		},
		{
			name:          "slice element anti-pattern",
			size:          32,
			cacheLineSize: 64,
			isSlice:       true,
			wantAction:    ActionNoPad,
		},
		{
			name:          "embedded anti-pattern",
			size:          32,
			cacheLineSize: 64,
			isEmbedded:    true,
			wantAction:    ActionNoPad,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := &StructInfo{
				Size:           tc.size,
				IsSliceElement: tc.isSlice,
				IsEmbedded:     tc.isEmbedded,
			}

			rec := generateRecommendation(info, tc.cacheLineSize)
			if rec.Action != tc.wantAction {
				t.Errorf("generateRecommendation() = %v, want %v", rec.Action, tc.wantAction)
			}
		})
	}
}

func TestFieldDetection(t *testing.T) {
	// These are unit tests for detection functions
	// Real struct types would be tested via analysistest

	t.Run("matchesIgnorePattern with spaces", func(t *testing.T) {
		ignorePatterns = "Foo, Bar, Baz"
		defer func() { ignorePatterns = "" }()

		if !matchesIgnorePattern("Bar") {
			t.Error("should match 'Bar' with spaces in pattern")
		}
	})
}
