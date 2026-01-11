package fix

import "sync"

// Struct that should get a fix suggestion (size 32, needs 32 padding for 64-byte cache line)
type NeedsPadding struct { // want `should add \d+-byte padding`
	mu    sync.RWMutex // 24 bytes
	value int64        // 8 bytes = 32 total
}

// Struct that fits well for padding
type GoodCandidate struct { // want `should add \d+-byte padding`
	mu   sync.Mutex // 8 bytes
	a    int64      // 8 bytes
	b    int64      // 8 bytes
	c    int64      // 8 bytes
	d    int64      // 8 bytes
	e    int64      // 8 bytes = 48 total, needs 16 for 64
}

// Struct where padding would be too large (16 bytes, would need 48 = 3x size)
type TooSmallForPadding struct { // want `could benefit.*significantly increase`
	mu    sync.Mutex
	value int64
}
