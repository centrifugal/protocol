package cfjsoncmp

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
	"sync"

	"github.com/centrifugal/protocol/cfjson"
	segmentio "github.com/segmentio/encoding/json"
)

// message is implemented by every generated type twice: by easyjson (the old
// implementation) and by cfjson (the new one), over the very same struct.
type message interface {
	MarshalEasyJSON(*writer)
	AppendJSON([]byte) []byte
	DecodeJSON([]byte, int, cfjson.Flags) int
	// DecodeJSONFold is generated with the -fold-keys option: a key which is
	// not the name of a field is matched again without regard to letter
	// case, which is what segmentio does by default.
	DecodeJSONFold([]byte, int, cfjson.Flags) int
}

// segmentioDecode is how protocol and Centrifugo decode with segmentio: with
// its default, case-insensitive, matching of keys.
func segmentioDecode(data []byte, m message, zeroCopy bool) error {
	var flags segmentio.ParseFlags
	if zeroCopy {
		flags = segmentio.ZeroCopy
	}
	_, err := segmentio.Parse(data, m, flags)
	return err
}

// foldDecode decodes with the case-insensitive decoder of cfjson.
func foldDecode(data []byte, m message, zeroCopy bool) error {
	var flags cfjson.Flags
	if zeroCopy {
		flags = cfjson.ZeroCopy
	}
	// What cfjson.Unmarshal does, with the other method.
	n := m.DecodeJSONFold(data, cfjson.SkipSpace(data, 0), cfjson.Prescan(data, flags))
	if n < 0 {
		return cfjson.Error(data, n)
	}
	if cfjson.SkipSpace(data, n) != len(data) {
		return cfjson.ErrTrailingData
	}
	return nil
}

// oldEncode is how protocol encoded a message before cfjson, see the
// JSON*Encoder types in encode.go as of v0.22.
func oldEncode(m message) []byte {
	w := newWriter()
	m.MarshalEasyJSON(w)
	res, err := w.BuildBytes()
	if err != nil {
		panic(err)
	}
	return res
}

type jsonBuffer struct{ b []byte }

var jsonBufferPool = sync.Pool{New: func() any { return &jsonBuffer{b: make([]byte, 0, 512)} }}

// newEncode is a copy of encodeJSON from protocol: what encoders do now.
func newEncode(m message) []byte {
	buf := jsonBufferPool.Get().(*jsonBuffer)
	b := m.AppendJSON(buf.b[:0])
	ret := make([]byte, len(b))
	copy(ret, b)
	buf.b = b
	jsonBufferPool.Put(buf)
	return ret
}

// oldDecode is how protocol decoded a message before cfjson, with one
// exception: keys are matched exactly, which cfjson does and segmentio has a
// flag for. protocol did not set it, so a key written in another case
// (`{"ID":1}`) used to be matched to its field.
func oldDecode(data []byte, m message, zeroCopy bool) error {
	flags := segmentio.DontMatchCaseInsensitiveStructFields
	if zeroCopy {
		flags |= segmentio.ZeroCopy
	}
	_, err := segmentio.Parse(data, m, flags)
	return err
}

// newDecode is how protocol decodes a message now.
func newDecode(data []byte, m message, zeroCopy bool) error {
	var flags cfjson.Flags
	if zeroCopy {
		flags = cfjson.ZeroCopy
	}
	return cfjson.Unmarshal(data, m, flags)
}

var rawType = reflect.TypeOf(Raw(nil))

var sampleStrings = []string{
	"", "a", "news", "chat:index", "12345678", "123456789", "exactly-sixteen-b",
	"a channel name which is quite a bit longer than one machine word",
	`quo"te`, `back\slash`, "new\nline", "tab\there", "\x00\x01\x1f", "\x7f",
	"<script>alert(1)&amp;</script>", "h\xc3\xa9llo", "\xe2\x9c\x93 check", "\xf0\x9f\x98\x80",
	"\xe2\x80\xa8sep\xe2\x80\xa9", "bad\xffutf8", "trunc\xc3", "\xed\xa0\x80", "\xef\xbf\xbd",
	"user@example.com", "550e8400-e29b-41d4-a716-446655440000", "/", "'", "{}", "null",
}

var sampleRaws = []string{
	`{}`, `[]`, `null`, `true`, `false`, `0`, `-1.5e10`, `"str"`, `{"a":1}`, `{"input":"hello"}`,
	`{"a": [1, 2, {"b": null}], "c": "d\"e"}`, "{\n  \"pretty\": true\n}", ` {"spaces":1} `,
	`[[[[]]]]`, "\n", "\n\n", `{"k":"\u00e9\ud83d\ude00"}`, "{\"utf8\":\"\xc3\xa9\"}", `"<b>&"`,
}

// filler fills messages with random values.
type filler struct {
	r *rand.Rand
	// maxMapLen above 1 makes the JSON encoding of a message non-deterministic,
	// since Go randomizes map iteration order.
	maxMapLen int
	// strings and raws are what string and Raw fields are filled from.
	strings, raws []string
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
		// Most fields are left empty, otherwise values explode in size: the
		// types are deeply nested.
		if depth > 5 || r.Intn(4) != 0 {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		f.fill(v.Elem(), depth+1)
	case reflect.String:
		if r.Intn(3) == 0 {
			return
		}
		s := f.strings[r.Intn(len(f.strings))]
		if r.Intn(8) == 0 {
			s = strings.Repeat(s, r.Intn(40))
		}
		v.SetString(s)
	case reflect.Bool:
		v.SetBool(r.Intn(2) == 0)
	case reflect.Int32, reflect.Int64, reflect.Int:
		var n int64
		switch r.Intn(7) {
		case 0:
		case 1:
			n = 1
		case 2:
			n = -1
		case 3:
			n = math.MaxInt64
		case 4:
			n = math.MinInt64
		case 5:
			n = int64(r.Intn(100000)) - 50000
		case 6:
			n = r.Int63()
		}
		if v.OverflowInt(n) {
			n = int64(int32(n))
		}
		v.SetInt(n)
	case reflect.Uint32, reflect.Uint64, reflect.Uint:
		var n uint64
		switch r.Intn(6) {
		case 0:
		case 1:
			n = 1
		case 2:
			n = math.MaxUint64
		case 3:
			n = uint64(r.Intn(100000))
		case 4:
			n = r.Uint64()
		case 5:
			n = 9
		}
		if v.OverflowUint(n) {
			n = uint64(uint32(n))
		}
		v.SetUint(n)
	case reflect.Slice:
		if v.Type() == rawType {
			switch r.Intn(5) {
			case 0:
			case 1:
				v.SetBytes([]byte{})
			default:
				v.SetBytes([]byte(f.raws[r.Intn(len(f.raws))]))
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
				key.SetString(f.strings[r.Intn(len(f.strings))])
				elem := reflect.New(v.Type().Elem()).Elem()
				f.fill(elem, depth+1)
				v.SetMapIndex(key, elem)
			}
		}
	default:
		panic("unsupported kind " + v.Kind().String())
	}
}

// randomMessage returns a message of a random type filled with random values.
func randomMessage(r *rand.Rand, maxMapLen int) message {
	m := allTypes[r.Intn(len(allTypes))]()
	f := filler{r: r, maxMapLen: maxMapLen, strings: sampleStrings, raws: sampleRaws}
	f.fill(reflect.ValueOf(m).Elem(), 0)
	return m
}

// zero returns a new zero value of the type of m.
func zero(m message) message {
	return reflect.New(reflect.TypeOf(m).Elem()).Interface().(message)
}
