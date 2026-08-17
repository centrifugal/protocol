package protocol

import (
	"bytes"
	"testing"
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
