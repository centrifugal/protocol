//go:build go1.18
// +build go1.18

package protocol

import (
	"testing"
)

func FuzzJSONDecode(f *testing.F) {
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
