package atomic

import "sync/atomic"

// Should trigger due to atomic.Int64 field
type AtomicCounter struct { // want `struct 'AtomicCounter'`
	value atomic.Int64
	flag  atomic.Bool
}

// Should trigger due to atomic.Value field
type AtomicValue struct { // want `struct 'AtomicValue'`
	data atomic.Value
}
