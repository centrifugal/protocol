package protocol

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// testCompressionLevel is a valid level for the tests which are not about the
// level: a fixed value, not a recommendation.
const testCompressionLevel = 7

var testFrameDict = []byte(`{"push":{"id":,"pub":{"data":{"offset":`)

func TestFrameCodecRoundTrip(t *testing.T) {
	c := NewDeflateFrameCodec("v1", testFrameDict, testCompressionLevel)

	msg := []byte(`{"push":{"id":7,"pub":{"data":{"price":123.45},"offset":42}}}`)
	frame := c.Compress(nil, msg)
	require.Equal(t, FrameCodecCompressed, frame[0])
	require.Less(t, len(frame), len(msg))
	out, err := c.Decompress(nil, frame, 1<<20)
	require.NoError(t, err)
	require.Equal(t, msg, out)

	// What does not compress is sent as it is, one byte longer.
	rnd := make([]byte, 64)
	for i := range rnd {
		rnd[i] = byte(i*7 + i*i*13)
	}
	frame = c.Compress(nil, rnd)
	require.Equal(t, FrameCodecRaw, frame[0])
	require.Len(t, frame, len(rnd)+1)
	out, err = c.Decompress(nil, frame, 1<<20)
	require.NoError(t, err)
	require.Equal(t, rnd, out)

	// A frame which inflates beyond the limit is refused.
	frame = c.Compress(nil, bytes.Repeat([]byte("A"), 100000))
	_, err = c.Decompress(nil, frame, 1000)
	require.ErrorIs(t, err, ErrFrameTooLarge)
}

func TestFrameCodecConcurrent(t *testing.T) {
	c := NewDeflateFrameCodec("v1", testFrameDict, testCompressionLevel)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				msg := []byte(fmt.Sprintf(`{"push":{"id":%d,"pub":{"data":{"v":%d}}}}`, g, i))
				out, err := c.Decompress(nil, c.Compress(nil, msg), 1<<20)
				if err != nil || !bytes.Equal(out, msg) {
					t.Errorf("round trip of %s: %q, %v", msg, out, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestFrameCodecDecompressErrors(t *testing.T) {
	c := NewDeflateFrameCodec("v1", []byte("dict"), testCompressionLevel)
	_, err := c.Decompress(nil, nil, 0)
	require.ErrorIs(t, err, ErrEmptyFrame)
	_, err = c.Decompress(nil, []byte{0xff, 0x01, 0x02}, 0)
	require.ErrorIs(t, err, ErrUnknownFrameCodec)
}

// A compressed frame comes from the network, so one which is not valid
// DEFLATE is an error and not partial output. The reader which failed goes
// back to the pool, so valid frames must decode after it as before.
func TestFrameCodecDecompressCorruptFrame(t *testing.T) {
	c := NewDeflateFrameCodec("v1", testFrameDict, testCompressionLevel)
	msg := []byte(`{"push":{"id":7,"pub":{"data":{"price":123.45},"offset":42}}}`)
	valid := c.Compress(nil, msg)
	require.Equal(t, FrameCodecCompressed, valid[0])

	for name, frame := range map[string][]byte{
		"truncated body": valid[:len(valid)/2],
		"no body":        {FrameCodecCompressed},
		// 0xff starts a block of the reserved DEFLATE block type.
		"reserved block type": {FrameCodecCompressed, 0xff, 0xff, 0xff},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := c.Decompress(nil, frame, 1<<20)
			require.Error(t, err)
			require.Nil(t, out)

			out, err = c.Decompress(nil, valid, 1<<20)
			require.NoError(t, err)
			require.Equal(t, msg, out)
		})
	}
}

// Compress and Decompress append to dst, for both kinds of frames, and
// maxSize is about the decompressed output only, not about what dst had.
func TestFrameCodecAppendsToDst(t *testing.T) {
	c := NewDeflateFrameCodec("v1", testFrameDict, testCompressionLevel)
	prefix := []byte("prefix")
	for _, tt := range []struct {
		name   string
		msg    []byte
		marker byte
	}{
		{"compressed", []byte(`{"push":{"id":7,"pub":{"data":{"price":123.45},"offset":42}}}`), FrameCodecCompressed},
		{"raw", []byte{0x01, 0x80, 0x7f, 0x13}, FrameCodecRaw},
	} {
		t.Run(tt.name, func(t *testing.T) {
			frame := c.Compress(bytes.Clone(prefix), tt.msg)
			require.True(t, bytes.HasPrefix(frame, prefix))
			frame = frame[len(prefix):]
			require.Equal(t, tt.marker, frame[0])

			out, err := c.Decompress(bytes.Clone(prefix), frame, len(tt.msg))
			require.NoError(t, err)
			require.Equal(t, append(bytes.Clone(prefix), tt.msg...), out)
		})
	}
}

// maxSize of math.MaxInt must not overflow the budget of the reader, which
// used to make Decompress and InflateDictionary return nothing without an
// error.
func TestFrameCodecMaxSizeOverflow(t *testing.T) {
	dict := []byte("some dictionary content used for compression testing 1234567890")
	c := NewDeflateFrameCodec("v1", dict, testCompressionLevel)
	msg := []byte("hello world hello world hello world hello world")
	out, err := c.Decompress(nil, c.Compress(nil, msg), math.MaxInt)
	require.NoError(t, err)
	require.Equal(t, msg, out)

	out, err = InflateDictionary(DeflateDictionary(dict, testCompressionLevel), math.MaxInt)
	require.NoError(t, err)
	require.Equal(t, dict, out)
}

func TestFrameCodecAccessors(t *testing.T) {
	dict := []byte("some dictionary content")
	c := NewDeflateFrameCodec("v42", dict, testCompressionLevel)
	require.Equal(t, "v42", c.ID())
	require.Equal(t, dict, c.Dict())
}

func TestDeflateDictionary(t *testing.T) {
	dict := bytes.Repeat([]byte(`{"push":{"id":3,"pub":{"data":{}}}}`), 50)
	compressed := DeflateDictionary(dict, testCompressionLevel)
	require.NotEmpty(t, compressed)
	require.Less(t, len(compressed), len(dict))

	out, err := InflateDictionary(compressed, len(dict))
	require.NoError(t, err)
	require.Equal(t, dict, out)

	_, err = InflateDictionary(compressed, len(dict)-1)
	require.Error(t, err, "a dictionary larger than maxSize")

	// Content which is not valid DEFLATE is refused, not installed cut short
	// or empty.
	for name, data := range map[string][]byte{
		"truncated":           compressed[:len(compressed)/2],
		"empty":               {},
		"reserved block type": {0xff, 0xff, 0xff},
	} {
		out, err := InflateDictionary(data, len(dict))
		require.Error(t, err, "%s: %d bytes of output", name, len(out))
	}
}

// Below MinDictionaryCompressionLevel the dictionary is silently ignored, so a
// codec with such a level would look fine and never use it.
func TestNewDeflateFrameCodecRejectsLowLevel(t *testing.T) {
	require.Panics(t, func() {
		NewDeflateFrameCodec("v1", []byte("dict"), MinDictionaryCompressionLevel-1)
	})
}

// The dictionary must actually be used. Both DEFLATE implementations take a
// dictionary at every level and ignore it at the lower ones, which shows only
// as most of the compression ratio gone.
func TestFrameCodecDictionaryApplies(t *testing.T) {
	var d bytes.Buffer
	for d.Len() < 4096 {
		d.WriteString(`{"push":{"id":3,"pub":{"data":{"price":100.00},"offset":1}}}`)
	}
	msg := []byte(`{"push":{"id":7,"pub":{"data":{"price":123.45},"offset":42}}}`)

	withDict := len(NewDeflateFrameCodec("v", d.Bytes()[:4096], testCompressionLevel).Compress(nil, msg))
	withoutDict := len(NewDeflateFrameCodec("v", nil, testCompressionLevel).Compress(nil, msg))
	// Worth a lot, not a rounding error.
	require.Less(t, float64(withDict), 0.6*float64(withoutDict), "%d bytes with the dictionary, %d without", withDict, withoutDict)
}
