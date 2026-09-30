package cfjsoncmp

import (
	"bytes"
	stdjson "encoding/json"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"github.com/centrifugal/protocol/cfjson"
	segmentio "github.com/segmentio/encoding/json"
)

// The encoding must be byte for byte what easyjson produced: client SDKs in
// other languages, and recorded data such as history in brokers, depend on it.
func TestEncodeMatchesEasyjson(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < 200000; n++ {
		m := randomMessage(r, 1)
		want := oldEncode(m)
		got := m.AppendJSON(nil)
		if !bytes.Equal(got, want) {
			t.Fatalf("%T:\n got %q\nwant %q", m, got, want)
		}
		if got := newEncode(m); !bytes.Equal(got, want) {
			t.Fatalf("%T (pooled):\n got %q\nwant %q", m, got, want)
		}
	}
}

// Appending must not depend on what is already in the buffer or on how much
// room it has.
func TestEncodeAppends(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for n := 0; n < 20000; n++ {
		m := randomMessage(r, 1)
		want := oldEncode(m)
		prefix := []byte("prefix")
		got := m.AppendJSON(prefix[:len(prefix):len(prefix)])
		if !bytes.Equal(got[len(prefix):], want) || string(got[:len(prefix)]) != "prefix" {
			t.Fatalf("%T:\n got %q\nwant %q", m, got, want)
		}
	}
}

// With several keys in a map the order of keys in the output is random, in
// both implementations, so the outputs are compared as JSON documents.
func TestEncodeMatchesEasyjson_MultiKeyMaps(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for n := 0; n < 50000; n++ {
		m := randomMessage(r, 4)
		want := oldEncode(m)
		got := m.AppendJSON(nil)
		if len(got) != len(want) {
			t.Fatalf("%T:\n got %q\nwant %q", m, got, want)
		}
		var gotDoc, wantDoc any
		// Raw values of a random message may be invalid JSON only if they
		// have invalid UTF-8, which encoding/json tolerates.
		if err := stdjson.Unmarshal(want, &wantDoc); err != nil {
			t.Fatalf("%T: %v: %q", m, err, want)
		}
		if err := stdjson.Unmarshal(got, &gotDoc); err != nil {
			t.Fatalf("%T: %v: %q", m, err, got)
		}
		if !reflect.DeepEqual(gotDoc, wantDoc) {
			t.Fatalf("%T:\n got %q\nwant %q", m, got, want)
		}
	}
}

// compareDecode decodes data with both implementations and fails if they
// disagree on whether it is acceptable, or on the result.
func compareDecode(t testing.TB, data []byte, proto message) {
	t.Helper()
	for _, zeroCopy := range []bool{false, true} {
		// Each implementation gets its own copy of the input: neither may
		// modify it, which is checked below.
		oldData, newData := bytes.Clone(data), bytes.Clone(data)
		oldMsg, newMsg := zero(proto), zero(proto)
		oldErr := oldDecode(oldData, oldMsg, zeroCopy)
		newErr := newDecode(newData, newMsg, zeroCopy)
		if !bytes.Equal(newData, data) {
			t.Fatalf("%T: decoding modified the input %q", proto, data)
		}
		if (oldErr == nil) != (newErr == nil) {
			if knownDivergence(data, oldErr, newErr) {
				continue
			}
			t.Fatalf("%T zeroCopy=%v input %q:\nold error: %v\nnew error: %v", proto, zeroCopy, data, oldErr, newErr)
		}
		if oldErr != nil {
			continue
		}
		if !reflect.DeepEqual(oldMsg, newMsg) {
			t.Fatalf("%T zeroCopy=%v input %q:\nold: %s\nnew: %s", proto, zeroCopy, data, dump(oldMsg), dump(newMsg))
		}
		// The same strings must point into the input and the same must be
		// copies of it. Code using protocol may depend, knowingly or not, on
		// which strings stay valid when the read buffer is reused.
		if aliasingDiffers(data) {
			continue
		}
		oldAliases, newAliases := aliases(oldMsg, oldData), aliases(newMsg, newData)
		if !zeroCopy && strings.Contains(newAliases, "1") {
			t.Fatalf("%T input %q: a string points into the input without zero-copy", proto, data)
		}
		if oldAliases != newAliases {
			t.Fatalf("%T zeroCopy=%v input %q: different strings point into the input:\nold: %s\nnew: %s",
				proto, zeroCopy, data, oldAliases, newAliases)
		}
	}
}

// aliases returns a digit for every string of m, in a fixed order: 1 if the
// string points into data, 0 if it does not.
func aliases(m message, data []byte) string {
	var out []byte
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.String:
			c := byte('0')
			if s := v.String(); len(s) > 0 && len(data) > 0 {
				p := uintptr(unsafe.Pointer(unsafe.StringData(s)))
				start := uintptr(unsafe.Pointer(unsafe.SliceData(data)))
				if p >= start && p < start+uintptr(len(data)) {
					c = '1'
				}
			}
			out = append(out, c)
		case reflect.Pointer:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				// Raw values are always copies.
				if v.Len() > 0 && len(data) > 0 {
					p := uintptr(v.UnsafePointer())
					start := uintptr(unsafe.Pointer(unsafe.SliceData(data)))
					if p >= start && p < start+uintptr(len(data)) {
						out = append(out, 'R')
					}
				}
				return
			}
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Map:
			keys := v.MapKeys()
			sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
			for _, key := range keys {
				walk(key)
				walk(v.MapIndex(key))
			}
		}
	}
	walk(reflect.ValueOf(m))
	return string(out)
}

func dump(m message) string {
	b, err := stdjson.Marshal(m)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// Whatever the encoders produce must decode to the same value with both
// decoders.
func TestDecodeMatchesSegmentio_Encoded(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for n := 0; n < 100000; n++ {
		m := randomMessage(r, 4)
		compareDecode(t, m.AppendJSON(nil), m)
	}
}

// edgeCases are inputs picked to hit the corners of the decoders: every one
// is decoded into every message type by both implementations.
var edgeCases = []string{
	``, ` `, `{}`, ` { } `, `null`, ` null `, `nul`, `nulll`, `{}x`, `{} {}`, `{}}`, `[]`, `1`, `"s"`, `true`,
	`{`, `{"`, `{"id`, `{"id"`, `{"id":`, `{"id":1`, `{"id":1,`, `{"id":1,}`, `{,}`, `{,"id":1}`, `{"id":1,,"x":2}`,
	`{"id" 1}`, `{"id":1 "x":2}`, `{id:1}`, `{'id':1}`, `{"id":1}garbage`, "{\"id\":1}\n", "\t\r\n {\"id\" \t:\r\n 1 \n}\n ",
	// Numbers.
	`{"id":0}`, `{"id":-0}`, `{"id":00}`, `{"id":01}`, `{"id":-1}`, `{"id":+1}`, `{"id":1.0}`, `{"id":1e2}`, `{"id":1E2}`,
	`{"id":4294967295}`, `{"id":4294967296}`, `{"id":"1"}`, `{"id":null}`, `{"id":true}`, `{"id":[]}`, `{"id":{}}`,
	`{"id":1.}`, `{"id":.5}`, `{"id":1e}`, `{"id":1e+}`, `{"id":-}`, `{"id":1x}`, `{"id":0x10}`, `{"id": 12 }`,
	`{"offset":18446744073709551615}`, `{"offset":18446744073709551616}`, `{"offset":-1}`, `{"offset":1.5}`,
	`{"time":9223372036854775807}`, `{"time":9223372036854775808}`, `{"time":-9223372036854775808}`, `{"time":-9223372036854775809}`,
	`{"time":-0}`, `{"time":-01}`, `{"time":-}`, `{"time":--1}`, `{"time":1e3}`, `{"time":null}`,
	`{"code":2147483647}`, `{"code":2147483648}`, `{"code":-2147483648}`, `{"code":-2147483649}`, `{"ttl":4294967295}`,
	// Booleans.
	`{"recover":true}`, `{"recover":false}`, `{"recover":null}`, `{"recover":1}`, `{"recover":"true"}`, `{"recover":tru}`,
	`{"recover":truex}`, `{"recover":True}`, `{"recover":falsey}`, `{"delta":true,"delta":false}`,
	// Strings.
	`{"channel":""}`, `{"channel":"a"}`, `{"channel":"1234567"}`, `{"channel":"12345678"}`, `{"channel":"123456789"}`,
	`{"channel":"a\"b"}`, `{"channel":"a\\b"}`, `{"channel":"a\/b"}`, `{"channel":"\b\f\n\r\t"}`, `{"channel":"\u0041"}`,
	`{"channel":"\u00e9"}`, `{"channel":"\ud83d\ude00"}`, `{"channel":"\ud83d"}`, `{"channel":"\ude00"}`, `{"channel":"\ud83dx"}`,
	`{"channel":"\ud83d\u0041"}`, `{"channel":"\ud83d\ud83d\ude00"}`, `{"channel":"\u12"}`, `{"channel":"\u12G4"}`, `{"channel":"\x"}`,
	`{"channel":"\`, `{"channel":"\"}`, `{"channel":"abc`, `{"channel":"a` + "\n" + `b"}`, `{"channel":"a` + "\t" + `b"}`,
	`{"channel":"a` + "\x00" + `b"}`, `{"channel":"a` + "\x7f" + `b"}`, "{\"channel\":\"h\xc3\xa9llo\"}", "{\"channel\":\"bad\xffutf8\"}",
	"{\"channel\":\"trunc\xc3\"}", "{\"channel\":\"\xed\xa0\x80\"}", "{\"channel\":\"\xf0\x9f\x98\x80\"}", "{\"channel\":\"esc\\n\xffmix\"}",
	`{"channel":null}`, `{"channel":1}`, `{"channel":{}}`, `{"channel":["a"]}`, `{"channel":"a","channel":"b"}`, `{"channel":"a","channel":null}`,
	`{"channel":'a'}`, `{"channel":"<b>&amp;</b>"}`, `{"channel":"a long channel name, longer than sixteen bytes"}`,
	// Keys.
	`{"ID":1}`, `{"Id":1}`, `{"iD":1,"id":2}`, `{"id":2,"ID":1}`, `{"\u0069d":1}`, `{"\u0049D":1}`, `{"id\u0000":1}`, `{"":1}`,
	`{"CHANNEL":"a"}`, `{"Channel":"a"}`, "{\"\xc5\xbfubscribe\":{}}", "{\"pu\xc5\xbfh\":{}}", "{\"\xe2\x84\xaaey\":\"v\"}",
	`{"unknown":1}`, `{"unknown":"s"}`, `{"unknown":[1,{"a":[]}]}`, `{"unknown":{"a":{"b":{"c":null}}}}`, `{"unknown":tru}`,
	`{"unknown":[1,]}`, `{"unknown":{"a"}}`, `{"unknown":{"a":}}`, `{"unknown":[}`, `{"unknown":{]}`, `{"unknown":01}`,
	`{"unknown":"a` + "\n" + `"}`, "{\"unknown\":\"\xff\"}", `{"unknown":"\ud83d"}`, `{"unknown":"\uZZZZ"}`, `{null:1}`, `{1:1}`, `{"a":1,null:2}`,
	// Raw values.
	`{"data":{}}`, `{"data":null}`, `{"data":"s"}`, `{"data":1}`, `{"data":[1, 2 ,3]}`, `{"data": {"a" : 1} }`, `{"data":{"a":"\u00e9\n"}}`,
	"{\"data\":{\n\"a\":1\n}}", `{"data":}`, `{"data":{}`, `{"data":{a}}`, `{"data":01}`, `{"data":1.}`, `{"data":nul}`, `{"data":nullx}`,
	"{\"data\":\"\xff\"}", `{"data":{"a":1},"data":[2]}`, `{"data":{"a":1},"data":null}`, `{"data":"` + "\x01" + `"}`, `{"data":[[[[[[[[[[]]]]]]]]]]}`,
	`{"data":{"a":{"b":[{"c":[{}]}]}}}`, `{"data":[1,[2,[3,{"a":[4]}]],5]}`, `{"data":[}`, `{"data":{]}`, `{"data":[1 2]}`, `{"data":{"a":1 "b":2}}`,
	`{"data":{"a":1,}}`, `{"data":[1,]}`, `{"data":[,1]}`, `{"data":{,}}`, `{"data":-}`, `{"data":1e5}`, `{"data":-0.0e-0}`, `{"data":tRue}`,
	// Nested messages.
	`{"connect":{}}`, `{"connect":null}`, `{"connect":1}`, `{"connect":[]}`, `{"connect":"x"}`, `{"connect":{"token":"t"},"connect":{"name":"n"}}`,
	`{"connect":{"token":"t"},"connect":null}`, `{"connect":null,"connect":{"name":"n"}}`, `{"publish":{"channel":"c","data":{"a":1}}}`,
	`{"push":{"pub":{"data":{},"info":{"user":"u","client":"c"},"tags":{"a":"b"}}}}`, `{"push":{"pub":{"info":null}}}`,
	// Maps.
	`{"subs":{}}`, `{"subs":null}`, `{"subs":[]}`, `{"subs":{"a":{}}}`, `{"subs":{"a":null}}`, `{"subs":{"a":{},"a":{"recover":true}}}`,
	`{"subs":{"a":{"recover":true}},"subs":{"b":{}}}`, `{"subs":{"a":{}},"subs":null}`, `{"subs":{"a":1}}`, `{"subs":{a:{}}}`, `{"subs":{"a":{},}}`,
	`{"subs":{null:{}}}`, `{"subs":{"\u0061":{}}}`, "{\"subs\":{\"\xff\":{}}}", `{"subs":{"":{}}}`, `{"subs":{"a" : { } , "b" : { } } }`,
	`{"tags":{}}`, `{"tags":null}`, `{"tags":{"a":"b"}}`, `{"tags":{"a":null}}`, `{"tags":{"a":1}}`, `{"tags":{"a":"b","a":"c"}}`, `{"tags":{"a":"b",}}`,
	`{"tags":{"a":"b"},"tags":{"c":"d"}}`, `{"tags":{"a"}}`, `{"tags":{"a":}}`, `{"tags":"x"}`, `{"headers":{"X-A":"1","x-a":"2"}}`,
	`{"presence":{"c1":{"user":"u","client":"c1"}}}`, `{"presence":{}}`, `{"presence":null}`,
	// Slices.
	`{"publications":[]}`, `{"publications":null}`, `{"publications":{}}`, `{"publications":[{}]}`, `{"publications":[null]}`, `{"publications":[{},null,{"offset":1}]}`,
	`{"publications":[{},]}`, `{"publications":[,{}]}`, `{"publications":[{} {}]}`, `{"publications":[1]}`, `{"publications":[{}],"publications":[]}`,
	`{"publications":[{}],"publications":null}`, `{"publications":[{"offset":1},{"offset":2}],"publications":[{"offset":3}]}`, `{"publications": [ { } , { } ] }`,
	`{"publications":[`, `{"publications":[{}`, `{"channels":[]}`, `{"channels":["a","b"]}`, `{"channels":[null]}`, `{"channels":["a",1]}`, `{"channels":"a"}`,
	`{"channels":["a","b"],"channels":["c"]}`, `{"items":[{"key":"k","data":{}}]}`, `{"state":[{"data":1}],"state":[null]}`,
}

func TestDecodeMatchesSegmentio_EdgeCases(t *testing.T) {
	for _, input := range edgeCases {
		for _, newMsg := range allTypes {
			compareDecode(t, []byte(input), newMsg())
		}
	}
}

// Each input is decoded into a message type picked by its first byte.
func FuzzDecodeMatchesSegmentio(f *testing.F) {
	for n, input := range edgeCases {
		f.Add(byte(n), []byte(input))
	}
	r := rand.New(rand.NewSource(5))
	for n := 0; n < 300; n++ {
		m := randomMessage(r, 3)
		for typ, newMsg := range allTypes {
			if reflect.TypeOf(newMsg()) == reflect.TypeOf(m) {
				f.Add(byte(typ), m.AppendJSON(nil))
			}
		}
	}
	f.Fuzz(func(t *testing.T, typ byte, data []byte) {
		compareDecode(t, data, allTypes[int(typ)%len(allTypes)]())
	})
}

// protocol validates every Reply and Push it encodes, since payloads come
// from the application. cfjson.Valid replaced segmentio's Valid there, so
// they must agree on every input which is valid UTF-8.
func FuzzValidMatchesSegmentio(f *testing.F) {
	for _, input := range edgeCases {
		f.Add([]byte(input))
	}
	for _, raw := range sampleRaws {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if got, want := cfjson.Valid(data), segmentio.Valid(data); got != want && !validDiffers(data, got, want) {
			t.Fatalf("Valid(%q) = %v, segmentio says %v", data, got, want)
		}
	})
}

func TestValidMatchesSegmentio(t *testing.T) {
	r := rand.New(rand.NewSource(6))
	check := func(data []byte) {
		if got, want := cfjson.Valid(data), segmentio.Valid(data); got != want && !validDiffers(data, got, want) {
			t.Fatalf("Valid(%q) = %v, segmentio says %v", data, got, want)
		}
	}
	for _, input := range edgeCases {
		check([]byte(input))
	}
	for n := 0; n < 50000; n++ {
		data := randomMessage(r, 3).AppendJSON(nil)
		check(data)
		// Damage the message in one place.
		if len(data) > 0 {
			pos := r.Intn(len(data))
			switch r.Intn(3) {
			case 0:
				data[pos] = byte(r.Intn(256))
			case 1:
				data = append(data[:pos], data[pos+1:]...)
			case 2:
				data = data[:pos]
			}
			check(data)
		}
	}
}

// newString encodes a string with cfjson twice, into a buffer without room
// and into one with room, which are different paths for short strings. Both
// must give the same result.
func newString(t *testing.T, s string) string {
	t.Helper()
	tight := string(cfjson.AppendString(nil, s))
	if roomy := string(cfjson.AppendString(make([]byte, 0, len(s)*6+16), s)); roomy != tight {
		t.Fatalf("%q: %s into a buffer with room, %s into one without", s, roomy, tight)
	}
	return tight
}

// oldString is how the old writer encoded a string.
func oldString(s string) string {
	w := newWriter()
	w.String(s)
	res, err := w.BuildBytes()
	if err != nil {
		panic(err)
	}
	return string(res)
}

// String escaping must be what the old writer did, exhaustively: every byte
// value at every position of strings around the sizes the fast path works in,
// and every Unicode code point.
func TestStringEscapingMatchesOldWriter(t *testing.T) {
	for c := 0; c < 256; c++ {
		for size := 1; size <= 40; size++ {
			for pos := 0; pos < size; pos++ {
				in := bytes.Repeat([]byte("a"), size)
				in[pos] = byte(c)
				if got, want := newString(t, string(in)), oldString(string(in)); got != want {
					t.Fatalf("byte %#x at %d of %d: got %s, want %s", c, pos, size, got, want)
				}
			}
		}
	}
	// Every pair of bytes next to each other: covers truncated and invalid
	// multi-byte sequences.
	for c1 := 0; c1 < 256; c1++ {
		for c2 := 0; c2 < 256; c2++ {
			in := "ab" + string([]byte{byte(c1), byte(c2)}) + "cdefghij"
			if got, want := newString(t, in), oldString(in); got != want {
				t.Fatalf("bytes %#x %#x: got %s, want %s", c1, c2, got, want)
			}
		}
	}
	for r := rune(0); r <= 0x10FFFF; r++ {
		in := "x" + string(r) + "y"
		if got, want := newString(t, in), oldString(in); got != want {
			t.Fatalf("rune %#x: got %s, want %s", r, got, want)
		}
	}
}

// With -fold-keys generated decoders match keys the way segmentio does by
// default. Messages are decoded with their keys as encoders write them and
// with every key capitalized, and both decoders must make the same message of
// each. (A key which repeats in another case is an error for cfjson only, see
// README.md, and does not occur here.)
func TestFoldKeysMatchesSegmentio(t *testing.T) {
	r := rand.New(rand.NewSource(8))
	for n := 0; n < 50000; n++ {
		m := randomMessage(r, 3)
		exact := m.AppendJSON(nil)
		for _, data := range [][]byte{exact, capitalizeKeys(exact)} {
			for _, zeroCopy := range []bool{false, true} {
				oldMsg, newMsg := zero(m), zero(m)
				oldErr := segmentioDecode(bytes.Clone(data), oldMsg, zeroCopy)
				newErr := foldDecode(bytes.Clone(data), newMsg, zeroCopy)
				if oldErr != nil || newErr != nil {
					// Map keys are capitalized as well, which may make two
					// of them the same key.
					if oldErr == nil && hasRepeatedKey(data) {
						continue
					}
					t.Fatalf("%T input %q:\nold error: %v\nnew error: %v", m, data, oldErr, newErr)
				}
				if !reflect.DeepEqual(oldMsg, newMsg) {
					t.Fatalf("%T input %q:\nold: %s\nnew: %s", m, data, dump(oldMsg), dump(newMsg))
				}
			}
		}
	}
}
