package protocol

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPutConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for capacity := 0; capacity < 100; capacity++ {
				bb := getByteBuffer(capacity)
				if len(bb.B) > 0 {
					t.Errorf("a buffer from the pool has %d bytes in it", len(bb.B))
					return
				}
				bb.B = append(bb.B, make([]byte, capacity)...)
				putByteBuffer(bb)
			}
		}()
	}
	wg.Wait()
}

func TestGetByteBuffer_NonPositiveLength(t *testing.T) {
	for _, length := range []int{0, -1, math.MinInt} {
		bb := getByteBuffer(length)
		require.NotNil(t, bb)
		require.Empty(t, bb.B)
	}
}

func TestGetCapacity(t *testing.T) {
	for i := 1; i < 130; i++ {
		idx := nextLogBase2(uint32(i))
		b := getByteBuffer(i)
		require.Equal(t, 1<<idx, cap(b.B))
		putByteBuffer(b)
	}
}

func TestByteBufferWrite(t *testing.T) {
	var bb ByteBuffer
	n, err := bb.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	n, err = bb.Write([]byte(" world"))
	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "hello world", string(bb.B))
	bb.Reset()
	require.Equal(t, "", string(bb.B))
}

func TestPrevLogBase2(t *testing.T) {
	// Exact powers of two: prevLogBase2 must equal the exponent itself.
	require.Equal(t, uint32(0), prevLogBase2(1))
	require.Equal(t, uint32(3), prevLogBase2(8))
	require.Equal(t, uint32(4), prevLogBase2(16))
	// Non-powers of two: prevLogBase2 must round down to the exponent below.
	require.Equal(t, uint32(3), prevLogBase2(9))
	require.Equal(t, uint32(3), prevLogBase2(15))
	require.Equal(t, uint32(4), prevLogBase2(17))
}
