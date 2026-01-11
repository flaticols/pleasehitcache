package embedded

import "sync"

// Should warn against padding because it's embedded in another struct
type EmbeddedMutex struct { // want `embedded in another struct`
	mu sync.Mutex
}

// This embeds EmbeddedMutex
type Container struct {
	EmbeddedMutex
	value int64
}
