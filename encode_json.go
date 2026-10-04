package protocol

import (
	"sync"

	"github.com/centrifugal/protocol/cfjson"
)

// jsonBuffer is a scratch buffer messages are encoded into before the result
// is copied to a slice of the exact size.
type jsonBuffer struct {
	b []byte
}

var jsonBufferPool = sync.Pool{
	New: func() any {
		return &jsonBuffer{b: make([]byte, 0, 512)}
	},
}

// encodeJSON returns the JSON encoding of m. If a reuse buffer is given and it
// is large enough the result is written into it, otherwise a new slice is
// allocated.
func encodeJSON(m cfjson.Appender, reuse ...[]byte) []byte {
	buf := jsonBufferPool.Get().(*jsonBuffer)
	b := m.AppendJSON(buf.b[:0])
	var ret []byte
	if len(reuse) == 1 && cap(reuse[0]) >= len(b) {
		ret = reuse[0][:len(b)]
	} else {
		ret = make([]byte, len(b))
	}
	copy(ret, b)
	// Keep the grown buffer for the next message, unless it is so large that
	// holding on to it would waste memory.
	if cap(b) <= maxBufferLength {
		buf.b = b
		jsonBufferPool.Put(buf)
	}
	return ret
}
