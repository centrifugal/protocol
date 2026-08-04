package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strconv"
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
	dec := GetStreamCommandDecoder(protoType, bytes.NewReader(frame))
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
			name:             "no limit",
			messageSizeLimit: 0,
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
	decoder = GetStreamCommandDecoderLimited(TypeJSON, bytes.NewBufferString(data), 0)
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
			// No message size limit configured - the hostile length must still
			// be rejected rather than allocated or panicked on.
			decoder := GetStreamCommandDecoder(TypeProtobuf, bytes.NewReader(uvarint(tt.msgLength)))
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

	decoder := GetStreamCommandDecoder(TypeProtobuf, bytes.NewReader(append(prefix, 0x01, 0x02)))
	defer PutStreamCommandDecoder(TypeProtobuf, decoder)
	cmd, _, err := decoder.Decode()
	require.Nil(t, cmd)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
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
