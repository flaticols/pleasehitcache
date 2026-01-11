package edge

import "sync"

// Struct that exceeds cache line - should say "already exceeds"
type LargeStruct struct { // want `already exceeds cache line`
	mu   sync.Mutex
	data [256]byte
}

// Empty struct - should not trigger (no fields to pad)
type EmptyStruct struct{}

// Single field struct with mutex
type SingleField struct { // want `struct 'SingleField'`
	mu sync.Mutex
}

// Struct with existing padding field - should still analyze
type AlreadyPadded struct { // want `struct 'AlreadyPadded'`
	mu    sync.Mutex
	value int64
	_     [32]byte // existing padding
}

// Struct with unexported fields only
type unexportedOnly struct { // want `struct 'unexportedOnly'`
	mu    sync.Mutex
	value int64
}

// Struct with pointer to mutex (not embedded)
type PointerMutex struct {
	mu    *sync.Mutex
	value int64
}

// Struct with interface field
type WithInterface struct { // want `struct 'WithInterface'`
	mu   sync.Mutex
	data any
}
