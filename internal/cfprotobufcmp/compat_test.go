package cfprotobufcmp

import (
	"bytes"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/centrifugal/protocol/cfprotobuf"
)

// message is implemented by every type twice: by vtprotobuf (the VT methods,
// the old implementation) and by cfprotobuf (generated here with the CF
// suffix, the new one), over the very same struct.
type message interface {
	MarshalVT() ([]byte, error)
	MarshalToVT([]byte) (int, error)
	MarshalToSizedBufferVT([]byte) (int, error)
	SizeVT() int
	UnmarshalVT([]byte) error

	MarshalCF() ([]byte, error)
	MarshalToCF([]byte) (int, error)
	MarshalToSizedBufferCF([]byte) (int, error)
	SizeCF() int
	UnmarshalCF([]byte) error
}

var rawType = reflect.TypeOf(Raw(nil))

var sampleStrings = []string{
	"a", "news", "chat:index", "user@example.com", "550e8400-e29b-41d4-a716-446655440000",
	"h\xc3\xa9llo \xf0\x9f\x98\x80", "bad\xffutf8", "\x00", strings.Repeat("x", 127), strings.Repeat("y", 128), strings.Repeat("long ", 4000),
}

// filler fills messages with random values.
type filler struct {
	r *rand.Rand
	// maxMapLen above 1 makes the encoding of a message non-deterministic,
	// since Go randomizes map iteration order.
	maxMapLen int
	// validUTF8 keeps invalid UTF-8 out of strings.
	validUTF8 bool
}

func (f *filler) str() string {
	for {
		s := sampleStrings[f.r.Intn(len(sampleStrings))]
		if !f.validUTF8 || cfprotobuf.ValidString([]byte(s)) {
			return s
		}
	}
}

func (f *filler) fill(v reflect.Value, depth int) {
	r := f.r
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				f.fill(v.Field(i), depth)
			}
		}
	case reflect.Pointer:
		if depth > 5 || r.Intn(4) != 0 {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		f.fill(v.Elem(), depth+1)
	case reflect.String:
		if r.Intn(3) != 0 {
			v.SetString(f.str())
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
		if v.Type() == rawType {
			switch r.Intn(4) {
			case 0:
			case 1:
				v.SetBytes([]byte{})
			default:
				v.SetBytes([]byte(sampleStrings[r.Intn(len(sampleStrings))]))
			}
			return
		}
		switch r.Intn(4) {
		case 0:
		case 1:
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		default:
			n := 1 + r.Intn(3)
			v.Set(reflect.MakeSlice(v.Type(), n, n))
			for i := 0; i < n; i++ {
				f.fill(v.Index(i), depth+1)
			}
		}
	case reflect.Map:
		switch r.Intn(4) {
		case 0:
		case 1:
			v.Set(reflect.MakeMap(v.Type()))
		default:
			v.Set(reflect.MakeMap(v.Type()))
			for n := 1 + r.Intn(f.maxMapLen); n > 0; n-- {
				key := reflect.New(v.Type().Key()).Elem()
				key.SetString(f.str())
				elem := reflect.New(v.Type().Elem()).Elem()
				f.fill(elem, depth+1)
				v.SetMapIndex(key, elem)
			}
		}
	default:
		panic("unsupported kind " + v.Kind().String())
	}
}

func randomMessage(r *rand.Rand, maxMapLen int, validUTF8 bool) message {
	m := allTypes[r.Intn(len(allTypes))]()
	f := filler{r: r, maxMapLen: maxMapLen, validUTF8: validUTF8}
	f.fill(reflect.ValueOf(m).Elem(), 0)
	return m
}

func zero(m message) message {
	return reflect.New(reflect.TypeOf(m).Elem()).Interface().(message)
}

// The encoding must be byte for byte what vtprotobuf produces, by every way
// there is to call the marshalers. Marshaling does not look into strings, so
// the messages have invalid UTF-8 in them too.
func TestMarshalMatchesVTProtobuf(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < 200000; n++ {
		m := randomMessage(r, 1, false)
		want, err := m.MarshalVT()
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.MarshalCF()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%T:\n got %x\nwant %x", m, got, want)
		}
		if m.SizeCF() != m.SizeVT() || m.SizeCF() != len(want) {
			t.Fatalf("%T: SizeCF = %d, SizeVT = %d, the encoding has %d bytes", m, m.SizeCF(), m.SizeVT(), len(want))
		}
		buf := make([]byte, len(want)+8)
		if written, err := m.MarshalToCF(buf); err != nil || written != len(want) || !bytes.Equal(buf[:written], want) {
			t.Fatalf("%T: MarshalToCF wrote %d bytes, %v", m, written, err)
		}
		if written, err := m.MarshalToSizedBufferCF(buf); err != nil || written != len(want) || !bytes.Equal(buf[8:], want) {
			t.Fatalf("%T: MarshalToSizedBufferCF wrote %d bytes, %v", m, written, err)
		}
	}
}

// With several keys in a map the order of entries in the output is random,
// in both implementations, so the outputs are compared after decoding.
func TestMarshalMatchesVTProtobuf_MultiKeyMaps(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for n := 0; n < 50000; n++ {
		m := randomMessage(r, 4, false)
		want, _ := m.MarshalVT()
		got, _ := m.MarshalCF()
		if len(got) != len(want) {
			t.Fatalf("%T: %d bytes, want %d", m, len(got), len(want))
		}
		gotMsg, wantMsg := zero(m), zero(m)
		if err := gotMsg.UnmarshalVT(got); err != nil {
			t.Fatal(err)
		}
		if err := wantMsg.UnmarshalVT(want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotMsg, wantMsg) {
			t.Fatalf("%T: the encodings decode to different messages", m)
		}
	}
}

// compareUnmarshal decodes data with both implementations.
//
// On input which is not what a marshaler produces the two are not expected
// to agree: vtprotobuf departs from the Protobuf specification in a number of
// ways (see README.md), cfprotobuf follows google.golang.org/protobuf. That
// cfprotobuf gets every input right is checked in the protocol package,
// against google.golang.org/protobuf itself. What is checked here is that the
// differences with vtprotobuf are the known ones and nothing else.
func compareUnmarshal(t testing.TB, data []byte, proto message) {
	t.Helper()
	oldData, newData := bytes.Clone(data), bytes.Clone(data)
	oldMsg, newMsg := zero(proto), zero(proto)
	oldErr := oldMsg.UnmarshalVT(oldData)
	newErr := newMsg.UnmarshalCF(newData)
	if !bytes.Equal(newData, data) {
		t.Fatalf("%T: decoding modified the input %x", proto, data)
	}
	if newErr != nil {
		// cfprotobuf is the stricter one in general: invalid UTF-8 in a
		// string, a varint over 64 bits, a field number out of range, a
		// length inside of a map entry which goes past the entry.
		return
	}
	if oldErr != nil {
		// With one exception: a field which came with a wire type its type
		// does not have is an unknown field for cfprotobuf. vtprotobuf
		// returns an error for it, or, inside of a map entry, reads it as
		// if it had the wire type it expects and fails on what comes out.
		if !consistent(newMsg) {
			t.Fatalf("%T input %x: vtprotobuf rejects it (%v), cfprotobuf accepts", proto, data, oldErr)
		}
		return
	}
	if reflect.DeepEqual(oldMsg, newMsg) {
		// Nothing decoded may point into the input.
		for i := range newData {
			newData[i] = 0xFF
		}
		if !reflect.DeepEqual(oldMsg, newMsg) {
			t.Fatalf("%T input %x: the decoded message points into the input", proto, data)
		}
		return
	}
	if consistent(newMsg) {
		return
	}
	t.Fatalf("%T input %x: the decoders disagree", proto, data)
}

// consistent covers the inputs which both decoders accept and get different
// messages out of. There are three ways for that to happen:
//
//   - A map entry which does not have the value, or has it more than once,
//     when the value is a message: vtprotobuf leaves nil in the map for the
//     first and keeps the last value for the second, cfprotobuf has an empty
//     message and merges, as google.golang.org/protobuf does.
//   - Inside of a map entry vtprotobuf does not check the wire type of the key
//     and of the value: it reads a varint as if it was a length, and so on.
//     cfprotobuf skips such a field.
//   - The tag of an unknown field which is not encoded in the shortest form is
//     kept as it came by vtprotobuf, and in the shortest form by cfprotobuf.
//
// Telling that one of these is what happened takes a parser of its own.
// Instead the message cfprotobuf decoded must at least make sense to both:
// marshaled and decoded again by each of them, it must come out the same.
func consistent(decoded message) bool {
	encoded, err := decoded.MarshalCF()
	if err != nil {
		return false
	}
	viaOld, viaNew := zero(decoded), zero(decoded)
	if viaNew.UnmarshalCF(encoded) != nil {
		return false
	}
	if err := viaOld.UnmarshalVT(encoded); err != nil {
		// A field cfprotobuf keeps as unknown because of its wire type is
		// written back as it came, and vtprotobuf refuses it again.
		return strings.Contains(err.Error(), "wrong wireType")
	}
	return reflect.DeepEqual(viaOld, viaNew)
}

// Whatever the marshalers produce from valid messages must decode to the
// same value with both decoders.
func TestUnmarshalMatchesVTProtobuf_Encoded(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for n := 0; n < 100000; n++ {
		m := randomMessage(r, 4, true)
		data, _ := m.MarshalCF()
		oldMsg, newMsg := zero(m), zero(m)
		if err := oldMsg.UnmarshalVT(data); err != nil {
			t.Fatal(err)
		}
		if err := newMsg.UnmarshalCF(data); err != nil {
			t.Fatalf("%T: %v: %x", m, err, data)
		}
		if !reflect.DeepEqual(oldMsg, newMsg) {
			t.Fatalf("%T input %x: the decoders disagree", m, data)
		}
	}
}

// Damaged messages: both decoders must agree on those too.
func TestUnmarshalMatchesVTProtobuf_Damaged(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for n := 0; n < 200000; n++ {
		m := randomMessage(r, 3, true)
		data, _ := m.MarshalCF()
		if len(data) == 0 || len(data) > 4096 {
			continue
		}
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
		compareUnmarshal(t, data, m)
	}
}

// Each input is decoded into a message type picked by its first byte.
func FuzzUnmarshalMatchesVTProtobuf(f *testing.F) {
	r := rand.New(rand.NewSource(5))
	for n := 0; n < 400; n++ {
		m := randomMessage(r, 3, true)
		data, _ := m.MarshalCF()
		if len(data) > 2048 {
			continue
		}
		for typ, newMsg := range allTypes {
			if reflect.TypeOf(newMsg()) == reflect.TypeOf(m) {
				f.Add(byte(typ), data)
			}
		}
	}
	f.Fuzz(func(t *testing.T, typ byte, data []byte) {
		compareUnmarshal(t, data, allTypes[int(typ)%len(allTypes)]())
	})
}
