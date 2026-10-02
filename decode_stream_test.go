package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tests of the decoders which read commands from a stream: the transports
// which do not deliver a frame at once (HTTP streaming, WebTransport) use
// them. A stream comes from an untrusted client, and the size limit is what
// keeps it from making the server hold on to arbitrary amounts of memory.

func newStream(tb testing.TB, protoType Type, frame []byte, limit int64) StreamCommandDecoder {
	tb.Helper()
	return GetStreamCommandDecoderLimited(protoType, bytes.NewReader(frame), limit)
}

func TestStreamCommandDecoder(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		t.Run("several commands", func(t *testing.T) {
			cmds := []*Command{publishCommand(1, "a", 10), {Id: 2}, publishCommand(3, "b", 10000)}
			decoder := newStream(t, protoType, commandFrame(t, protoType, cmds...), 1<<20)
			requireCommands(t, cmds, readStream(t, decoder))
			PutStreamCommandDecoder(protoType, decoder)
		})
		t.Run("size of a command", func(t *testing.T) {
			cmds := []*Command{publishCommand(1, "a", 10000), publishCommand(2, "b", 10000)}
			decoder := newStream(t, protoType, commandFrame(t, protoType, cmds...), 1<<20)
			for _, cmd := range cmds {
				_, size, err := decoder.Decode()
				if err != nil {
					require.ErrorIs(t, err, io.EOF)
				}
				// What is reported is what the command took on the wire, the
				// delimiter included for JSON, and with 8 bytes on top of the
				// body for Protobuf.
				encoded := encodeCommand(t, protoType, cmd)
				if protoType == TypeJSON {
					want := len(encoded)
					if cmd.Id == 1 {
						want++ // The newline.
					}
					require.Equal(t, want, size)
				} else {
					body, err := cmd.MarshalCF()
					require.NoError(t, err)
					require.Equal(t, len(body)+8, size)
				}
			}
		})
		t.Run("sizes mixed in a frame", func(t *testing.T) {
			var cmds []*Command
			for i, size := range []int{10, 5000, 70000, 300000, 1_000_000, 20, 100000} {
				cmds = append(cmds, publishCommand(uint32(i+1), "c", size))
			}
			decoder := newStream(t, protoType, commandFrame(t, protoType, cmds...), 50_000_000)
			requireCommands(t, cmds, readStream(t, decoder))
		})
		t.Run("pooled decoder over connections", func(t *testing.T) {
			// A decoder from the pool serves one connection after another,
			// with commands whose sizes jump across the buffer boundaries.
			sizes := []int{100, 1_000_000, 50, 4096, 300000, 1, 5_000_000, 4097, 65536}
			for round := 0; round < 3; round++ {
				var cmds []*Command
				for i, size := range sizes {
					cmds = append(cmds, publishCommand(uint32(round*100+i+1), "c", size))
				}
				decoder := newStream(t, protoType, commandFrame(t, protoType, cmds...), 10_000_000)
				requireCommands(t, cmds, readStream(t, decoder))
				PutStreamCommandDecoder(protoType, decoder)
			}
		})
		t.Run("message size limit", func(t *testing.T) {
			decoder := newStream(t, protoType, commandFrame(t, protoType, publishCommand(1, "c", 10000)), 100)
			_, _, err := decoder.Decode()
			require.ErrorIs(t, err, ErrMessageTooLarge)
			// A pooled decoder takes the limit it is given next.
			PutStreamCommandDecoder(protoType, decoder)
			decoder = newStream(t, protoType, commandFrame(t, protoType, publishCommand(1, "c", 10000)), 1<<20)
			require.Len(t, readStream(t, decoder), 1)
		})
		t.Run("limit which does not fit a buffer", func(t *testing.T) {
			// math.MaxInt64 must not overflow the reading budget, which
			// would drop every command silently.
			for _, limit := range []int64{1 << 20, math.MaxInt64 - 1, math.MaxInt64} {
				cmds := []*Command{publishCommand(1, "c", 10)}
				requireCommands(t, cmds, readStream(t, newStream(t, protoType, commandFrame(t, protoType, cmds...), limit)))
			}
		})
		t.Run("limit must be positive", func(t *testing.T) {
			// A decoder without a limit could be made to allocate any amount
			// of memory by one frame, so asking for one fails loudly. See
			// GHSA-4r3x-2rwr-6w65.
			for _, limit := range []int64{0, -1} {
				require.Panics(t, func() { newStream(t, protoType, nil, limit) })
			}
			require.Panics(t, func() { NewJSONStreamCommandDecoder(bytes.NewReader(nil), 0) })
			require.Panics(t, func() { NewProtobufStreamCommandDecoder(bytes.NewReader(nil), 0) })
		})
	})
}

// The limit is checked for every command of a frame, also for one whose
// delimiter was in the buffer already: it comes back with a nil error, and
// used to skip the check.
func TestJSONStreamCommandDecoder_LimitAppliesToEveryCommand(t *testing.T) {
	const limit = 100
	command := func(length int) string {
		prefix, suffix := `{"id":1,"publish":{"channel":"`, `"}}`
		return prefix + strings.Repeat("x", length-len(prefix)-len(suffix)) + suffix
	}
	for _, length := range []int{limit - 1, limit, limit + 1, limit + 50, limit + 90} {
		for position, frame := range map[string]string{
			"alone": command(length) + "\n",
			"after": `{"id":9}` + "\n" + command(length) + "\n",
		} {
			decoder := newStream(t, TypeJSON, []byte(frame), limit)
			if position == "after" {
				_, _, _ = decoder.Decode()
			}
			cmd, _, err := decoder.Decode()
			PutStreamCommandDecoder(TypeJSON, decoder)
			if length > limit {
				require.Nil(t, cmd, "%s: %d bytes with limit %d", position, length, limit)
				require.ErrorIs(t, err, ErrMessageTooLarge, "%s: %d bytes", position, length)
			} else {
				require.NotNil(t, cmd, "%s: %d bytes with limit %d", position, length, limit)
			}
		}
	}
}

// A limit equal to the size of a command lets it through, one byte less does
// not: around the size of the buffer of the reader too. The delimiter after
// a command is not a part of it, whether it is a newline or CRLF.
func TestJSONStreamCommandDecoder_LimitBoundary(t *testing.T) {
	for _, size := range []int{10, 4000, 4090, 4096, 4100, 9000} {
		cmd := &Command{Id: 1, Publish: &PublishRequest{Channel: strings.Repeat("a", size), Data: Raw(`{}`)}}
		frame := commandFrame(t, TypeJSON, cmd)
		for _, delimiter := range []string{"", "\n", "\r\n"} {
			delimited := append(bytes.Clone(frame), delimiter...)
			requireCommands(t, []*Command{cmd}, readStream(t, newStream(t, TypeJSON, delimited, int64(len(frame)))))

			_, _, err := newStream(t, TypeJSON, delimited, int64(len(frame))-1).Decode()
			require.ErrorIs(t, err, ErrMessageTooLarge, "size %d, delimiter %q", size, delimiter)
		}
	}
}

// The JSON decoder reads into a buffer it reuses from command to command, so a
// command decoded earlier must not change when the next one is read.
func TestJSONStreamCommandDecoder_NoAliasing(t *testing.T) {
	// Around the size of the buffer of the reader, for both ways a line is
	// read: from the buffer, and put together from several reads.
	for _, size := range []int{16, 4000, 4090, 4096, 5000, 10000} {
		var cmds []*Command
		for i := 0; i < 3; i++ {
			cmds = append(cmds, &Command{Id: uint32(i + 1), Publish: &PublishRequest{
				Channel: strings.Repeat(string(rune('a'+i)), size), Data: Raw(`{"k":1}`),
			}})
		}
		decoded := readStream(t, newStream(t, TypeJSON, commandFrame(t, TypeJSON, cmds...), 1<<20))
		// Checked once all are read, which is when aliasing would show.
		requireCommands(t, cmds, decoded)
	}
}

// A decoder from the pool must not keep the state of a much larger command it
// decoded before.
func TestJSONStreamCommandDecoder_PoolReuse(t *testing.T) {
	for i := 0; i < 50; i++ {
		size := 9000
		if i%2 == 1 {
			size = 10
		}
		cmds := []*Command{{Id: 1, Publish: &PublishRequest{Channel: strings.Repeat("a", size), Data: Raw(`{}`)}}}
		decoder := newStream(t, TypeJSON, commandFrame(t, TypeJSON, cmds...), 1<<20)
		requireCommands(t, cmds, readStream(t, decoder))
		PutStreamCommandDecoder(TypeJSON, decoder)
	}
}

// A buffer grown for a command larger than maxRetainedLineBuffer is not kept,
// neither by a decoder which goes on decoding nor by one back in the pool.
func TestJSONStreamCommandDecoder_DropsOversizedBuffer(t *testing.T) {
	const oversized = maxRetainedLineBuffer + 1024
	frame := commandFrame(t, TypeJSON,
		&Command{Id: 1, Publish: &PublishRequest{Channel: strings.Repeat("a", oversized), Data: Raw(`{}`)}},
		&Command{Id: 2, Publish: &PublishRequest{Channel: strings.Repeat("a", 10), Data: Raw(`{}`)}},
	)
	decoder := newStream(t, TypeJSON, frame, 1<<20)
	jsonDecoder, ok := decoder.(*JSONStreamCommandDecoder)
	require.True(t, ok)

	cmd, _, err := jsonDecoder.Decode()
	require.NoError(t, err)
	require.Len(t, cmd.Publish.Channel, oversized)
	require.NotNil(t, jsonDecoder.buf, "the large command must have been put together in the buffer")
	require.Greater(t, cap(jsonDecoder.buf.B), maxRetainedLineBuffer)

	cmd, _, _ = jsonDecoder.Decode()
	require.NotNil(t, cmd)
	require.Len(t, cmd.Publish.Channel, 10)
	require.Nil(t, jsonDecoder.buf, "the oversized buffer must not be kept")

	PutStreamCommandDecoder(TypeJSON, decoder)
	require.Nil(t, jsonDecoder.buf)
}

// The length of a Protobuf message is declared before its body, by the other
// side. It is not an allocation size: a few bytes must not make the decoder
// allocate, or panic on, any size they declare. See also
// FuzzProtobufStreamDecode.
func TestProtobufStreamCommandDecoder_HostileLength(t *testing.T) {
	for _, length := range []uint64{math.MaxInt32 + 1, 1 << 40, 1 << 62, math.MaxUint64} {
		decoder := newStream(t, TypeProtobuf, binary.AppendUvarint(nil, length), 1<<20)
		cmd, size, err := decoder.Decode()
		require.Nil(t, cmd)
		require.Zero(t, size)
		require.ErrorIs(t, err, ErrMessageTooLarge, "length %d", length)
		PutStreamCommandDecoder(TypeProtobuf, decoder)
	}

	// A length within the limit is fine until the body turns out to be
	// shorter.
	frame := append(binary.AppendUvarint(nil, 1024), 0x01, 0x02)
	cmd, _, err := newStream(t, TypeProtobuf, frame, 1<<20).Decode()
	require.Nil(t, cmd)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// What is allocated for a large message follows the bytes which arrive, not
// the length declared for them.
func TestProtobufStreamCommandDecoder_AllocatesAsDataArrives(t *testing.T) {
	const limit = 1 << 30
	frame := append(binary.AppendUvarint(nil, limit), make([]byte, 1000)...)
	decoder := NewProtobufStreamCommandDecoder(bytes.NewReader(frame), limit)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	cmd, _, err := decoder.Decode()
	runtime.ReadMemStats(&after)
	require.Nil(t, cmd)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(4*maxBufferLength))
}

// A message too large for a pooled buffer is read in steps, and takes exactly
// its bytes from the stream.
func TestProtobufStreamCommandDecoder_LargeMessage(t *testing.T) {
	for _, size := range []int{maxBufferLength - 1, maxBufferLength, maxBufferLength + 1, 3*maxBufferLength + 17, 5 << 20} {
		cmds := []*Command{publishCommand(7, "news", size), publishCommand(8, "news", size)}
		decoder := NewProtobufStreamCommandDecoder(bytes.NewReader(commandFrame(t, TypeProtobuf, cmds...)), 16<<20)
		requireCommands(t, cmds, readStream(t, decoder))
	}
}

// A message whose body fails to decode is consumed all the same, so that a
// caller which goes on sees the next message and not the same body again.
func TestProtobufStreamCommandDecoder_AdvancesPastBadMessage(t *testing.T) {
	badBody := []byte{0x0F} // Field 1 with wire type 7, which does not exist.
	frame := append(binary.AppendUvarint(nil, uint64(len(badBody))), badBody...)
	frame = append(frame, commandFrame(t, TypeProtobuf, &Command{Id: 42})...)

	decoder := newStream(t, TypeProtobuf, frame, 1<<20)
	_, _, err := decoder.Decode()
	require.Error(t, err)
	cmd, _, _ := decoder.Decode()
	require.NotNil(t, cmd, "the decoder must have gone past the bad message")
	require.Equal(t, uint32(42), cmd.Id)
}
