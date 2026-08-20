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

// decodeAllTo decodes a frame through DecodeTo, reusing a single Command, and
// returns a description of every command it saw. Descriptions are built during
// the loop, since cmd is overwritten by the next call.
func decodeAllTo(tb testing.TB, protoType Type, frame []byte) []string {
	tb.Helper()
	dec := GetStreamCommandDecoderLimited(protoType, bytes.NewReader(frame), 1<<20)
	defer PutStreamCommandDecoder(protoType, dec)
	decTo, ok := dec.(StreamCommandDecoderTo)
	require.True(tb, ok, "decoder must implement StreamCommandDecoderTo")

	var cmd Command
	var out []string
	for {
		size, err := decTo.DecodeTo(&cmd)
		if size > 0 {
			out = append(out, describeCommand(&cmd, size))
		}
		if err != nil {
			require.ErrorIs(tb, err, io.EOF)
			break
		}
	}
	return out
}

// describeCommand renders the fields a reused Command could leak between calls.
func describeCommand(cmd *Command, size int) string {
	s := "id=" + strconv.Itoa(int(cmd.Id)) + " size=" + strconv.Itoa(size)
	if cmd.Publish != nil {
		s += " publish=" + cmd.Publish.Channel + ":" + string(cmd.Publish.Data)
	}
	if cmd.Subscribe != nil {
		s += " subscribe=" + cmd.Subscribe.Channel
	}
	if cmd.Unsubscribe != nil {
		s += " unsubscribe=" + cmd.Unsubscribe.Channel
	}
	if cmd.Ping != nil {
		s += " ping"
	}
	return s
}

// decodeAll decodes a frame through Decode, for comparison with DecodeTo.
func decodeAll(tb testing.TB, protoType Type, frame []byte) []string {
	tb.Helper()
	dec := GetStreamCommandDecoderLimited(protoType, bytes.NewReader(frame), 1<<20)
	defer PutStreamCommandDecoder(protoType, dec)
	var out []string
	for {
		cmd, size, err := dec.Decode()
		if cmd != nil {
			out = append(out, describeCommand(cmd, size))
		}
		if err != nil {
			require.ErrorIs(tb, err, io.EOF)
			break
		}
	}
	return out
}

// mixedCommandFrame builds a frame whose commands set different fields, so a
// Command reused without a reset would carry stale ones over.
func mixedCommandFrame(tb testing.TB, protoType Type) []byte {
	tb.Helper()
	cmds := []*Command{
		{Id: 1, Publish: &PublishRequest{Channel: "ch1", Data: Raw(`{"a":1}`)}},
		{Id: 2, Subscribe: &SubscribeRequest{Channel: "ch2"}},
		{Id: 3, Ping: &PingRequest{}},
		{Id: 4, Unsubscribe: &UnsubscribeRequest{Channel: "ch4"}},
		{Id: 5, Publish: &PublishRequest{Channel: "ch5", Data: Raw(`{"b":2}`)}},
		// Long enough to spill past the bufio.Reader buffer.
		{Id: 6, Publish: &PublishRequest{Channel: strings.Repeat("z", 9000), Data: Raw(`{}`)}},
		{Id: 7, Ping: &PingRequest{}},
	}
	encoder := GetDataEncoder(protoType)
	for _, cmd := range cmds {
		var data []byte
		var err error
		if protoType == TypeProtobuf {
			data, err = cmd.MarshalVT()
		} else {
			data, err = NewJSONCommandEncoder().Encode(cmd)
		}
		require.NoError(tb, err)
		require.NoError(tb, encoder.Encode(data))
	}
	frame := encoder.Finish()
	PutDataEncoder(protoType, encoder)
	return frame
}

// DecodeTo must produce exactly what Decode produces, with no field of a
// previous command surviving into the next one.
func TestStreamingDecodeTo_MatchesDecode(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		frame := mixedCommandFrame(t, protoType)
		require.Equal(t, decodeAll(t, protoType, frame), decodeAllTo(t, protoType, frame))
	}
}

// A Command reused across DecodeTo calls must not keep fields from a command
// decoded earlier.
func TestStreamingDecodeTo_NoStaleFields(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		got := decodeAllTo(t, protoType, mixedCommandFrame(t, protoType))
		require.Len(t, got, 7)
		require.Equal(t, "id=2 subscribe=ch2", trimSize(got[1]), "Publish must not survive into the next command")
		require.Equal(t, "id=3 ping", trimSize(got[2]))
		require.Equal(t, "id=4 unsubscribe=ch4", trimSize(got[3]))
		require.Equal(t, "id=7 ping", trimSize(got[6]))
	}
}

// trimSize drops the size field, which differs between the two protocols.
func trimSize(s string) string {
	start := strings.Index(s, " size=")
	end := strings.Index(s[start+1:], " ")
	if end < 0 {
		return s[:start]
	}
	return s[:start] + s[start+1+end:]
}

// DecodeTo must enforce the message size limit exactly as Decode does.
func TestStreamingDecodeTo_MessageLimit(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		frame := getTestFrame(t, protoType, 10000)
		dec := GetStreamCommandDecoderLimited(protoType, bytes.NewReader(frame), 100)
		var cmd Command
		_, err := dec.(StreamCommandDecoderTo).DecodeTo(&cmd)
		require.ErrorIs(t, err, ErrMessageTooLarge)
		PutStreamCommandDecoder(protoType, dec)
	}
}

// Data reachable from a Command decoded with DecodeTo must stay valid after
// later DecodeTo calls, since a broker may keep a Publication built from it in
// history for as long as it likes.
func TestStreamingDecodeTo_RetainedPayloadsStayValid(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		const numCommands = 12
		encoder := GetDataEncoder(protoType)
		for i := 0; i < numCommands; i++ {
			// Sizes straddle the bufio.Reader buffer so both decode paths are used.
			payload := `{"n":` + strconv.Itoa(i) + `,"pad":"` + strings.Repeat(string(rune('a'+i)), 400*i+1) + `"}`
			cmd := &Command{
				Id:      uint32(i + 1),
				Publish: &PublishRequest{Channel: "ch" + strconv.Itoa(i), Data: Raw(payload)},
			}
			var data []byte
			var err error
			if protoType == TypeProtobuf {
				data, err = cmd.MarshalVT()
			} else {
				data, err = NewJSONCommandEncoder().Encode(cmd)
			}
			require.NoError(t, err)
			require.NoError(t, encoder.Encode(data))
		}
		frame := encoder.Finish()
		PutDataEncoder(protoType, encoder)

		dec := GetStreamCommandDecoderLimited(protoType, bytes.NewReader(frame), 1<<20).(StreamCommandDecoderTo)
		// Retained the way a memory broker retains Publication data.
		type retained struct {
			channel string
			data    Raw
			req     *PublishRequest
		}
		var kept []retained
		var cmd Command
		for {
			size, err := dec.DecodeTo(&cmd)
			// A zero size means cmd was not written to and still holds the
			// previous command, so it must not be looked at.
			if size > 0 && cmd.Publish != nil {
				kept = append(kept, retained{
					channel: cmd.Publish.Channel,
					data:    cmd.Publish.Data,
					req:     cmd.Publish,
				})
			}
			if err != nil {
				require.ErrorIs(t, err, io.EOF)
				break
			}
		}
		PutStreamCommandDecoder(protoType, dec.(StreamCommandDecoder))

		require.Len(t, kept, numCommands)
		for i, k := range kept {
			want := `{"n":` + strconv.Itoa(i) + `,"pad":"` + strings.Repeat(string(rune('a'+i)), 400*i+1) + `"}`
			require.Equal(t, "ch"+strconv.Itoa(i), k.channel, "retained channel %d", i)
			require.Equal(t, want, string(k.data), "retained data %d", i)
			// The request struct itself must be a fresh one per command, not the
			// one the next command was decoded into.
			require.Equal(t, want, string(k.req.Data), "retained request %d", i)
		}
	}
}
