package cfjson

import (
	"strconv"
	"strings"
	"testing"
)

// Strings of the sizes generated encoders write most: identifiers, channel
// names, and something long.
func BenchmarkAppendString(b *testing.B) {
	for _, size := range []int{1, 2, 4, 6, 7, 8, 12, 16, 17, 24, 32, 36, 160} {
		s := strings.Repeat("channel:", 20)[:size]
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			buf := make([]byte, 0, 256)
			for i := 0; i < b.N; i++ {
				buf = AppendString(buf[:0], s)
			}
		})
	}
}
