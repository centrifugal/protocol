package protocol

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/centrifugal/protocol/cfprotobuf"
)

var pbStrings = []string{
	"a", "news", "chat:index", "user@example.com", "h\xc3\xa9llo \xf0\x9f\x98\x80",
	strings.Repeat("x", 127), strings.Repeat("y", 128), strings.Repeat("long ", 4000),
}

// fillPB sets the exported fields of a message to random values. Strings are
// valid UTF-8: google.golang.org/protobuf refuses to marshal others.
func fillPB(r *rand.Rand, v reflect.Value, depth, maxMapLen int) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillPB(r, v.Field(i), depth, maxMapLen)
			}
		}
	case reflect.Pointer:
		if depth > 5 || r.Intn(4) != 0 {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		fillPB(r, v.Elem(), depth+1, maxMapLen)
	case reflect.String:
		if r.Intn(3) != 0 {
			v.SetString(pbStrings[r.Intn(len(pbStrings))])
		}
	case reflect.Bool:
		v.SetBool(r.Intn(2) == 0)
	case reflect.Int32, reflect.Int64:
		n := []int64{0, 1, -1, 127, 128, math.MaxInt32, math.MinInt32, math.MaxInt64, math.MinInt64, r.Int63()}[r.Intn(10)]
		if v.Kind() == reflect.Int32 {
			n = int64(int32(n))
		}
		v.SetInt(n)
	case reflect.Uint32, reflect.Uint64:
		n := []uint64{0, 1, 127, 128, 16384, math.MaxUint32, math.MaxUint64, r.Uint64()}[r.Intn(8)]
		if v.Kind() == reflect.Uint32 {
			n = uint64(uint32(n))
		}
		v.SetUint(n)
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			switch r.Intn(4) {
			case 0:
			case 1:
				v.SetBytes([]byte{})
			default:
				v.SetBytes([]byte(pbStrings[r.Intn(len(pbStrings))]))
			}
			return
		}
		if depth > 5 || r.Intn(3) == 0 {
			return
		}
		n := 1 + r.Intn(3)
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		for i := 0; i < n; i++ {
			elem := v.Index(i)
			if elem.Kind() == reflect.Pointer {
				// google.golang.org/protobuf does not marshal a nil element.
				elem.Set(reflect.New(elem.Type().Elem()))
				fillPB(r, elem.Elem(), depth+1, maxMapLen)
			} else {
				fillPB(r, elem, depth+1, maxMapLen)
				if elem.Kind() == reflect.String && elem.Len() == 0 {
					elem.SetString("s")
				}
			}
		}
	case reflect.Map:
		if depth > 5 || r.Intn(3) == 0 {
			return
		}
		v.Set(reflect.MakeMap(v.Type()))
		for n := 1 + r.Intn(maxMapLen); n > 0; n-- {
			key := reflect.New(v.Type().Key()).Elem()
			key.SetString(pbStrings[r.Intn(len(pbStrings)-1)])
			elem := reflect.New(v.Type().Elem()).Elem()
			if elem.Kind() == reflect.Pointer {
				elem.Set(reflect.New(elem.Type().Elem()))
				fillPB(r, elem.Elem(), depth+1, maxMapLen)
			} else {
				fillPB(r, elem, depth+1, maxMapLen)
			}
			v.SetMapIndex(key, elem)
		}
	}
}

func randomPBMessage(r *rand.Rand, maxMapLen int) (string, message) {
	name := messageNames[r.Intn(len(messageNames))]
	m := newMessage(name)
	fillPB(r, reflect.ValueOf(m).Elem(), 0, maxMapLen)
	return name, m
}

// The encoding must be what google.golang.org/protobuf, the reference
// implementation, produces for the same message.
func TestProtobufMarshalMatchesOfficial(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	official := proto.MarshalOptions{Deterministic: true}
	for n := 0; n < 50000; n++ {
		// With one key in a map the encoding is deterministic.
		name, m := randomPBMessage(r, 1)
		want, err := official.Marshal(m)
		require.NoError(t, err)

		got, err := m.MarshalCF()
		require.NoError(t, err)
		require.True(t, bytes.Equal(got, want), "%s:\n got %x\nwant %x", name, got, want)
		require.Equal(t, len(want), m.SizeCF(), name)
		require.Equal(t, len(want), proto.Size(m), name)

		buf := make([]byte, len(want)+8)
		written, err := m.MarshalToCF(buf)
		require.NoError(t, err)
		require.True(t, bytes.Equal(buf[:written], want), name)
		written, err = m.MarshalToSizedBufferCF(buf)
		require.NoError(t, err)
		require.True(t, bytes.Equal(buf[len(buf)-written:], want), name)
	}
}

// With several keys in a map the order of entries is random, so what is
// compared is the message the other implementation decodes from the output.
func TestProtobufMarshalMatchesOfficial_MultiKeyMaps(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for n := 0; n < 20000; n++ {
		name, m := randomPBMessage(r, 4)
		data, err := m.MarshalCF()
		require.NoError(t, err)
		require.Equal(t, proto.Size(m), len(data), name)

		decoded := newMessage(name)
		require.NoError(t, proto.Unmarshal(data, decoded), name)
		require.True(t, proto.Equal(m, decoded), name)
	}
}

// compareWithOfficial decodes data with generated code and with
// google.golang.org/protobuf. They must agree on whether it is acceptable
// and on the message it holds.
func compareWithOfficial(t testing.TB, name string, data []byte) {
	t.Helper()
	input := bytes.Clone(data)
	want, got := newMessage(name), newMessage(name)
	wantErr := proto.Unmarshal(data, want)
	gotErr := got.UnmarshalCF(data)
	if !bytes.Equal(input, data) {
		t.Fatalf("%s: decoding modified the input %x", name, input)
	}
	// The one deliberate difference: messages of a recursive type may nest
	// 10000 levels deep for google.golang.org/protobuf, and cfprotobuf.MaxDepth
	// levels here.
	if wantErr == nil && errors.Is(gotErr, cfprotobuf.ErrTooDeep) {
		return
	}
	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("%s input %x:\n  official error: %v\ngenerated error: %v", name, input, wantErr, gotErr)
	}
	if gotErr != nil {
		return
	}
	// Generated code does not keep the fields a message does not have,
	// google.golang.org/protobuf does.
	if len(got.ProtoReflect().GetUnknown()) != 0 {
		t.Fatalf("%s input %x: unknown fields were kept", name, input)
	}
	dropUnknown(want.ProtoReflect())
	// proto.Equal does not tell a nil message from an empty one, and code
	// which reads the fields of a map value would.
	if where := nilElement(reflect.ValueOf(got)); where != "" {
		t.Fatalf("%s input %x: %s", name, input, where)
	}
	if !proto.Equal(want, got) {
		t.Fatalf("%s input %x: decoded to different messages:\nofficial:  %v\ngenerated: %v", name, input, want, got)
	}
	// And it must encode to the same size, and back to an equal message.
	encoded, err := got.MarshalCF()
	if err != nil {
		t.Fatalf("%s input %x: %v", name, input, err)
	}
	if len(encoded) != proto.Size(want) {
		t.Fatalf("%s input %x: encoded to %d bytes, official size is %d", name, input, len(encoded), proto.Size(want))
	}
	again := newMessage(name)
	if err := proto.Unmarshal(encoded, again); err != nil || !proto.Equal(want, again) {
		t.Fatalf("%s input %x: own output %x does not decode back: %v", name, input, encoded, err)
	}
}

// dropUnknown removes the unknown fields of a message and of every message
// inside of it.
func dropUnknown(m protoreflect.Message) {
	m.SetUnknown(nil)
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			if fd.MapValue().Message() != nil {
				v.Map().Range(func(_ protoreflect.MapKey, elem protoreflect.Value) bool {
					dropUnknown(elem.Message())
					return true
				})
			}
		case fd.IsList():
			if fd.Message() != nil {
				for i := 0; i < v.List().Len(); i++ {
					dropUnknown(v.List().Get(i).Message())
				}
			}
		case fd.Message() != nil:
			dropUnknown(v.Message())
		}
		return true
	})
}

// What a message has of unknown fields, because it was decoded by
// google.golang.org/protobuf for example, is still marshaled.
func TestProtobufUnknownFields(t *testing.T) {
	data := []byte{0x08, 0x07, 0xa0, 0x06, 0x05} // id = 7, and field 100.
	var viaOfficial, viaGenerated Command
	require.NoError(t, proto.Unmarshal(data, &viaOfficial))
	require.NoError(t, viaGenerated.UnmarshalCF(data))
	require.Equal(t, uint32(7), viaGenerated.Id)

	encoded, err := viaOfficial.MarshalCF()
	require.NoError(t, err)
	require.Equal(t, data, encoded)
	require.Equal(t, len(data), viaOfficial.SizeCF())

	encoded, err = viaGenerated.MarshalCF()
	require.NoError(t, err)
	require.Equal(t, []byte{0x08, 0x07}, encoded)
}

func TestProtobufUnmarshalMatchesOfficial(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for n := 0; n < 100000; n++ {
		name, m := randomPBMessage(r, 3)
		data, err := m.MarshalCF()
		require.NoError(t, err)
		compareWithOfficial(t, name, data)
		if len(data) == 0 || len(data) > 4096 {
			continue
		}
		// The same message, damaged in one place.
		pos := r.Intn(len(data))
		switch r.Intn(4) {
		case 0:
			data[pos] = byte(r.Intn(256))
		case 1:
			data = append(data[:pos], data[pos+1:]...)
		case 2:
			data = data[:pos]
		case 3:
			data[pos] ^= 1 << r.Intn(8)
		}
		compareWithOfficial(t, name, data)
	}
}

// An entry of a map is a message of two fields, and a sender is free to write
// it in ways no encoder does: without the key, without the value, with either
// of them more than once.
func TestProtobufMapEntriesMatchOfficial(t *testing.T) {
	tests := []struct{ name, message, input string }{
		// subs: {"a": recover} and then, in the same entry, {offset: 5}.
		{"message value twice", "ConnectRequest", "1a0b0a01611202180112023805"},
		{"no value", "ConnectRequest", "1a030a0161"},
		{"no key", "ConnectRequest", "1a0412021801"},
		{"empty entry", "ConnectRequest", "1a00"},
		{"key twice", "ConnectRequest", "1a0a0a01610a016212021801"},
		{"value before key", "ConnectRequest", "1a07120218010a0161"},
		{"same key in two entries", "ConnectRequest", "1a070a0161120218011a070a016112023805"},
		{"presence without value", "PresenceResult", "0a030a0161"},
		{"presence value twice", "PresenceResult", "0a0d0a016112030a017512030a0176"},
		{"string value twice", "Publication", "22090a016b120161120162"},
		{"string map without value", "Publication", "22030a016b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(strings.ReplaceAll(tt.input, " ", ""))
			require.NoError(t, err)
			// The cases are meant to be accepted: one which is not would
			// compare nothing.
			require.NoError(t, proto.Unmarshal(data, newMessage(tt.message)))
			compareWithOfficial(t, tt.message, data)
		})
	}

	var req ConnectRequest
	data, err := hex.DecodeString("1a0b0a01611202180112023805")
	require.NoError(t, err)
	require.NoError(t, req.UnmarshalCF(data))
	require.True(t, req.Subs["a"].Recover)
	require.Equal(t, uint64(5), req.Subs["a"].Offset)
}

// Groups inside of a group which is skipped may be nested as deep as
// google.golang.org/protobuf lets them, and no deeper: every level is kept
// track of.
func TestProtobufSkippedGroupNesting(t *testing.T) {
	nested := func(levels int) []byte {
		// Field 100, which no message has.
		data := bytes.Repeat([]byte{0xa3, 0x06}, levels)
		return append(data, bytes.Repeat([]byte{0xa4, 0x06}, levels)...)
	}
	var deepest int
	for _, levels := range []int{1, 100, 9999, 10000, 10001, 10002, 10003, 20000} {
		data := nested(levels)
		compareWithOfficial(t, "Command", data)
		var c Command
		if c.UnmarshalCF(data) == nil {
			deepest = levels
		}
	}
	require.Greater(t, deepest, 9999)
	require.Less(t, deepest, 20000)

	// Groups which are opened and never closed.
	var c Command
	require.Error(t, c.UnmarshalCF(bytes.Repeat([]byte{0x0b}, 1<<20)))
}
