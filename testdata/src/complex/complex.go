package complex

import (
	"sync"
	"sync/atomic"
)

// Multiple detection triggers: mutex + atomic
type MultiTrigger struct { // want `struct 'MultiTrigger'`
	mu      sync.Mutex
	counter atomic.Int64
	flag    atomic.Bool
}

// Nested struct usage
type Inner struct {
	value int64
}

type Outer struct { // want `struct 'Outer'`
	mu    sync.Mutex
	inner Inner
}

// Struct with multiple mutex fields
type DoubleMutex struct { // want `struct 'DoubleMutex'`
	readMu  sync.RWMutex
	writeMu sync.Mutex
	value   int64
}

// Channel field
type WithChannel struct { // want `struct 'WithChannel'`
	mu   sync.Mutex
	done chan struct{}
}

// Map field
type WithMap struct { // want `struct 'WithMap'`
	mu   sync.Mutex
	data map[string]int
}

// Slice field (not as element, but containing slice)
type ContainsSlice struct { // want `struct 'ContainsSlice'`
	mu    sync.Mutex
	items []int
}

// Both slice element AND has mutex - anti-pattern should win
type SliceAndMutex struct { // want `slice element`
	mu    sync.Mutex
	value int64
}

var sliceOfSliceAndMutex []SliceAndMutex

// Both embedded AND has atomic - anti-pattern should win
type EmbeddedAndAtomic struct { // want `embedded in another struct`
	counter atomic.Int64
}

type ContainerOfEmbedded struct {
	EmbeddedAndAtomic
	extra int64
}

// Directive on struct that's also slice element - directive should analyze but warn
//go:cachepad
type DirectiveButSlice struct { // want `slice element`
	mu sync.Mutex
}

var sliceOfDirectiveButSlice []DirectiveButSlice
