package protocol

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// encodeJSON writes into the caller supplied buffer when it is large enough,
// which is what the reuse argument of the push encoders relies on to avoid an
// allocation per subscriber.
func TestEncodeJSON_Reuse(t *testing.T) {
	cmd := &Command{Id: 1}
	reuse := make([]byte, 0, 64)
	out := encodeJSON(cmd, reuse)
	require.Equal(t, `{"id":1}`, string(out))
	require.Same(t, &reuse[:1][0], &out[0], "expected the result to be written into the reuse buffer")

	// A buffer which is too small must be left alone and a new one allocated.
	small := make([]byte, 0, 2)
	out = encodeJSON(cmd, small)
	require.Equal(t, `{"id":1}`, string(out))
	require.Equal(t, 2, cap(small))
}

// The result must never alias the pooled scratch buffer: the next message
// encoded would overwrite it.
func TestEncodeJSON_ResultIsNotShared(t *testing.T) {
	first := encodeJSON(&Command{Id: 1})
	second := encodeJSON(&Command{Id: 2})
	require.Equal(t, `{"id":1}`, string(first))
	require.Equal(t, `{"id":2}`, string(second))
	require.Equal(t, len(first), cap(first))
}

// A message larger than maxBufferLength is encoded correctly, its scratch
// buffer is just not kept.
func TestEncodeJSON_Large(t *testing.T) {
	channel := strings.Repeat("c", maxBufferLength+1)
	out := encodeJSON(&Push{Channel: channel})
	require.Equal(t, `{"channel":"`+channel+`"}`, string(out))
}
