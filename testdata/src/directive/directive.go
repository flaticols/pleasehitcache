package directive

import "sync"

//go:cachepad
type Counter struct { // want `struct 'Counter'`
	mu    sync.Mutex
	value int64
	data  [48]byte // Make struct larger so padding is reasonable
}

// No directive - should not trigger unless -all flag
type RegularStruct struct {
	a int
	b string
}
