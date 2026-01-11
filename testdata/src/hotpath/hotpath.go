package hotpath

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

// LoopCounter - accessed in tight loops
type LoopCounter struct { // want `struct 'LoopCounter'`
	mu    sync.Mutex
	value int64
}

func useInLoop() {
	counter := &LoopCounter{}
	for i := 0; i < 1000; i++ {
		counter.mu.Lock()
		counter.value++
		counter.mu.Unlock()
	}
}

// NestedLoopData - accessed in nested loops (higher score)
type NestedLoopData struct { // want `struct 'NestedLoopData'`
	mu    sync.Mutex
	value int64
}

func useInNestedLoop() {
	data := &NestedLoopData{}
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			data.mu.Lock()
			data.value++
			data.mu.Unlock()
		}
	}
}

// RangeLoopItem - accessed in range loops
type RangeLoopItem struct { // want `struct 'RangeLoopItem'`
	mu    sync.Mutex
	value int64
}

func useInRangeLoop(items []*RangeLoopItem) {
	for _, item := range items {
		item.mu.Lock()
		item.value++
		item.mu.Unlock()
	}
}

// GoroutineState - passed to goroutines
type GoroutineState struct { // want `struct 'GoroutineState'`
	mu    sync.Mutex
	value int64
}

func useInGoroutine() {
	state := &GoroutineState{}
	go func() {
		state.mu.Lock()
		state.value++
		state.mu.Unlock()
	}()
}

// CapturedInClosure - captured by closure in goroutine
type CapturedInClosure struct { // want `struct 'CapturedInClosure'`
	counter atomic.Int64
}

func captureInClosure() {
	data := &CapturedInClosure{}
	go func() {
		data.counter.Add(1)
	}()
}

// HandlerData - used in HTTP handler
type HandlerData struct { // want `struct 'HandlerData'`
	mu      sync.Mutex
	counter int64
}

func (h *HandlerData) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.counter++
	h.mu.Unlock()
}

// BenchmarkTarget - used in benchmark
type BenchmarkTarget struct { // want `struct 'BenchmarkTarget'`
	mu    sync.Mutex
	value int64
}

func BenchmarkTarget_Inc(b *testing.B) {
	target := &BenchmarkTarget{}
	for i := 0; i < b.N; i++ {
		target.mu.Lock()
		target.value++
		target.mu.Unlock()
	}
}

// ChannelData - used in channel select
type ChannelData struct { // want `struct 'ChannelData'`
	mu    sync.Mutex
	value int64
}

func useWithChannel() {
	ch := make(chan *ChannelData)
	go func() {
		select {
		case data := <-ch:
			data.mu.Lock()
			data.value++
			data.mu.Unlock()
		}
	}()
}

// NotHot - not in any hot path, no mutex
type NotHot struct {
	name string
	id   int
}

func useNotHot() {
	data := &NotHot{name: "test"}
	_ = data.id
}
