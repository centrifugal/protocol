package protocol

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetPutConcurrent(t *testing.T) {
	const concurrency = 10
	doneCh := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for capacity := 0; capacity < 100; capacity++ {
				bb := getByteBuffer(capacity)
				if len(bb.B) > 0 {
					panic(fmt.Errorf("len(bb.B) must be zero; got %d", len(bb.B)))
				}
				if capacity < 0 {
					capacity = 0
				}
				bb.B = append(bb.B, make([]byte, capacity)...)
				putByteBuffer(bb)
			}
			doneCh <- struct{}{}
		}()
	}
	tc := time.After(10 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case <-tc:
			t.Fatalf("timeout")
		case <-doneCh:
		}
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
