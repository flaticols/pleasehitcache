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
