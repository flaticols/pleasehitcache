package pool

import "sync"

// Should trigger because it's stored in sync.Pool
type PooledBuffer struct { // want `struct 'PooledBuffer'`
	data [32]byte
}

var bufferPool = sync.Pool{
	New: func() any {
		return &PooledBuffer{}
	},
}

func GetBuffer() *PooledBuffer {
	return bufferPool.Get().(*PooledBuffer)
}

func PutBuffer(b *PooledBuffer) {
	bufferPool.Put(b)
}
