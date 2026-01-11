package mutex

import "sync"

// Should trigger due to sync.Mutex field
type MutexCounter struct { // want `struct 'MutexCounter'`
	mu    sync.Mutex
	value int64
}

// Should trigger due to sync.RWMutex field
type RWMutexCounter struct { // want `struct 'RWMutexCounter'`
	mu    sync.RWMutex
	value int64
}
