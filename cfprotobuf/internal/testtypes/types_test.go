package testtypes

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/centrifugal/protocol/cfprotobuf"
	"github.com/centrifugal/protocol/cfprotobuf/gen"
)

// The generated file must be what the generator produces today.
func TestGeneratedCodeIsUpToDate(t *testing.T) {
	want, err := gen.Generate(gen.Config{Files: []string{"types.go"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("types_cfprotobuf.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("types_cfprotobuf.go is out of date, run go generate")
	}
}

// A reference encoder for All, written the obvious way: front to back, one
// field after another, with nothing shared with the generated code, which
// fills its buffer from the end.

func refVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func refTag(b []byte, number, wire int) []byte {
	return refVarint(b, uint64(number)<<3|uint64(wire))
}

func refBytes(b []byte, number int, v []byte) []byte {
	b = refTag(b, number, 2)
	b = refVarint(b, uint64(len(v)))
	return append(b, v...)
}

func refLeaf(m *Leaf) []byte {
	var b []byte
	if m == nil {
		return b
	}
	if m.ID != 0 {
		b = refVarint(refTag(b, 1, 0), uint64(m.ID))
	}
	if m.Text != "" {
		b = refBytes(b, 2, []byte(m.Text))
	}
	return append(b, m.unknownFields...)
}

func sortedKeys[V any, K ~string](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func refAll(m *All) []byte {
	var b []byte
	if m == nil {
		return b
	}
	if m.String != "" {
		b = refBytes(b, 1, []byte(m.String))
	}
	if len(m.Bytes) > 0 {
		b = refBytes(b, 2, m.Bytes)
	}
	if len(m.Raw) > 0 {
		b = refBytes(b, 3, m.Raw)
	}
	if m.Bool {
		b = append(refTag(b, 4, 0), 1)
	}
	if m.Int32 != 0 {
		b = refVarint(refTag(b, 5, 0), uint64(int64(m.Int32)))
	}
	if m.Int64 != 0 {
		b = refVarint(refTag(b, 6, 0), uint64(m.Int64))
	}
	if m.Uint32 != 0 {
		b = refVarint(refTag(b, 7, 0), uint64(m.Uint32))
	}
	if m.Uint64 != 0 {
		b = refVarint(refTag(b, 8, 0), m.Uint64)
	}
	if m.Sint64 != 0 {
		b = refVarint(refTag(b, 9, 0), uint64(m.Sint64<<1)^uint64(m.Sint64>>63))
	}
	if m.Double != 0 {
		b = binary.LittleEndian.AppendUint64(refTag(b, 10, 1), math.Float64bits(m.Double))
	}
	if m.Float != 0 {
		b = binary.LittleEndian.AppendUint32(refTag(b, 11, 5), math.Float32bits(m.Float))
	}
	if m.Leaf != nil {
		b = refBytes(b, 12, refLeaf(m.Leaf))
	}
	if m.Self != nil {
		b = refBytes(b, 13, refAll(m.Self))
	}
	for _, leaf := range m.Leaves {
		b = refBytes(b, 14, refLeaf(leaf))
	}
	for _, s := range m.Strings {
		b = refBytes(b, 15, []byte(s))
	}
	for _, k := range sortedKeys(m.StrMap) {
		b = refBytes(b, 16, refBytes(refBytes(nil, 1, []byte(k)), 2, []byte(m.StrMap[k])))
	}
	for _, k := range sortedKeys(m.LeafMap) {
		b = refBytes(b, 17, refBytes(refBytes(nil, 1, []byte(k)), 2, refLeaf(m.LeafMap[k])))
	}
	for _, k := range sortedKeys(m.FloatMap) {
		entry := binary.LittleEndian.AppendUint64(refTag(refBytes(nil, 1, []byte(k)), 2, 1), math.Float64bits(m.FloatMap[k]))
		b = refBytes(b, 18, entry)
	}
	for _, k := range sortedKeys(m.IntMap) {
		b = refBytes(b, 19, refVarint(refTag(refBytes(nil, 1, []byte(k)), 2, 0), uint64(m.IntMap[k])))
	}
	for _, k := range sortedKeys(m.BoolMap) {
		v := byte(0)
		if m.BoolMap[k] {
			v = 1
		}
		b = refBytes(b, 20, append(refTag(refBytes(nil, 1, []byte(k)), 2, 0), v))
	}
	if m.Name != "" {
		b = refBytes(b, 21, []byte(m.Name))
	}
	if m.Level != 0 {
		b = refVarint(refTag(b, 22, 0), uint64(int64(m.Level)))
	}
	if m.Far != "" {
		b = refBytes(b, 2047, []byte(m.Far))
	}
	if m.Farther != 0 {
		b = refVarint(refTag(b, 300000, 0), m.Farther)
	}
	if m.Farthest != nil {
		b = refBytes(b, 536870911, refLeaf(m.Farthest))
	}
	return append(b, m.unknownFields...)
}

var strs = []string{"a", "plain text", "h\xc3\xa9llo \xf0\x9f\x98\x80", strings.Repeat("long ", 40), strings.Repeat("x", 127), strings.Repeat("y", 128), strings.Repeat("z", 20000)}

// fill sets the fields of v to random values. Slices and maps are left nil
// or get something in them, never empty: an empty one is not written, so it
// would not survive a round trip as it is. Maps get at most maxMapLen keys.
func fill(r *rand.Rand, v reflect.Value, depth, maxMapLen int) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Type().Field(i); f.IsExported() && f.Tag.Get("protobuf") != "" {
				fill(r, v.Field(i), depth+1, maxMapLen)
			}
		}
	case reflect.Pointer:
		if depth < 6 && r.Intn(3) == 0 {
			v.Set(reflect.New(v.Type().Elem()))
			fill(r, v.Elem(), depth+1, maxMapLen)
		}
	case reflect.String:
		if r.Intn(3) != 0 {
			v.SetString(strs[r.Intn(len(strs))])
		}
	case reflect.Bool:
		v.SetBool(r.Intn(2) == 0)
	case reflect.Int32, reflect.Int64:
		n := []int64{0, 1, -1, 127, 128, -129, math.MaxInt32, math.MinInt32, math.MaxInt64, math.MinInt64, r.Int63()}[r.Intn(11)]
		if v.Kind() == reflect.Int32 {
			n = int64(int32(n))
		}
		v.SetInt(n)
	case reflect.Uint32, reflect.Uint64:
		n := []uint64{0, 1, 127, 128, 16383, 16384, math.MaxUint32, math.MaxUint64, r.Uint64()}[r.Intn(9)]
		if v.Kind() == reflect.Uint32 {
			n = uint64(uint32(n))
		}
		v.SetUint(n)
	case reflect.Float32:
		v.SetFloat(float64([]float32{0, 1.5, -2.25, math.MaxFloat32, float32(math.Inf(1))}[r.Intn(5)]))
	case reflect.Float64:
		v.SetFloat([]float64{0, 1.5, -2.25, math.MaxFloat64, math.SmallestNonzeroFloat64, math.Inf(-1)}[r.Intn(6)])
	case reflect.Slice:
		if r.Intn(2) == 0 {
			return
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			v.SetBytes([]byte(strs[r.Intn(len(strs))]))
			return
		}
		n := 1 + r.Intn(3)
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		for i := 0; i < n; i++ {
			elem := v.Index(i)
			if elem.Kind() == reflect.Pointer {
				// A nil element is written as an empty message and comes
				// back as one.
				elem.Set(reflect.New(elem.Type().Elem()))
				fill(r, elem.Elem(), depth+1, maxMapLen)
			} else {
				elem.SetString(strs[r.Intn(len(strs))])
			}
		}
	case reflect.Map:
		if r.Intn(2) == 0 {
			return
		}
		v.Set(reflect.MakeMap(v.Type()))
		for n := 1 + r.Intn(maxMapLen); n > 0; n-- {
			key, elem := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
			key.SetString(strs[r.Intn(len(strs)-1)])
			if elem.Kind() == reflect.Pointer {
				elem.Set(reflect.New(elem.Type().Elem()))
				fill(r, elem.Elem(), depth+1, maxMapLen)
			} else {
				fill(r, elem, depth+1, maxMapLen)
			}
			v.SetMapIndex(key, elem)
		}
	}
}

func randomAll(r *rand.Rand, maxMapLen int) *All {
	m := new(All)
	fill(r, reflect.ValueOf(m).Elem(), 0, maxMapLen)
	return m
}

// Generated marshalers must produce what the reference encoder produces, by
// every way there is to call them.
func TestMarshalMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < 20000; n++ {
		// With a single key in a map the encoding is deterministic.
		m := randomAll(r, 1)
		want := refAll(m)

		got, err := m.MarshalVT()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("MarshalVT of %+v:\n got %x\nwant %x", m, got, want)
		}
		if size := m.SizeVT(); size != len(want) {
			t.Fatalf("SizeVT = %d, the encoding has %d bytes", size, len(want))
		}

		// MarshalToVT writes to the beginning of a buffer.
		buf := bytes.Repeat([]byte{0xAA}, len(want)+10)
		written, err := m.MarshalToVT(buf)
		if err != nil || written != len(want) || !bytes.Equal(buf[:written], want) {
			t.Fatalf("MarshalToVT wrote %d bytes, %v", written, err)
		}
		if !bytes.Equal(buf[written:], bytes.Repeat([]byte{0xAA}, 10)) {
			t.Fatal("MarshalToVT wrote past the encoding")
		}

		// MarshalToSizedBufferVT writes to the end of one.
		buf = bytes.Repeat([]byte{0xAA}, len(want)+10)
		written, err = m.MarshalToSizedBufferVT(buf)
		if err != nil || written != len(want) || !bytes.Equal(buf[10:], want) {
			t.Fatalf("MarshalToSizedBufferVT wrote %d bytes, %v", written, err)
		}
		if !bytes.Equal(buf[:10], bytes.Repeat([]byte{0xAA}, 10)) {
			t.Fatal("MarshalToSizedBufferVT wrote before the encoding")
		}
	}
}

// Whatever is marshaled must unmarshal to the same value, including maps
// with several keys, which are written in random order.
func TestRoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for n := 0; n < 20000; n++ {
		m := randomAll(r, 4)
		data, err := m.MarshalVT()
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != m.SizeVT() {
			t.Fatalf("SizeVT = %d, the encoding has %d bytes", m.SizeVT(), len(data))
		}
		got := new(All)
		if err := got.UnmarshalVT(data); err != nil {
			t.Fatalf("%v: %x", err, data)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatalf("round trip changed the message:\n got %+v\nwant %+v", got, m)
		}
		// Nothing decoded may point into the input.
		for i := range data {
			data[i] = 0xFF
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatal("the decoded message changed together with the input")
		}
	}
}

func TestNil(t *testing.T) {
	var m *All
	data, err := m.MarshalVT()
	if data != nil || err != nil {
		t.Fatalf("MarshalVT of nil = %v, %v", data, err)
	}
	if m.SizeVT() != 0 {
		t.Fatal("SizeVT of nil is not 0")
	}
	if n, err := m.MarshalToSizedBufferVT(nil); n != 0 || err != nil {
		t.Fatalf("MarshalToSizedBufferVT of nil = %d, %v", n, err)
	}
	data, _ = new(Empty).MarshalVT()
	if len(data) != 0 {
		t.Fatalf("an empty message is %x", data)
	}
	// A nil element of a repeated field and a nil map value are empty
	// messages.
	m = &All{Leaves: []*Leaf{nil, {ID: 1}}, LeafMap: map[string]*Leaf{"k": nil}}
	data, _ = m.MarshalVT()
	if want := refAll(m); !bytes.Equal(data, want) {
		t.Fatalf("got %x, want %x", data, want)
	}
	got := new(All)
	if err := got.UnmarshalVT(data); err != nil {
		t.Fatal(err)
	}
	if len(got.Leaves) != 2 || got.Leaves[0] == nil || got.Leaves[1].ID != 1 {
		t.Fatalf("got leaves %+v", got.Leaves)
	}
	if leaf, ok := got.LeafMap["k"]; !ok || leaf == nil {
		t.Fatalf("got leaf map %+v", got.LeafMap)
	}
}

// Decoding into a message which has something in it merges, the way Protobuf
// defines it.
func TestUnmarshalMerges(t *testing.T) {
	first, _ := (&All{String: "a", Leaf: &Leaf{ID: 1}, Strings: []string{"x"}, StrMap: map[string]string{"k": "1", "only": "first"}, Bytes: []byte("b")}).MarshalVT()
	second, _ := (&All{Int32: 5, Leaf: &Leaf{Text: "t"}, Strings: []string{"y"}, StrMap: map[string]string{"k": "2"}}).MarshalVT()
	m := new(All)
	if err := m.UnmarshalVT(append(first, second...)); err != nil {
		t.Fatal(err)
	}
	want := &All{String: "a", Int32: 5, Leaf: &Leaf{ID: 1, Text: "t"}, Strings: []string{"x", "y"},
		StrMap: map[string]string{"k": "2", "only": "first"}, Bytes: []byte("b")}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("got %+v", m)
	}
	// A bytes field which is present but empty is not nil.
	if err := m.UnmarshalVT([]byte{0x12, 0x00, 0x1a, 0x00}); err != nil {
		t.Fatal(err)
	}
	if m.Bytes == nil || len(m.Bytes) != 0 || m.Raw == nil {
		t.Fatalf("got bytes %v, raw %v", m.Bytes, m.Raw)
	}
}

// Fields a message does not have are kept and written back, unless the type
// has nowhere to keep them.
func TestUnknownFields(t *testing.T) {
	unknown := []byte{
		0xa0, 0x06, 0x05, // field 100, varint
		0xaa, 0x06, 0x02, 'h', 'i', // field 101, bytes
		0xb1, 0x06, 1, 2, 3, 4, 5, 6, 7, 8, // field 102, fixed64
		0xbd, 0x06, 1, 2, 3, 4, // field 103, fixed32
		0xc3, 0x06, 0x08, 0x01, 0xcb, 0x06, 0x10, 0x02, 0xcc, 0x06, 0xc4, 0x06, // field 104, a group with a group in it
	}
	known, _ := (&All{String: "s", Uint32: 7}).MarshalVT()
	m := new(All)
	if err := m.UnmarshalVT(append(append([]byte{}, unknown...), known...)); err != nil {
		t.Fatal(err)
	}
	if m.String != "s" || m.Uint32 != 7 || !bytes.Equal(m.Unknown(), unknown) {
		t.Fatalf("got %+v", m)
	}
	data, _ := m.MarshalVT()
	if want := append(append([]byte{}, known...), unknown...); !bytes.Equal(data, want) {
		t.Fatalf("got %x, want %x", data, want)
	}
	if m.SizeVT() != len(data) {
		t.Fatal("SizeVT does not count unknown fields")
	}

	var n NoUnknown
	if err := n.UnmarshalVT(append([]byte{0x08, 0x03}, unknown...)); err != nil {
		t.Fatal(err)
	}
	if data, _ := n.MarshalVT(); !bytes.Equal(data, []byte{0x08, 0x03}) {
		t.Fatalf("got %x", data)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"truncated tag", []byte{0x80}, cfprotobuf.ErrTruncated},
		{"truncated varint", []byte{0x28, 0x80}, cfprotobuf.ErrTruncated},
		{"truncated length", []byte{0x0a}, cfprotobuf.ErrTruncated},
		{"length past the end", []byte{0x0a, 0x05, 'a', 'b'}, cfprotobuf.ErrTruncated},
		{"huge length", []byte{0x0a, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}, cfprotobuf.ErrTruncated},
		{"truncated double", []byte{0x51, 1, 2, 3, 4, 5, 6, 7}, cfprotobuf.ErrTruncated},
		{"truncated float", []byte{0x5d, 1, 2, 3}, cfprotobuf.ErrTruncated},
		{"truncated nested message", []byte{0x62, 0x02, 0x08}, cfprotobuf.ErrTruncated},
		{"truncated unknown fixed64", []byte{0xb1, 0x06, 1, 2, 3}, cfprotobuf.ErrTruncated},
		{"unterminated group", []byte{0xc3, 0x06, 0x08, 0x01}, cfprotobuf.ErrTruncated},
		{"varint of eleven bytes", []byte{0x28, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}, cfprotobuf.ErrIntOverflow},
		{"varint over 64 bits", []byte{0x28, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02}, cfprotobuf.ErrIntOverflow},
		{"field number zero", []byte{0x00, 0x01}, cfprotobuf.ErrInvalidTag},
		{"field number too large", []byte{0x80, 0x80, 0x80, 0x80, 0x10, 0x01}, cfprotobuf.ErrInvalidTag},
		{"end of a group which was not started", []byte{0xc4, 0x06}, cfprotobuf.ErrInvalidTag},
		{"group closed with another number", []byte{0xc3, 0x06, 0xcc, 0x06}, cfprotobuf.ErrInvalidTag},
		{"wire type 6", []byte{0xa6, 0x06}, cfprotobuf.ErrInvalidTag},
		{"string which is not UTF-8", []byte{0x0a, 0x02, 'a', 0xff}, cfprotobuf.ErrInvalidUTF8},
		{"repeated string which is not UTF-8", []byte{0x7a, 0x01, 0xc3}, cfprotobuf.ErrInvalidUTF8},
		{"map key which is not UTF-8", []byte{0x82, 0x01, 0x03, 0x0a, 0x01, 0xff}, cfprotobuf.ErrInvalidUTF8},
		{"map value which is not UTF-8", []byte{0x82, 0x01, 0x03, 0x12, 0x01, 0xff}, cfprotobuf.ErrInvalidUTF8},
		{"error in a nested message", []byte{0x62, 0x03, 0x12, 0x01, 0xff}, cfprotobuf.ErrInvalidUTF8},
		{"error in a map value", []byte{0x8a, 0x01, 0x05, 0x12, 0x03, 0x12, 0x01, 0xff}, cfprotobuf.ErrInvalidUTF8},
	}
	for _, tt := range tests {
		if err := new(All).UnmarshalVT(tt.data); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
	// A field the message has, with a wire type it does not have, is not an
	// error: it is kept as a field the message does not know.
	for _, data := range [][]byte{{0x08, 0x01}, {0x22, 0x01, 0x01}, {0x55, 1, 2, 3, 4}, {0x60, 0x01}, {0x0b, 0x0c}} {
		m := new(All)
		if err := m.UnmarshalVT(data); err != nil {
			t.Errorf("%x: %v", data, err)
		}
		if !bytes.Equal(m.Unknown(), data) || m.String != "" || m.Bool || m.Leaf != nil {
			t.Errorf("%x: got %+v", data, m)
		}
	}
	// An end of a group is an error whatever its number is.
	if err := new(All).UnmarshalVT([]byte{0x0c}); !errors.Is(err, cfprotobuf.ErrInvalidTag) {
		t.Errorf("got %v, want ErrInvalidTag", err)
	}
	// Bytes fields may hold anything, only strings must be UTF-8.
	m := new(All)
	if err := m.UnmarshalVT([]byte{0x12, 0x02, 0xff, 0xfe, 0x1a, 0x01, 0xc3}); err != nil || !bytes.Equal(m.Bytes, []byte{0xff, 0xfe}) {
		t.Fatalf("got %v, %v", m.Bytes, err)
	}
	// In a map entry a field with an unexpected number or wire type is
	// skipped, a missing key or value is the zero value.
	m = new(All)
	if err := m.UnmarshalVT([]byte{0x82, 0x01, 0x07, 0x08, 0x05, 0x1a, 0x01, 'x', 0x10, 0x01}); err != nil {
		t.Fatal(err)
	}
	if v, ok := m.StrMap[""]; !ok || v != "" || len(m.StrMap) != 1 {
		t.Fatalf("got %v", m.StrMap)
	}
}

// All can hold another All, and Ping and Pong hold each other, so the input
// decides how deep the decoder recurses. There must be a limit.
func TestDepthLimit(t *testing.T) {
	nested := func(depth int) []byte {
		// Built from the inside out: each level wraps the previous one as
		// the field Self.
		data := []byte{0x28, 0x01}
		for ; depth > 1; depth-- {
			data = refBytes(nil, 13, data)
		}
		return data
	}
	m := new(All)
	if err := m.UnmarshalVT(nested(cfprotobuf.MaxDepth)); err != nil {
		t.Fatal(err)
	}
	depth := 1
	for p := m; p.Self != nil; p = p.Self {
		depth++
	}
	if depth != cfprotobuf.MaxDepth {
		t.Fatalf("decoded %d levels", depth)
	}
	// It must marshal back, however deep it is.
	if data, err := m.MarshalVT(); err != nil || !bytes.Equal(data, nested(cfprotobuf.MaxDepth)) {
		t.Fatalf("marshaling a deep message: %v", err)
	}
	if err := new(All).UnmarshalVT(nested(cfprotobuf.MaxDepth + 1)); !errors.Is(err, cfprotobuf.ErrTooDeep) {
		t.Fatalf("got %v, want ErrTooDeep", err)
	}

	// Ping -> Pong -> map of Ping.
	data := []byte{}
	for level := 0; level < cfprotobuf.MaxDepth+2; level++ {
		if level%2 == 0 {
			data = refBytes(nil, 1, refBytes(refBytes(nil, 1, []byte("k")), 2, data)) // Pong.Pings
		} else {
			data = refBytes(nil, 1, data) // Ping.Pong
		}
	}
	// The last level added is a Ping.
	if err := new(Ping).UnmarshalVT(data); !errors.Is(err, cfprotobuf.ErrTooDeep) {
		t.Fatalf("got %v, want ErrTooDeep", err)
	}

	// Siblings do not add up, only nesting does.
	wide := new(All)
	for n := 0; n < 50000; n++ {
		wide.Leaves = append(wide.Leaves, &Leaf{ID: 1})
	}
	data, _ = wide.MarshalVT()
	if err := new(All).UnmarshalVT(data); err != nil {
		t.Fatal(err)
	}
}

// Decoding of arbitrary bytes must not panic, and whatever it accepts must
// survive being marshaled and unmarshaled again.
func FuzzUnmarshal(f *testing.F) {
	r := rand.New(rand.NewSource(3))
	for n := 0; n < 50; n++ {
		data, _ := randomAll(r, 2).MarshalVT()
		if len(data) < 4096 {
			f.Add(data)
		}
	}
	f.Add([]byte{0xc3, 0x06, 0x08, 0x01, 0xc4, 0x06})
	f.Fuzz(func(t *testing.T, data []byte) {
		input := bytes.Clone(data)
		m := new(All)
		err := m.UnmarshalVT(data)
		if !bytes.Equal(input, data) {
			t.Fatal("decoding modified the input")
		}
		if err != nil {
			return
		}
		encoded, err := m.MarshalVT()
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) != m.SizeVT() {
			t.Fatalf("SizeVT = %d, the encoding has %d bytes", m.SizeVT(), len(encoded))
		}
		// The first round trip may change the message in one way: a bytes
		// field which was present and empty is not written, so it comes back
		// nil. After that nothing may change.
		first := new(All)
		if err := first.UnmarshalVT(encoded); err != nil {
			t.Fatalf("own output does not decode: %v", err)
		}
		encoded, _ = first.MarshalVT()
		second := new(All)
		if err := second.UnmarshalVT(encoded); err != nil {
			t.Fatalf("own output does not decode: %v", err)
		}
		// NaN is not equal to itself, which DeepEqual respects.
		if !hasNaN(first) && !reflect.DeepEqual(first, second) {
			t.Fatalf("round trip of %x is not stable", input)
		}
	})
}

func hasNaN(m *All) bool {
	for ; m != nil; m = m.Self {
		if math.IsNaN(m.Double) || math.IsNaN(float64(m.Float)) {
			return true
		}
		for _, v := range m.FloatMap {
			if math.IsNaN(v) {
				return true
			}
		}
	}
	return false
}

// The UTF-8 check looks at several bytes at a time, with words which may
// overlap: a byte which is not ASCII must be noticed wherever it is.
func TestValidString(t *testing.T) {
	for size := 0; size <= 150; size++ {
		ascii := bytes.Repeat([]byte("a"), size)
		if !cfprotobuf.ValidString(ascii) {
			t.Fatalf("size %d: ASCII rejected", size)
		}
		if got, want := cfprotobuf.ShortASCII(ascii), size <= 16; got != want {
			t.Fatalf("size %d: ShortASCII = %v", size, got)
		}
		for pos := 0; pos < size; pos++ {
			for _, c := range []byte{0x80, 0xc3, 0xff} {
				bad := bytes.Clone(ascii)
				bad[pos] = c
				if cfprotobuf.ShortASCII(bad) || cfprotobuf.ValidString(bad) {
					t.Fatalf("size %d: %#x at %d accepted", size, c, pos)
				}
			}
			if pos+1 < size {
				good := bytes.Clone(ascii)
				good[pos], good[pos+1] = 0xc3, 0xa9
				if cfprotobuf.ShortASCII(good) || !cfprotobuf.ValidString(good) {
					t.Fatalf("size %d: a valid rune at %d is handled wrong", size, pos)
				}
			}
		}
	}
}
