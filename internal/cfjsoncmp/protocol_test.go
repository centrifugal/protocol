package cfjsoncmp

import (
	"strings"
	"testing"

	"github.com/centrifugal/protocol"
	"github.com/centrifugal/protocol/cfjson"
	segmentio "github.com/segmentio/encoding/json"
)

// Users of protocol (centrifuge among them) marshal protocol messages with
// segmentio/encoding/json, which must apply Raw.MarshalJSON and so keep raw
// newlines out of the result. This test lived in protocol until segmentio
// stopped being its dependency.
func TestSegmentioMarshalsRaw(t *testing.T) {
	data := []byte(`{
  "num": "1\n"

}
`)
	res, err := segmentio.Marshal(&protocol.Publication{Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(res), "\n") {
		t.Fatalf("raw newline in %q", res)
	}
}

// centrifuge puts deltas and payloads into JSON messages with segmentio's
// Escape. cfjson.AppendString is meant to replace it there, so the two must
// produce the same bytes.
//
// They do, with one exception: backspace and form feed are written by
// segmentio as \b and \f, and by cfjson as \u0008 and \u000c, which is what
// the writer of protocol always did. Both spellings are the same character
// to any JSON parser.
func TestAppendStringMatchesSegmentioEscape(t *testing.T) {
	check := func(in string) {
		t.Helper()
		got, want := cfjson.AppendString(make([]byte, 0, 64), in), segmentio.Escape(in)
		if strings.ContainsAny(in, "\b\f") {
			gotString, err := cfjson.UnmarshalString(got)
			if err != nil {
				t.Fatalf("%q: %v", in, err)
			}
			wantString, err := cfjson.UnmarshalString(want)
			if err != nil || gotString != wantString {
				t.Fatalf("%q: %s and %s are different strings", in, got, want)
			}
			return
		}
		if string(got) != string(want) {
			t.Fatalf("%q: got %s, segmentio gives %s", in, got, want)
		}
	}
	for c := 0; c < 256; c++ {
		for size := 1; size <= 24; size++ {
			for pos := 0; pos < size; pos++ {
				in := []byte(strings.Repeat("a", size))
				in[pos] = byte(c)
				check(string(in))
			}
		}
	}
	for c1 := 0; c1 < 256; c1++ {
		for c2 := 0; c2 < 256; c2++ {
			check("ab" + string([]byte{byte(c1), byte(c2)}) + "cdefghij")
		}
	}
	for r := rune(0); r <= 0x10FFFF; r++ {
		check("x" + string(r) + "y")
	}
}
