package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"unicode/utf8"

	"github.com/centrifugal/protocol/cfjson"
)

// Fuzz targets. Everything a decoder of this package reads may come from the
// other side of a connection, so decoding must end, without a panic, on any
// input; what the targets check on top of that is said at each of them. The
// nightly workflow runs every target, see .github/workflows/fuzz.yml.

// fuzzCommandDecoder decodes a frame to its end. Every successful Decode
// takes at least a byte of the frame, so it takes no more than len(frame)+1
// calls to get to an error.
func fuzzCommandDecoder(t *testing.T, protoType Type, frame []byte) {
	decoder := GetCommandDecoder(protoType, frame)
	defer PutCommandDecoder(protoType, decoder)
	for i := 0; i <= len(frame); i++ {
		cmd, err := decoder.Decode()
		if err == nil && cmd == nil {
			t.Fatalf("no command and no error for %q", frame)
		}
		if cmd != nil {
			requireEncodes(t, protoType, cmd)
		}
		if err != nil {
			return
		}
	}
	t.Fatalf("decoding %q did not end", frame)
}

// requireEncodes checks that what was decoded encodes again.
func requireEncodes(t *testing.T, protoType Type, cmd *Command) {
	if protoType == TypeJSON {
		if data := cmd.AppendJSON(nil); !cfjson.Valid(data) {
			t.Fatalf("decoded command encodes to invalid JSON %q", data)
		}
		return
	}
	if _, err := cmd.MarshalCF(); err != nil {
		t.Fatalf("decoded command does not encode: %v", err)
	}
}

func FuzzJSONCommandDecode(f *testing.F) {
	f.Add([]byte(`{"id":1,"connect":{"token":"t"}}`))
	f.Add([]byte(`{"id":1}` + "\n" + `{"id":2,"subscribe":{"channel":"c"}}` + "\n"))
	f.Fuzz(func(t *testing.T, frame []byte) {
		fuzzCommandDecoder(t, TypeJSON, frame)
	})
}

func FuzzProtobufCommandDecode(f *testing.F) {
	f.Add([]byte{0x02, 0x08, 0x01})
	f.Add([]byte{0x02, 0x08, 0x01, 0x02, 0x08, 0x02})
	f.Fuzz(func(t *testing.T, frame []byte) {
		fuzzCommandDecoder(t, TypeProtobuf, frame)
	})
}

// Replies are decoded by clients and come from a server.
func FuzzProtobufReplyDecode(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte{0x02, 0x08, 0x01})
	f.Fuzz(func(t *testing.T, frame []byte) {
		decoder := NewProtobufReplyDecoder(frame)
		for i := 0; i <= len(frame); i++ {
			if _, err := decoder.Decode(); err != nil {
				return
			}
		}
		t.Fatalf("decoding %q did not end", frame)
	})
}

// fuzzStreamDecoder decodes a stream to its end. A stream decoder reads the
// length of a Protobuf message before the message, and that length must be
// held to the limit, not taken for an allocation size.
func fuzzStreamDecoder(t *testing.T, protoType Type, data []byte) {
	decoder := GetStreamCommandDecoderLimited(protoType, bytes.NewReader(data), 1<<20)
	defer PutStreamCommandDecoder(protoType, decoder)
	for i := 0; i <= len(data); i++ {
		cmd, size, err := decoder.Decode()
		if cmd != nil {
			if size <= 0 {
				t.Fatalf("command of size %d", size)
			}
			requireEncodes(t, protoType, cmd)
		}
		if err != nil {
			return
		}
	}
	t.Fatalf("decoding %q did not end", data)
}

func FuzzJSONStreamDecode(f *testing.F) {
	f.Add([]byte(`{"id":1}` + "\n"))
	f.Add([]byte(`{"id":1}` + "\n" + `{"id":2}` + "\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzStreamDecoder(t, TypeJSON, data)
	})
}

func FuzzProtobufStreamDecode(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte{0x02, 0x08, 0x01})
	f.Add([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x40}) // A huge length.
	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzStreamDecoder(t, TypeProtobuf, data)
	})
}

// fuzzJSONRoundTrip checks what decoding of untrusted JSON must be, whatever
// the input: it does not modify the input, copying and zero-copy decoding
// agree, what is accepted is valid JSON and encodes to valid JSON, and that
// decodes back to the very same message.
func fuzzJSONRoundTrip(t *testing.T, data []byte, newMessage func() message) {
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
	if !cfjson.Valid(data[:n]) {
		t.Fatalf("decoded %q which is not valid JSON", data[:n])
	}
	// The copy must not depend on the input anymore.
	for i := range data {
		data[i] = 'X'
	}
	encoded := encodeJSON(msg)
	if !cfjson.Valid(encoded) {
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
	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzJSONRoundTrip(t, data, func() message { return new(Command) })
	})
}

// Replies are decoded by clients and come from a server.
func FuzzJSONReplyRoundTrip(f *testing.F) {
	f.Add([]byte(`{"id":1,"connect":{"client":"c","subs":{"c":{"publications":[{"data":{},"info":{"user":"u","client":"c"},"tags":{"a":"b"}}]}}}}`))
	f.Add([]byte(`{"push":{"channel":"c","pub":{"data":{"a":1},"offset":7}}}`))
	f.Add([]byte(`{"id":2,"error":{"code":100,"message":"internal server error","temporary":true}}`))
	f.Add([]byte(`{"id":3,"history":{"publications":[],"epoch":"e","offset":0},"presence":{"presence":null}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		decoder := NewJSONReplyDecoder(bytes.Clone(data))
		for i := 0; i <= len(data); i++ {
			if _, err := decoder.Decode(); err != nil {
				if !errors.Is(err, io.EOF) {
					// The error stays.
					if _, again := decoder.Decode(); !errors.Is(again, err) {
						t.Fatalf("error %v, then %v", err, again)
					}
				}
				break
			}
		}
		fuzzJSONRoundTrip(t, data, func() message { return new(Reply) })
	})
}

// A payload is bytes of an application, written into a message as they are.
// Whatever they are, a reply is either refused or encoded to exactly what it
// is with a harmless payload in the same place, the payload aside: nothing in
// a payload may add to a message, take from it, or break it.
func FuzzJSONPayloads(f *testing.F) {
	for _, s := range []string{
		`{}`, `{"a":[1,2,{"b":null}]}`, `"text"`, `1.5e3`, `null`, ` {"a":1} `, "{\n\"a\": 1\n}", "", "\n",
		`nope`, `{`, `{"a":1}}`, `1 2`, "\xff", " ", "\"a\nb\"",
		`1,"offset":999`, `{}},"x":{"a":1`, `{},"info":{"user":"admin"}},"x":{"a":1`, `1}},"id":7,"push":{"pub":{"data":2`,
	} {
		for place := range payloadPlaces {
			f.Add(byte(place), []byte(s))
		}
	}
	const marker = `"@@payload@@"`
	f.Fuzz(func(t *testing.T, place byte, raw []byte) {
		build := payloadPlaces[int(place)%len(payloadPlaces)].build
		reply, _ := build(bytes.Clone(raw))
		data, err := NewJSONReplyEncoder().Encode(reply)

		// What is accepted is what encoding/json takes for one JSON value
		// (it does not mind invalid UTF-8, which is not JSON), or nothing: no
		// payload, or newlines, which are dropped.
		blank := len(bytes.ReplaceAll(raw, []byte("\n"), nil)) == 0
		want := blank || (json.Valid(raw) && utf8.Valid(raw))
		if (err == nil) != want {
			t.Fatalf("payload %q: error %v, want accepted = %v", raw, err, want)
		}
		if err != nil {
			return
		}
		if !json.Valid(data) || bytes.IndexByte(data, '\n') >= 0 {
			t.Fatalf("payload %q encoded to %q", raw, data)
		}
		if blank {
			return
		}
		// The same message with a marker for the payload, and the payload put
		// where the marker is.
		reply, _ = build(Raw(marker))
		expected, err := NewJSONReplyEncoder().Encode(reply)
		if err != nil {
			t.Fatal(err)
		}
		expected = bytes.ReplaceAll(expected, []byte(marker), bytes.ReplaceAll(raw, []byte("\n"), nil))
		if !bytes.Equal(data, expected) {
			t.Fatalf("payload %q:\n     got %s\nexpected %s", raw, data, expected)
		}
	})
}

// The Protobuf decoders agree with google.golang.org/protobuf on what is
// acceptable and on the message it holds, see protobuf_official_test.go. An
// input is decoded into a message type picked by its first byte.
func FuzzProtobufMatchesOfficial(f *testing.F) {
	r := rand.New(rand.NewSource(4))
	for n := 0; n < 300; n++ {
		name, m := randomPBMessage(r, 2)
		if data, _ := m.MarshalCF(); len(data) < 1024 {
			f.Add(byte(sort.SearchStrings(messageNames, name)), data)
		}
	}
	f.Add(byte(0), []byte{0xc3, 0x06, 0x08, 0x01, 0xc4, 0x06})
	f.Fuzz(func(t *testing.T, typ byte, data []byte) {
		compareWithOfficial(t, messageNames[int(typ)%len(messageNames)], data)
	})
}
