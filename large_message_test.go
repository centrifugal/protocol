package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

// randomPayload returns n raw bytes, for use with the Protobuf transport
// where Data is an opaque byte slice.
func randomPayload(t *testing.T, n int, seed int64) []byte {
	t.Helper()
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + r.Intn(26))
	}
	return b
}

// randomJSONPayload returns a valid JSON document of roughly n bytes: a JSON
// string of n arbitrary characters. Command.Publish.Data is of type Raw,
// which is embedded verbatim into the JSON output (see raw.go), so on the
// JSON transport it must already be valid JSON - unlike Protobuf, where Data
// is an opaque byte slice.
func randomJSONPayload(t *testing.T, n int, seed int64) []byte {
	t.Helper()
	r := rand.New(rand.NewSource(seed))
	s := make([]byte, n)
	for i := range s {
		s[i] = byte('a' + r.Intn(26))
	}
	b, err := json.Marshal(string(s))
	if err != nil {
		t.Fatalf("marshal json payload: %v", err)
	}
	return b
}

// largeMessageSizes covers every size boundary relevant to allocation and
// buffering logic in this package: the bufio.Reader default buffer (4096),
// maxRetainedLineBuffer (65536), maxBufferLength / the byte pool ceiling
// (262144), and a couple of multi-megabyte payloads.
func largeMessageSizes() []int {
	return []int{
		0, 1, 100,
		4095, 4096, 4097,
		65535, 65536, 65537,
		262143, 262144, 262145,
		1_000_000,
		5_000_000,
	}
}

func TestLargeMessage_WholeFrame_JSON_RoundTrip(t *testing.T) {
	for _, n := range largeMessageSizes() {
		n := n
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			payload := randomJSONPayload(t, n, int64(n))
			cmd := &Command{Id: 42, Publish: &PublishRequest{Channel: "chan", Data: payload}}
			enc := NewJSONCommandEncoder()
			data, err := enc.Encode(cmd)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			dec := NewJSONCommandDecoder(data)
			got, err := dec.Decode()
			if err != nil && err != io.EOF {
				t.Fatalf("decode: %v", err)
			}
			if got.Id != 42 {
				t.Fatalf("id mismatch")
			}
			if !bytes.Equal(got.Publish.Data, payload) {
				t.Fatalf("payload mismatch: len got=%d want=%d", len(got.Publish.Data), len(payload))
			}
		})
	}
}

func TestLargeMessage_WholeFrame_Protobuf_RoundTrip(t *testing.T) {
	for _, n := range largeMessageSizes() {
		n := n
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			payload := randomPayload(t, n, int64(n)+1)
			cmd := &Command{Id: 42, Publish: &PublishRequest{Channel: "chan", Data: payload}}
			enc := NewProtobufCommandEncoder()
			data, err := enc.Encode(cmd)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			dec := NewProtobufCommandDecoder(data)
			got, err := dec.Decode()
			if err != nil && err != io.EOF {
				t.Fatalf("decode: %v", err)
			}
			if got.Id != 42 {
				t.Fatalf("id mismatch")
			}
			if !bytes.Equal(got.Publish.Data, payload) {
				t.Fatalf("payload mismatch: len got=%d want=%d", len(got.Publish.Data), len(payload))
			}
		})
	}
}

// TestLargeMessage_MixedBatch_JSON builds a frame with several commands of
// varying (including large) sizes and decodes them all via the whole-frame
// JSONCommandDecoder, then again via the streaming JSONStreamCommandDecoder
// (simulating a decoder handling a real connection), checking both against
// the same expected payloads.
func TestLargeMessage_MixedBatch_JSON(t *testing.T) {
	szs := []int{10, 5000, 70000, 300000, 1_000_000, 20, 100000}
	payloads := make([][]byte, len(szs))
	enc := NewJSONCommandEncoder()
	var frame []byte
	for i, n := range szs {
		payloads[i] = randomJSONPayload(t, n, int64(i)*7+1)
		cmd := &Command{Id: uint32(i + 1), Publish: &PublishRequest{Channel: "c", Data: payloads[i]}}
		data, err := enc.Encode(cmd)
		if err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
		if i > 0 {
			frame = append(frame, '\n')
		}
		frame = append(frame, data...)
	}

	// Whole-frame decoder.
	dec := NewJSONCommandDecoder(frame)
	for i := range szs {
		cmd, err := dec.Decode()
		if err != nil && err != io.EOF {
			t.Fatalf("whole-frame decode %d: %v", i, err)
		}
		if cmd.Id != uint32(i+1) {
			t.Fatalf("whole-frame id mismatch at %d: got %d", i, cmd.Id)
		}
		if !bytes.Equal(cmd.Publish.Data, payloads[i]) {
			t.Fatalf("whole-frame payload mismatch at %d: len got=%d want=%d", i, len(cmd.Publish.Data), len(payloads[i]))
		}
	}

	// Streaming decoder, with a generous limit.
	r := bytes.NewReader(frame)
	sdec := NewJSONStreamCommandDecoder(r, 50_000_000)
	for i := range szs {
		cmd, n, err := sdec.Decode()
		if err != nil && err != io.EOF {
			t.Fatalf("stream decode %d: %v", i, err)
		}
		if cmd.Id != uint32(i+1) {
			t.Fatalf("stream id mismatch at %d: got %d", i, cmd.Id)
		}
		if !bytes.Equal(cmd.Publish.Data, payloads[i]) {
			t.Fatalf("stream payload mismatch at %d: len got=%d want=%d n=%d", i, len(cmd.Publish.Data), len(payloads[i]), n)
		}
	}
}

// TestLargeMessage_MixedBatch_Protobuf mirrors the JSON test above for the
// protobuf streaming decoder, which has separate buffering logic (varint
// length prefix + Peek fast path + scratch buffer via the byte pool).
func TestLargeMessage_MixedBatch_Protobuf(t *testing.T) {
	szs := []int{10, 5000, 70000, 300000, 1_000_000, 20, 100000}
	payloads := make([][]byte, len(szs))
	enc := NewProtobufCommandEncoder()
	var frame []byte
	for i, n := range szs {
		payloads[i] = randomPayload(t, n, int64(i)*13+3)
		cmd := &Command{Id: uint32(i + 1), Publish: &PublishRequest{Channel: "c", Data: payloads[i]}}
		data, err := enc.Encode(cmd)
		if err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
		frame = append(frame, data...)
	}

	dec := NewProtobufCommandDecoder(frame)
	for i := range szs {
		cmd, err := dec.Decode()
		if err != nil && err != io.EOF {
			t.Fatalf("whole-frame decode %d: %v", i, err)
		}
		if cmd.Id != uint32(i+1) {
			t.Fatalf("whole-frame id mismatch at %d: got %d", i, cmd.Id)
		}
		if !bytes.Equal(cmd.Publish.Data, payloads[i]) {
			t.Fatalf("whole-frame payload mismatch at %d: len got=%d want=%d", i, len(cmd.Publish.Data), len(payloads[i]))
		}
	}

	r := bytes.NewReader(frame)
	sdec := NewProtobufStreamCommandDecoder(r, 50_000_000)
	for i := range szs {
		cmd, n, err := sdec.Decode()
		if err != nil && err != io.EOF {
			t.Fatalf("stream decode %d: %v", i, err)
		}
		if cmd.Id != uint32(i+1) {
			t.Fatalf("stream id mismatch at %d: got %d", i, cmd.Id)
		}
		if !bytes.Equal(cmd.Publish.Data, payloads[i]) {
			t.Fatalf("stream payload mismatch at %d: len got=%d want=%d n=%d", i, len(cmd.Publish.Data), len(payloads[i]), n)
		}
	}
}

// TestLargeMessage_PooledDecoderReuse_Protobuf simulates real server usage: a
// decoder taken from the pool via GetStreamCommandDecoderLimited, used for
// one connection's lifetime, decoding a sequence of commands whose sizes
// jump around across all the relevant boundaries, repeated across several
// simulated connections to exercise buffer reuse/trimming between them.
func TestLargeMessage_PooledDecoderReuse_Protobuf(t *testing.T) {
	szs := []int{100, 1_000_000, 50, 4096, 300000, 1, 5_000_000, 4097, 65536}
	for round := 0; round < 3; round++ {
		var frame []byte
		payloads := make([][]byte, len(szs))
		enc := NewProtobufCommandEncoder()
		for i, n := range szs {
			payloads[i] = randomPayload(t, n, int64(round)*1000+int64(i))
			cmd := &Command{Id: uint32(i + 1), Publish: &PublishRequest{Channel: "c", Data: payloads[i]}}
			data, err := enc.Encode(cmd)
			if err != nil {
				t.Fatalf("round %d encode %d: %v", round, i, err)
			}
			frame = append(frame, data...)
		}
		r := bytes.NewReader(frame)
		dec := GetStreamCommandDecoderLimited(TypeProtobuf, r, 10_000_000)
		for i := range szs {
			cmd, _, err := dec.Decode()
			if err != nil && err != io.EOF {
				t.Fatalf("round %d decode %d: %v", round, i, err)
			}
			if cmd.Id != uint32(i+1) {
				t.Fatalf("round %d id mismatch at %d: got %d", round, i, cmd.Id)
			}
			if !bytes.Equal(cmd.Publish.Data, payloads[i]) {
				t.Fatalf("round %d payload mismatch at %d: len got=%d want=%d", round, i, len(cmd.Publish.Data), len(payloads[i]))
			}
		}
		PutStreamCommandDecoder(TypeProtobuf, dec)
	}
}

// TestLargeMessage_PooledDecoderReuse_JSON is the JSON counterpart of
// TestLargeMessage_PooledDecoderReuse_Protobuf.
func TestLargeMessage_PooledDecoderReuse_JSON(t *testing.T) {
	szs := []int{100, 1_000_000, 50, 4096, 300000, 1, 5_000_000, 4097, 65536}
	for round := 0; round < 3; round++ {
		var frame []byte
		payloads := make([][]byte, len(szs))
		enc := NewJSONCommandEncoder()
		for i, n := range szs {
			payloads[i] = randomJSONPayload(t, n, int64(round)*1000+int64(i)+99)
			cmd := &Command{Id: uint32(i + 1), Publish: &PublishRequest{Channel: "c", Data: payloads[i]}}
			data, err := enc.Encode(cmd)
			if err != nil {
				t.Fatalf("round %d encode %d: %v", round, i, err)
			}
			if i > 0 {
				frame = append(frame, '\n')
			}
			frame = append(frame, data...)
		}
		r := bytes.NewReader(frame)
		dec := GetStreamCommandDecoderLimited(TypeJSON, r, 10_000_000)
		for i := range szs {
			cmd, _, err := dec.Decode()
			if err != nil && err != io.EOF {
				t.Fatalf("round %d decode %d: %v", round, i, err)
			}
			if cmd.Id != uint32(i+1) {
				t.Fatalf("round %d id mismatch at %d: got %d", round, i, cmd.Id)
			}
			if !bytes.Equal(cmd.Publish.Data, payloads[i]) {
				t.Fatalf("round %d payload mismatch at %d: len got=%d want=%d", round, i, len(cmd.Publish.Data), len(payloads[i]))
			}
		}
		PutStreamCommandDecoder(TypeJSON, dec)
	}
}

// TestLargeMessage_DataEncoder_RoundTrip exercises the DataEncoder framing
// used on the server's outbound write path (handler_websocket.go Write /
// WriteMany in centrifuge), for large already-encoded reply/push payloads.
func TestLargeMessage_DataEncoder_RoundTrip(t *testing.T) {
	for _, n := range largeMessageSizes() {
		n := n
		t.Run(fmt.Sprintf("protobuf/n=%d", n), func(t *testing.T) {
			payload := randomPayload(t, n, int64(n)+555)
			pub := &Publication{Data: payload}
			replyEnc := NewProtobufReplyEncoder()
			msg, err := replyEnc.Encode(&Reply{Push: &Push{Channel: "c", Pub: pub}})
			if err != nil {
				t.Fatalf("reply encode: %v", err)
			}
			dataEnc := GetDataEncoder(TypeProtobuf)
			defer PutDataEncoder(TypeProtobuf, dataEnc)
			if err := dataEnc.Encode(msg); err != nil {
				t.Fatalf("data encode: %v", err)
			}
			framed := dataEnc.Finish()

			replyDec := NewProtobufReplyDecoder(framed)
			got, err := replyDec.Decode()
			if err != nil {
				t.Fatalf("reply decode: %v", err)
			}
			if !bytes.Equal(got.Push.Pub.Data, payload) {
				t.Fatalf("payload mismatch: len got=%d want=%d", len(got.Push.Pub.Data), len(payload))
			}
			// Finish (copy) and FinishNoCopy must return the same content.
			if h1, h2 := sha256.Sum256(framed), sha256.Sum256(dataEnc.FinishNoCopy()); h1 != h2 {
				t.Fatalf("Finish vs FinishNoCopy content mismatch")
			}
		})
	}
}
