package protocol

import (
	"io"
	"math/bits"
	"sync"
)

var (
	// Verify ByteBuffer implements the given interfaces.
	_ io.Writer = &ByteBuffer{}
)

// ByteBuffer implements a simple byte buffer.
type ByteBuffer struct {
	// B is the underlying byte slice.
	B []byte
}

// Reset resets bb.
func (bb *ByteBuffer) Reset() {
	bb.B = bb.B[:0]
}

// Write appends p to bb.
func (bb *ByteBuffer) Write(p []byte) (int, error) {
	bb.B = append(bb.B, p...)
	return len(p), nil
}

// pools contains pools for byte slices of various capacities.
//
//    pools[0] is for capacities from 0 to 8
//    pools[1] is for capacities from 9 to 16
//    pools[2] is for capacities from 17 to 32
//    ...
//    pools[n] is for capacities from 2^(n+2)+1 to 2^(n+3)
//
// Limit the maximum capacity to 2^18, since there are no performance benefits
// in caching byte slices with bigger capacities.
var pools [17]sync.Pool

// Get returns byte buffer with the given capacity.
func Get(capacity int) *ByteBuffer {
	id, capacityNeeded := getPoolIDAndCapacity(capacity)
	for i := 0; i < 2; i++ {
		if id < 0 || id >= len(pools) {
			break
		}
		if v := pools[id].Get(); v != nil {
			return v.(*ByteBuffer)
		}
		id++
	}
	return &ByteBuffer{
		B: make([]byte, 0, capacityNeeded),
	}
}

// Put returns bb to the pool.
func Put(bb *ByteBuffer) {
	capacity := cap(bb.B)
	id, poolCapacity := getPoolIDAndCapacity(capacity)
	if capacity <= poolCapacity {
		bb.Reset()
		pools[id].Put(bb)
	}
}

func getPoolIDAndCapacity(size int) (int, int) {
	size--
	if size < 0 {
		size = 0
	}
	size >>= 3
	id := bits.Len(uint(size))
	if id >= len(pools) {
		id = len(pools) - 1
	}
	return id, 1 << (id + 3)
}
