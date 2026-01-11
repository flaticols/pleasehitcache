package shardedcounter

import (
	"sync/atomic"
)

// Counter with atomic accessed from multiple goroutines - should suggest sharding
type HighContentionCounter struct { // want `high contention risk` `could benefit from padding`
	counter atomic.Int64
}

func useHighContention() {
	c := &HighContentionCounter{}

	// Multiple goroutines accessing the same counter
	go func() {
		c.counter.Add(1)
	}()

	go func() {
		c.counter.Add(1)
	}()

	go func() {
		c.counter.Add(1)
	}()
}

// Counter accessed in loop inside goroutine - high contention
type LoopCounter struct { // want `high contention risk` `could benefit from padding`
	value atomic.Int64
}

func useLoopCounter() {
	c := &LoopCounter{}

	go func() {
		for i := 0; i < 1000; i++ {
			c.value.Add(1)
		}
	}()
}

// Counter with multiple atomic fields - both fields accessed from different goroutines
type MultiCounter struct { // want `could benefit from padding`
	requests atomic.Int64
	errors   atomic.Int64
}

func useMultiCounter() {
	c := &MultiCounter{}

	go func() {
		c.requests.Add(1)
	}()

	go func() {
		c.errors.Add(1)
	}()
}

// Counter used only in single goroutine - should NOT suggest sharding but may suggest padding
type SingleGoroutineCounter struct { // want `could benefit from padding`
	value atomic.Int64
}

func useSingleGoroutine() {
	c := &SingleGoroutineCounter{}
	c.value.Add(1)
	c.value.Add(1)
}

// Stats struct with multiple counters - common pattern
type Stats struct { // want `could benefit from padding`
	hits   atomic.Int64
	misses atomic.Int64
}

func useStats() {
	s := &Stats{}

	// Worker pool accessing stats
	go func() {
		s.hits.Add(1)
	}()

	go func() {
		s.misses.Add(1)
	}()
}

// Uint64 counter - should also detect
type Uint64Counter struct { // want `high contention risk` `could benefit from padding`
	count atomic.Uint64
}

func useUint64() {
	c := &Uint64Counter{}

	go func() {
		c.count.Add(1)
	}()

	go func() {
		c.count.Add(1)
	}()
}

// Int32 counter - should also detect
type Int32Counter struct { // want `high contention risk` `could benefit from padding`
	count atomic.Int32
}

func useInt32() {
	c := &Int32Counter{}

	go func() {
		c.count.Add(1)
	}()

	go func() {
		c.count.Add(1)
	}()
}

// Counter only using Load (read) - reads don't cause write contention
type ReadOnlyCounter struct { // want `could benefit from padding`
	value atomic.Int64
}

func useReadOnly() {
	c := &ReadOnlyCounter{}

	go func() {
		_ = c.value.Load()
	}()

	go func() {
		_ = c.value.Load()
	}()
}
