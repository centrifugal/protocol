package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/segmentio/encoding/json"

	"github.com/stretchr/testify/require"
)

func getTestFrame(tb testing.TB, protoType Type, minCommandLength int) []byte {
	tb.Helper()
	ch := make([]byte, minCommandLength)
	for i := 0; i < minCommandLength; i++ {
		ch[i] = 'a'
	}
	cmd := &Command{
		Publish: &PublishRequest{
			Channel: string(ch),
			Data:    []byte(`{}`),
		},
	}
	var frame []byte
	if protoType == TypeProtobuf {
		data, err := cmd.MarshalVT()
		require.NoError(tb, err)
		encoder := GetDataEncoder(TypeProtobuf)
		err = encoder.Encode(data)
		require.NoError(tb, err)
		err = encoder.Encode(data)
		require.NoError(tb, err)
		frame = encoder.Finish()
		PutDataEncoder(TypeProtobuf, encoder)
	} else {
		data, err := json.Marshal(cmd)
		require.NoError(tb, err)
		encoder := GetDataEncoder(TypeJSON)
		err = encoder.Encode(data)
		require.NoError(tb, err)
		err = encoder.Encode(data)
		require.NoError(tb, err)
		frame = encoder.Finish()
		PutDataEncoder(TypeJSON, encoder)
	}
	return frame
}

func TestStreamingDecode_Protobuf(t *testing.T) {
	frame := getTestFrame(t, TypeProtobuf, 10000)
	testDecodingFrame(t, frame, TypeProtobuf)
}

func TestStreamingDecode_JSON(t *testing.T) {
	frame := getTestFrame(t, TypeJSON, 10000)
	testDecodingFrame(t, frame, TypeJSON)
}

func TestStreamingDecode_JSON_MessageLimit(t *testing.T) {
	frame := getTestFrame(t, TypeJSON, 10000)
	dec := GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(frame), 100)
	_, _, err := dec.Decode()
	require.ErrorIs(t, err, ErrMessageTooLarge)
}

func TestStreamingDecode_Protobuf_MessageLimit(t *testing.T) {
	frame := getTestFrame(t, TypeProtobuf, 10000)
	dec := GetStreamCommandDecoderLimited(TypeProtobuf, bytes.NewReader(frame), 100)
	_, _, err := dec.Decode()
	require.ErrorIs(t, err, ErrMessageTooLarge)
}

// BenchmarkStreamingDecode_Protobuf is mostly to check correctness under parallel execution
// and with large enough messages.
func BenchmarkStreamingDecode_Protobuf(b *testing.B) {
	frame := getTestFrame(b, TypeProtobuf, 10000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testDecodingFrame(b, frame, TypeProtobuf)
		}
	})
}

// BenchmarkStreamingDecode_JSON is mostly to check correctness under parallel execution
// and with large enough messages.
func BenchmarkStreamingDecode_JSON(b *testing.B) {
	frame := getTestFrame(b, TypeJSON, 10000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testDecodingFrame(b, frame, TypeJSON)
		}
	})
}

func testDecodingFrame(tb testing.TB, frame []byte, protoType Type) {
	dec := GetStreamCommandDecoderLimited(protoType, bytes.NewReader(frame), 1<<20)
	_, size, err := dec.Decode()
	require.NoError(tb, err)
	if protoType == TypeProtobuf {
		require.Equal(tb, 10018, size)
	} else {
		require.Equal(tb, 10037, size)
	}
	_, size, err = dec.Decode()
	if protoType == TypeProtobuf {
		require.Equal(tb, 10018, size)
	} else {
		require.Equal(tb, 10036, size)
	}
	if err != nil {
		require.ErrorIs(tb, err, io.EOF)
	} else {
		_, _, err = dec.Decode()
		require.ErrorIs(tb, err, io.EOF)
	}
	PutStreamCommandDecoder(protoType, dec)
}

func TestJSONStreamCommandDecoder(t *testing.T) {
	// Sample data emulating a network stream of JSON commands with newlines
	data := `{"publish":{"channel":"1","data":{}}}
{"publish":{"channel":"2","data":{}}}
{"publish":{"channel":"3","data":{}}}
{"publish":{"channel":"4","data":{}}}
{"publish":{"channel":"5","data":{}}}
{"publish":{"channel":"6","data":{}}}`

	testCases := []struct {
		name             string
		messageSizeLimit int64
	}{
		{
			name:             "generous limit",
			messageSizeLimit: 1 << 20,
		},
		{
			name:             "with limit",
			messageSizeLimit: 50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewBufferString(data)
			decoder := NewJSONStreamCommandDecoder(reader, tc.messageSizeLimit)

			numMessagesRead := 0
			i := 0
			for {
				i++
				cmd, _, err := decoder.Decode()
				if err != nil {
					if err == io.EOF {
						require.NotNil(t, cmd)
						require.Equal(t, cmd.Publish.Channel, strconv.Itoa(i))
						numMessagesRead += 1
						break // End of data reached.
					} else {
						require.NoError(t, err)
					}
				}
				require.NotNil(t, cmd)
				require.Equal(t, cmd.Publish.Channel, strconv.Itoa(i))
				numMessagesRead += 1
			}
			require.Equal(t, 6, numMessagesRead)
		})
	}
}

func TestJSONStreamCommandDecoder_ReuseDifferentLimit(t *testing.T) {
	// Sample data emulating a network stream of JSON commands with newlines
	data := `{"publish":{"channel":"1","data":{}}}
{"publish":{"channel":"1","data":{}}}`
	decoder := GetStreamCommandDecoderLimited(TypeJSON, bytes.NewBufferString(data), 10)
	_, _, err := decoder.Decode()
	require.ErrorIs(t, err, ErrMessageTooLarge)
	PutStreamCommandDecoder(TypeJSON, decoder)
	decoder = GetStreamCommandDecoderLimited(TypeJSON, bytes.NewBufferString(data), 1<<20)
	cmd, _, err := decoder.Decode()
	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.NotNil(t, cmd.Publish)
	cmd, _, err = decoder.Decode()
	require.ErrorIs(t, err, io.EOF)
	require.NotNil(t, cmd)
	require.NotNil(t, cmd.Publish)
}

// A Protobuf stream declares the length of each message before its body, so the
// length must never be trusted as an allocation size - a few bytes must not be
// able to make the decoder allocate (or panic) on an arbitrary size. See also
// FuzzProtobufStreamDecode.
func TestProtobufStreamCommandDecoder_HostileLength(t *testing.T) {
	uvarint := func(v uint64) []byte {
		b := make([]byte, binary.MaxVarintLen64)
		return b[:binary.PutUvarint(b, v)]
	}

	tests := []struct {
		name      string
		msgLength uint64
	}{
		{name: "larger than max int32", msgLength: math.MaxInt32 + 1},
		{name: "overflows int32", msgLength: 1 << 40},
		{name: "overflows int64 when converted", msgLength: 1 << 62},
		{name: "max uint64", msgLength: math.MaxUint64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The hostile length far exceeds the configured limit and must be
			// rejected rather than allocated or panicked on.
			decoder := GetStreamCommandDecoderLimited(TypeProtobuf, bytes.NewReader(uvarint(tt.msgLength)), 1<<20)
			defer PutStreamCommandDecoder(TypeProtobuf, decoder)
			require.NotPanics(t, func() {
				cmd, n, err := decoder.Decode()
				require.Nil(t, cmd)
				require.Zero(t, n)
				require.ErrorIs(t, err, ErrMessageTooLarge)
			})
		})
	}
}

// A length within the ceiling must still be handled normally: it is only
// rejected once the body turns out to be shorter than declared.
func TestProtobufStreamCommandDecoder_TruncatedBody(t *testing.T) {
	b := make([]byte, binary.MaxVarintLen64)
	prefix := b[:binary.PutUvarint(b, 1024)]

	decoder := GetStreamCommandDecoderLimited(TypeProtobuf, bytes.NewReader(append(prefix, 0x01, 0x02)), 1<<20)
	defer PutStreamCommandDecoder(TypeProtobuf, decoder)
	cmd, _, err := decoder.Decode()
	require.Nil(t, cmd)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// The stream decoders must refuse to be constructed without a positive message
// size limit: an unbounded decoder over untrusted input can be driven to allocate
// arbitrary memory by a single frame, so the misconfiguration fails loudly rather
// than silently. See GHSA-4r3x-2rwr-6w65.
func TestStreamCommandDecoder_PanicsOnNonPositiveLimit(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		require.Panics(t, func() {
			GetStreamCommandDecoderLimited(TypeProtobuf, bytes.NewReader(nil), limit)
		})
		require.Panics(t, func() {
			GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(nil), limit)
		})
		require.Panics(t, func() {
			NewProtobufStreamCommandDecoder(bytes.NewReader(nil), limit)
		})
		require.Panics(t, func() {
			NewJSONStreamCommandDecoder(bytes.NewReader(nil), limit)
		})
	}
}

func TestGetByteBuffer_NonPositiveLength(t *testing.T) {
	for _, length := range []int{0, -1, math.MinInt} {
		require.NotPanics(t, func() {
			bb := getByteBuffer(length)
			require.NotNil(t, bb)
			require.Empty(t, bb.B)
		})
	}
}

// makeJSONCommandFrame builds a single `\n` terminated JSON command whose
// channel is chanSize bytes long.
func makeJSONCommandFrame(tb testing.TB, chanSize int) []byte {
	tb.Helper()
	data, err := NewJSONCommandEncoder().Encode(&Command{
		Id:      1,
		Publish: &PublishRequest{Channel: strings.Repeat("a", chanSize), Data: Raw(`{}`)},
	})
	require.NoError(tb, err)
	encoder := GetDataEncoder(TypeJSON)
	require.NoError(tb, encoder.Encode(data))
	frame := encoder.Finish()
	PutDataEncoder(TypeJSON, encoder)
	return frame
}

// JSONStreamCommandDecoder reads into a buffer reused across Decode calls, so
// commands decoded earlier must not be affected by later ones.
func TestStreamingDecode_JSON_NoAliasing(t *testing.T) {
	// Sizes straddling the bufio.Reader buffer, to cover both the fast path and
	// the accumulating path of readLine.
	for _, size := range []int{16, 4000, 4090, 4096, 5000, 10000} {
		channels := make([]string, 3)
		encoder := GetDataEncoder(TypeJSON)
		for i := 0; i < len(channels); i++ {
			channels[i] = strings.Repeat(string(rune('a'+i)), size)
			data, err := NewJSONCommandEncoder().Encode(&Command{
				Id:      uint32(i + 1),
				Publish: &PublishRequest{Channel: channels[i], Data: Raw(`{"k":1}`)},
			})
			require.NoError(t, err)
			require.NoError(t, encoder.Encode(data))
		}
		frame := encoder.Finish()
		PutDataEncoder(TypeJSON, encoder)

		dec := GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(frame), 1<<20)
		var decoded []*Command
		for {
			cmd, _, err := dec.Decode()
			if cmd != nil {
				decoded = append(decoded, cmd)
			}
			if err != nil {
				break
			}
		}
		PutStreamCommandDecoder(TypeJSON, dec)

		// Checked after every Decode call, so aliasing would show up here.
		require.Len(t, decoded, len(channels))
		for i, cmd := range decoded {
			require.Equal(t, uint32(i+1), cmd.Id)
			require.Equal(t, channels[i], cmd.Publish.Channel)
			require.Equal(t, `{"k":1}`, string(cmd.Publish.Data))
		}
	}
}

func TestStreamingDecode_JSON_MessageLimitBoundary(t *testing.T) {
	for _, chanSize := range []int{10, 4000, 4090, 4096, 4100, 9000} {
		frame := makeJSONCommandFrame(t, chanSize)
		size := int64(len(frame))

		// A limit equal to the command size must still decode it.
		dec := GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(frame), size)
		cmd, _, err := dec.Decode()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
		}
		require.NotNil(t, cmd)
		require.Equal(t, strings.Repeat("a", chanSize), cmd.Publish.Channel)
		PutStreamCommandDecoder(TypeJSON, dec)

		// One byte less must be rejected.
		dec = GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(frame), size-1)
		_, _, err = dec.Decode()
		require.ErrorIs(t, err, ErrMessageTooLarge)
		PutStreamCommandDecoder(TypeJSON, dec)
	}
}

// A decoder taken from the pool must not carry over the read buffer state of a
// previously decoded, much larger command.
func TestStreamingDecode_JSON_PoolReuse(t *testing.T) {
	large := makeJSONCommandFrame(t, 9000)
	small := makeJSONCommandFrame(t, 10)
	for i := 0; i < 50; i++ {
		frame, chanSize := large, 9000
		if i%2 == 1 {
			frame, chanSize = small, 10
		}
		dec := GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(frame), 1<<20)
		cmd, _, err := dec.Decode()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
		}
		require.NotNil(t, cmd)
		require.Equal(t, strings.Repeat("a", chanSize), cmd.Publish.Channel)
		PutStreamCommandDecoder(TypeJSON, dec)
	}
}

// A message whose body fails to unmarshal must still be consumed, so that a
// caller which keeps decoding sees the next message instead of this body again.
func TestStreamingDecode_Protobuf_AdvancesPastBadMessage(t *testing.T) {
	badBody := []byte{0x0F} // Field 1 with wire type 7, which is not valid.
	goodBody, err := (&Command{Id: 42}).MarshalVT()
	require.NoError(t, err)

	var frame []byte
	lengthPrefix := make([]byte, binary.MaxVarintLen64)
	for _, body := range [][]byte{badBody, goodBody} {
		n := binary.PutUvarint(lengthPrefix, uint64(len(body)))
		frame = append(frame, lengthPrefix[:n]...)
		frame = append(frame, body...)
	}

	dec := GetStreamCommandDecoderLimited(TypeProtobuf, bytes.NewReader(frame), 1<<20)
	defer PutStreamCommandDecoder(TypeProtobuf, dec)

	_, _, err = dec.Decode()
	require.Error(t, err, "the bad body must fail to unmarshal")

	cmd, _, _ := dec.Decode()
	require.NotNil(t, cmd, "decoder must have advanced past the bad message")
	require.Equal(t, uint32(42), cmd.Id)
}
