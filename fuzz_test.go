package protocol

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/centrifugal/protocol/cfjson"
)

func FuzzJSONDecodeSingle(f *testing.F) {
	f.Add([]byte(`{"id": 1, "method": "", "params": {}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		decoder := GetCommandDecoder(TypeJSON, b)
		_, err := decoder.Decode()
		if err != nil {
			t.Skip()
		}
		PutCommandDecoder(TypeJSON, decoder)
	})
}

func FuzzJSONDecodeMultiple(f *testing.F) {
	f.Add([]byte(`{"id": 1, "method": "", "params": {}}
{"id": 2, "method": "", "params": {}}
`))
	f.Fuzz(func(t *testing.T, b []byte) {
		decoder := GetCommandDecoder(TypeJSON, b)
		_, err := decoder.Decode()
		if err != nil {
			t.Skip()
		}
		_, err = decoder.Decode()
		if err != nil {
			t.Skip()
		}
		PutCommandDecoder(TypeJSON, decoder)
	})
}

func FuzzProtobufDecode(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		decoder := GetCommandDecoder(TypeProtobuf, b)
		_, err := decoder.Decode()
		if err != nil {
			t.Skip()
		}
		PutCommandDecoder(TypeProtobuf, decoder)
	})
}

// Replies are decoded on the client side and come from a server, so decoding
// must never panic and must always terminate on arbitrary input.
func FuzzProtobufReplyDecode(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte{0x02, 0x08, 0x01})
	f.Fuzz(func(t *testing.T, b []byte) {
		decoder := NewProtobufReplyDecoder(b)
		// Every successful Decode consumes at least the length prefix byte, so
		// the loop cannot run more than len(b) times before returning an error.
		for i := 0; i <= len(b); i++ {
			if _, err := decoder.Decode(); err != nil {
				return
			}
		}
		t.Fatal("decoder did not terminate")
	})
}

// The stream decoders read a length prefix from an untrusted peer before the
// message body, so they are the ones that must not allocate or panic on it. A
// positive size limit is always configured (the decoders require one); a declared
// length above it must be rejected rather than allocated.
func FuzzProtobufStreamDecode(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte{0x02, 0x08, 0x01})
	f.Add([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x40}) // Huge declared length.
	f.Fuzz(func(t *testing.T, b []byte) {
		decoder := GetStreamCommandDecoderLimited(TypeProtobuf, bytes.NewReader(b), 1<<20)
		defer PutStreamCommandDecoder(TypeProtobuf, decoder)
		// Each successful Decode consumes at least the length prefix byte.
		for i := 0; i <= len(b); i++ {
			if _, _, err := decoder.Decode(); err != nil {
				return
			}
		}
		t.Fatal("decoder did not terminate")
	})
}

func FuzzJSONStreamDecode(f *testing.F) {
	f.Add([]byte(`{"id":1}` + "\n"))
	f.Add([]byte(`{"id":1}` + "\n" + `{"id":2}` + "\n"))
	f.Fuzz(func(t *testing.T, b []byte) {
		decoder := GetStreamCommandDecoderLimited(TypeJSON, bytes.NewReader(b), 1<<20)
		defer PutStreamCommandDecoder(TypeJSON, decoder)
		for i := 0; i <= len(b); i++ {
			if _, _, err := decoder.Decode(); err != nil {
				return
			}
		}
		t.Fatal("decoder did not terminate")
	})
}

// fuzzJSONRoundTrip checks the properties decoding of untrusted JSON must
// have, whatever the input: it never panics and never modifies the input,
// copying and zero-copy decoding agree, whatever was accepted encodes to valid
// JSON, and that JSON decodes back to the very same message.
func fuzzJSONRoundTrip(t *testing.T, data []byte, newMessage func() jsonMessage) {
	input := bytes.Clone(data)
	msg, zeroCopy := newMessage(), newMessage()
	n := msg.DecodeJSON(data, cfjson.SkipSpace(data, 0), 0)
	nz := zeroCopy.DecodeJSON(data, cfjson.SkipSpace(data, 0), cfjson.ZeroCopy)
	if !bytes.Equal(input, data) {
		t.Fatalf("decoding modified the input %q", input)
	}
	if n != nz {
		t.Fatalf("decoding %q returned %d, with ZeroCopy %d", input, n, nz)
	}
	if n < 0 {
		if err := cfjson.Error(data, n); err == nil {
			t.Fatalf("no error for %d", n)
		}
		return
	}
	if n > len(data) {
		t.Fatalf("decoding %q returned %d", input, n)
	}
	if !reflect.DeepEqual(msg, zeroCopy) {
		t.Fatalf("decoding %q with ZeroCopy gives a different message", input)
	}
	// What was decoded is valid JSON, so it must be accepted as a whole too.
	if !cfjson.Valid(data[:n]) {
		t.Fatalf("decoded %q which is not valid JSON", data[:n])
	}
	// The copy must not depend on the input anymore.
	for i := range data {
		data[i] = 'X'
	}
	encoded := encodeJSON(msg)
	if err := isValidJSON(encoded); err != nil {
		t.Fatalf("input %q encoded to invalid JSON %q", input, encoded)
	}
	first := newMessage()
	if n := first.DecodeJSON(encoded, 0, 0); n != len(encoded) {
		t.Fatalf("own output %q decoded with %d", encoded, n)
	}
	second := newMessage()
	again := encodeJSON(first)
	if n := second.DecodeJSON(again, 0, 0); n != len(again) {
		t.Fatalf("own output %q decoded with %d", again, n)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("round trip of %q is not stable: %q then %q", input, encoded, again)
	}
}

func FuzzJSONCommandRoundTrip(f *testing.F) {
	f.Add([]byte(`{"id":1,"connect":{"token":"t","data":{"a":[1,2]},"subs":{"c":{"recover":true,"offset":5}},"headers":{"k":"v"}}}`))
	f.Add([]byte(`{"id":2,"subscribe":{"channel":"c","tf":{"op":"and","nodes":[{"op":"eq","key":"k","val":"v"}]}}}`))
	f.Add([]byte(`{"id":3,"publish":{"channel":"\u00e9\ud83d\ude00","data":"\n"}}`))
	f.Add([]byte(`{"ID":4,"rpc":{"method":"m","data":null},"unknown":[{"a":1e5}]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		fuzzJSONRoundTrip(t, b, func() jsonMessage { return new(Command) })
	})
}

// Replies are decoded on the client side and come from a server.
func FuzzJSONReplyRoundTrip(f *testing.F) {
	f.Add([]byte(`{"id":1,"connect":{"client":"c","subs":{"c":{"publications":[{"data":{},"info":{"user":"u","client":"c"},"tags":{"a":"b"}}]}}}}`))
	f.Add([]byte(`{"push":{"channel":"c","pub":{"data":{"a":1},"offset":7}}}`))
	f.Add([]byte(`{"id":2,"error":{"code":100,"message":"internal server error","temporary":true}}`))
	f.Add([]byte(`{"id":3,"history":{"publications":[],"epoch":"e","offset":0},"presence":{"presence":null}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		fuzzJSONRoundTrip(t, b, func() jsonMessage { return new(Reply) })

		decoder := NewJSONReplyDecoder(b)
		for {
			if _, err := decoder.Decode(); err != nil {
				break
			}
		}
	})
}
