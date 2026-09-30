package testtypes

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"math"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/centrifugal/protocol/cfjson"
	"github.com/centrifugal/protocol/cfjson/gen"
)

// MarshalJSON and UnmarshalJSON make Raw behave like json.RawMessage for
// encoding/json, which the generated code is compared to.
func (r Raw) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return bytes.ReplaceAll(r, []byte("\n"), nil), nil
}

func (r *Raw) UnmarshalJSON(data []byte) error {
	*r = append((*r)[:0], data...)
	return nil
}

type value interface {
	AppendJSON([]byte) []byte
	DecodeJSON([]byte, int, cfjson.Flags) int
}

var constructors = []func() value{
	func() value { return new(All) },
	func() value { return new(Required) },
	func() value { return new(Mixed) },
	func() value { return new(Leaf) },
	func() value { return new(Empty) },
}

func decode(data []byte, v value, f cfjson.Flags) error {
	return cfjson.Error(data, v.DecodeJSON(data, cfjson.SkipSpace(data, 0), f))
}

// The generated file must be what the generator produces today.
func TestGeneratedCodeIsUpToDate(t *testing.T) {
	want, err := gen.Generate(gen.Config{Files: []string{"types.go"}, RawTypes: []string{"Raw"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("types_cfjson.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("types_cfjson.go is out of date, run go generate")
	}
}

func TestEncodeExact(t *testing.T) {
	s := "p"
	n := int64(-5)
	tests := []struct {
		v    value
		want string
	}{
		{&Empty{}, `{}`},
		{&Leaf{}, `{}`},
		{&Leaf{ID: 7}, `{"id":7}`},
		{&Leaf{Text: "t"}, `{"text":"t"}`},
		{&Leaf{ID: 7, Text: "t"}, `{"id":7,"text":"t"}`},
		{&Mixed{}, `{"b":""}`},
		{&Mixed{A: "a"}, `{"a":"a","b":""}`},
		{&Mixed{C: "c"}, `{"b":"","c":"c"}`},
		{&Mixed{A: "a", B: "b", C: "c"}, `{"a":"a","b":"b","c":"c"}`},
		{&All{}, `{"leaf":{},"Untagged":""}`},
		{&Required{}, `{"string":"","bool":false,"int64":0,"uint32":0,"float64":0,"raw":null,"leaf":{},"leaf_ptr":null,` +
			`"str_ptr":null,"strings":null,"leaf_ptrs":null,"str_map":null,"leaf_map":null}`},
		{&Required{Raw: Raw{}, Strings: []string{}, LeafPtrs: []*Leaf{}, StrMap: map[string]string{}, LeafMap: map[string]*Leaf{}},
			`{"string":"","bool":false,"int64":0,"uint32":0,"float64":0,"raw":null,"leaf":{},"leaf_ptr":null,` +
				`"str_ptr":null,"strings":[],"leaf_ptrs":[],"str_map":{},"leaf_map":{}}`},
		{&Required{String: "s", Bool: true, Int64: -1, Uint32: 2, Float64: 1.5, Raw: Raw("[1,\n2]"), Leaf: Leaf{ID: 1},
			LeafPtr: &Leaf{Text: "x"}, StrPtr: &s, Strings: []string{"a", "b"}, LeafPtrs: []*Leaf{nil, {ID: 3}},
			StrMap: map[string]string{"k": "v"}, LeafMap: map[string]*Leaf{"n": nil}},
			`{"string":"s","bool":true,"int64":-1,"uint32":2,"float64":1.5,"raw":[1,2],"leaf":{"id":1},"leaf_ptr":{"text":"x"},` +
				`"str_ptr":"p","strings":["a","b"],"leaf_ptrs":[null,{"id":3}],"str_map":{"k":"v"},"leaf_map":{"n":null}}`},
		{&All{Bool: true, Int8: -8, Uint64: math.MaxUint64, Float32: 0.25, Level: 3, Name: "n", Labels: Labels{"a": "b"},
			Names: Names{"x", "y"}, StrPtr: &s, IntPtr: &n, Ints: []int64{1, -2}, Floats: []float64{0.5}, Bools: []bool{true, false},
			Raws: []Raw{Raw(`{}`), nil}, Leaves: []Leaf{{ID: 1}, {}}, Matrix: [][]uint32{{1, 2}, nil, {}},
			FloatMap: map[string]float64{"f": 2.5}, ListMap: map[string][]string{"l": {"a"}}, NameMap: map[Name]Level{"nm": 9},
			Skipped: "skipped", hidden: "hidden", MixedCase: "mc", Untagged: "u"},
			`{"bool":true,"int8":-8,"uint64":18446744073709551615,"float32":0.25,"level":3,"name":"n","labels":{"a":"b"},` +
				`"names":["x","y"],"leaf":{},"str_ptr":"p","int_ptr":-5,"ints":[1,-2],"floats":[0.5],"bools":[true,false],` +
				`"raws":[{},null],"leaves":[{"id":1},{}],"matrix":[[1,2],null,[]],"float_map":{"f":2.5},"list_map":{"l":["a"]},` +
				`"name_map":{"nm":9},"mixedCase":"mc","Untagged":"u"}`},
	}
	for _, tt := range tests {
		if got := string(tt.v.AppendJSON(nil)); got != tt.want {
			t.Errorf("%T:\n got %s\nwant %s", tt.v, got, tt.want)
		}
	}
}

func TestDecodeExact(t *testing.T) {
	var a All
	input := ` { "string" : "s" , "bool":true, "int8":-128, "uint8":255, "float32":1e2, "raw": {"a" : 1} , "level":2, "name":"n",
		"labels":{"a":"b"}, "names":["x"], "leaf":{"id":1}, "leaf_ptr":{"text":"t"}, "self":{"self":{"string":"deep"}},
		"str_ptr":"p", "int_ptr":null, "strings":[], "matrix":[[1],[],null], "leaf_ptrs":[null,{}], "name_map":{"k":1},
		"list_map":{"l":["a","b"]}, "mixedCase":"mc", "Untagged":"u", "-":"x", "Skipped":"x", "hidden":"x", "unknown":[{"a":null}],
		"STRING":"wrong case", "Bool":false, "mixedcase":"wrong case", "untagged":"wrong case", "unknown":"twice is fine" } `
	if err := decode([]byte(input), &a, 0); err != nil {
		t.Fatal(err)
	}
	s := "p"
	want := All{String: "s", Bool: true, Int8: -128, Uint8: 255, Float32: 100, Raw: Raw(`{"a" : 1}`), Level: 2, Name: "n",
		Labels: Labels{"a": "b"}, Names: Names{"x"}, Leaf: Leaf{ID: 1}, LeafPtr: &Leaf{Text: "t"},
		Self: &All{Self: &All{String: "deep"}}, StrPtr: &s, Strings: []string{}, Matrix: [][]uint32{{1}, {}, nil},
		LeafPtrs: []*Leaf{nil, {}}, NameMap: map[Name]Level{"k": 1}, ListMap: map[string][]string{"l": {"a", "b"}},
		MixedCase: "mc", Untagged: "u"}
	if !reflect.DeepEqual(a, want) {
		t.Fatalf("got %+v", a)
	}
}

// A key may come once in an object: a parser which takes the first of two
// and one which takes the last read different messages out of the same
// bytes.
func TestDecodeRejectsRepeatedKeys(t *testing.T) {
	for _, input := range []string{
		`{"string":"a","string":"b"}`,
		`{"string":"a","int":1,"string":"a"}`,
		`{"bool":true,"bool":true}`,
		`{"raw":1,"raw":2}`,
		`{"leaf":{},"leaf":{}}`,
		`{"leaf_ptr":{},"leaf_ptr":null}`,
		`{"leaf_ptr":null,"leaf_ptr":{}}`,
		`{"strings":[],"strings":[]}`,
		`{"str_map":{},"str_map":{}}`,
		`{"leaf":{"id":1,"id":2}}`,
		`{"self":{"self":{"int":1,"int":1}}}`,
		`{"leaves":[{"text":"a","text":"b"}]}`,
		`{"str_map":{"k":"a","k":"b"}}`,
		`{"leaf_map":{"k":{},"k":null}}`,
		`{"name_map":{"k":1,"other":2,"k":3}}`,
		`{"mixedCase":"a","mixedCase":"b"}`,
		`{"Untagged":"a","Untagged":"b"}`,
		// The last field of All, which is past the 32nd.
		`{"Untagged":"a","string":"s","Untagged":"b"}`,
	} {
		var de *cfjson.DecodeError
		if err := decode([]byte(input), new(All), 0); !errors.As(err, &de) || de.Syntax {
			t.Errorf("%s: got %v, want a DecodeError which is not a syntax error", input, err)
		}
	}
	// The same key in different objects is not a repetition.
	a := new(All)
	input := `{"string":"a","self":{"string":"b"},"leaf":{"id":1},"leaf_ptr":{"id":1},"leaves":[{"id":1},{"id":1}],` +
		`"str_map":{"k":"v"},"leaf_map":{"k":{"id":1},"j":{"id":1}}}`
	if err := decode([]byte(input), a, 0); err != nil {
		t.Fatal(err)
	}
	// A map is replaced by the one decoded, not added to.
	a = &All{StrMap: map[string]string{"old": "v", "k": "old"}}
	if err := decode([]byte(`{"str_map":{"k":"new"}}`), a, 0); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.StrMap, map[string]string{"k": "new"}) {
		t.Fatalf("got %v", a.StrMap)
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, input := range []string{
		``, `[`, `{"int8":128}`, `{"int8":-129}`, `{"uint8":256}`, `{"uint8":-1}`, `{"int16":40000}`, `{"uint16":70000}`,
		`{"int32":2147483648}`, `{"uint":1.5}`, `{"float32":1e39}`, `{"float64":1e400}`, `{"float64":"1"}`, `{"float64":.5}`,
		`{"matrix":[1]}`, `{"matrix":[[1,]]}`, `{"strings":[1]}`, `{"name_map":{"a":"b"}}`, `{"leaf":[]}`, `{"bools":[1]}`,
		`{"str_ptr":1}`, `{"labels":[]}`, `{"names":{}}`, `{"level":"x"}`, `{"list_map":{"a":"b"}}`, `{"leaves":[{]}`,
	} {
		var a All
		err := decode([]byte(input), &a, 0)
		if err == nil {
			t.Errorf("%s: no error", input)
			continue
		}
		var de *cfjson.DecodeError
		if !reflectAs(err, &de) || de.Offset < 0 || de.Offset > len(input) {
			t.Errorf("%s: bad error %#v", input, err)
		}
		if want := !stdjson.Valid([]byte(input)); de.Syntax != want {
			t.Errorf("%s: Syntax = %v, want %v", input, de.Syntax, want)
		}
	}
}

func reflectAs(err error, target **cfjson.DecodeError) bool {
	return errors.As(err, target)
}

var strs = []string{"", "a", "plain text", `q"\`, "line\nbreak", "<&>", "\xc3\xa9\xf0\x9f\x98\x80", "bad\xff", "\x00\x1f",
	strings.Repeat("long ", 30)}
var raws = []string{`{}`, `[1, 2]`, `"s"`, `null`, `1.5e3`, "{\n\"a\":true\n}", `{"n":{"m":[{}]}}`}
var floats = []float64{0, 1, -1, 0.5, 1e21, 1e-7, 123456.789, math.MaxFloat64, math.SmallestNonzeroFloat64, 1 << 53}

func fill(r *rand.Rand, v reflect.Value, depth int) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			// Fields generated code ignores stay empty, so that values can be
			// compared after a round trip.
			if f := v.Type().Field(i); f.IsExported() && f.Tag.Get("json") != "-" {
				fill(r, v.Field(i), depth+1)
			}
		}
	case reflect.Pointer:
		if depth < 6 && r.Intn(3) == 0 {
			v.Set(reflect.New(v.Type().Elem()))
			fill(r, v.Elem(), depth+1)
		}
	case reflect.String:
		if r.Intn(3) != 0 {
			v.SetString(strs[r.Intn(len(strs))])
		}
	case reflect.Bool:
		v.SetBool(r.Intn(2) == 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := []int64{0, 1, -1, math.MaxInt64, math.MinInt64, r.Int63(), int64(r.Intn(1000)) - 500}[r.Intn(7)]
		v.SetInt(n >> (64 - v.Type().Bits()) << 0)
		if r.Intn(2) == 0 {
			v.SetInt(n % 100)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n := []uint64{0, 1, 9, 10, math.MaxUint64, r.Uint64()}[r.Intn(6)]
		v.SetUint(n >> (64 - v.Type().Bits()))
	case reflect.Float32:
		v.SetFloat(float64(float32(floats[r.Intn(6)])))
	case reflect.Float64:
		v.SetFloat(floats[r.Intn(len(floats))])
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			switch r.Intn(4) {
			case 0:
			case 1:
				v.SetBytes([]byte{})
			default:
				v.SetBytes([]byte(raws[r.Intn(len(raws))]))
			}
			return
		}
		if n := r.Intn(5) - 1; n >= 0 {
			v.Set(reflect.MakeSlice(v.Type(), n, n))
			for i := 0; i < n; i++ {
				fill(r, v.Index(i), depth+1)
			}
		}
	case reflect.Map:
		if n := r.Intn(5) - 1; n >= 0 {
			v.Set(reflect.MakeMap(v.Type()))
			for ; n > 0; n-- {
				key, elem := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
				key.SetString(strs[r.Intn(len(strs))])
				fill(r, elem, depth+1)
				v.SetMapIndex(key, elem)
			}
		}
	}
}

// Generated encoders must produce JSON which means what encoding/json
// produces for the same value, and generated decoders must decode the output
// of both to what encoding/json decodes it to.
func TestMatchesEncodingJSON(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < 30000; n++ {
		newValue := constructors[r.Intn(len(constructors))]
		v := newValue()
		fill(r, reflect.ValueOf(v).Elem(), 0)

		ours := v.AppendJSON(nil)
		std, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var oursDoc, stdDoc any
		if err := stdjson.Unmarshal(ours, &oursDoc); err != nil {
			t.Fatalf("%v: %s", err, ours)
		}
		if err := stdjson.Unmarshal(std, &stdDoc); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(oursDoc, stdDoc) {
			t.Fatalf("%T:\nours %s\n std %s", v, ours, std)
		}

		for _, data := range [][]byte{ours, std} {
			want := newValue()
			if err := stdjson.Unmarshal(data, want); err != nil {
				t.Fatal(err)
			}
			for _, f := range []cfjson.Flags{0, cfjson.ZeroCopy} {
				got := newValue()
				if err := decode(data, got, f); err != nil {
					t.Fatalf("%v: %s", err, data)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%T decoded from %s:\n got %+v\nwant %+v", v, data, got, want)
				}
			}
		}
	}
}

// Without ZeroCopy nothing decoded may point into the input.
func TestDecodeCopies(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for n := 0; n < 5000; n++ {
		v := new(All)
		fill(r, reflect.ValueOf(v).Elem(), 0)
		data := v.AppendJSON(nil)
		got, want := new(All), new(All)
		if err := decode(data, got, 0); err != nil {
			t.Fatal(err)
		}
		if err := decode(bytes.Clone(data), want, 0); err != nil {
			t.Fatal(err)
		}
		for i := range data {
			data[i] = 'X'
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("decoded value changed together with the input")
		}
	}
}

// All can hold another All, so input decides how deep the decoder recurses:
// there must be a limit, and it must not be hit by anything else.
func TestDecodeDepthLimit(t *testing.T) {
	nested := func(depth int) []byte {
		return []byte(strings.Repeat(`{"self":`, depth-1) + `{"string":"bottom"}` + strings.Repeat(`}`, depth-1))
	}
	var a All
	if err := decode(nested(cfjson.MaxDepth), &a, 0); err != nil {
		t.Fatal(err)
	}
	depth := 1
	for p := &a; p.Self != nil; p = p.Self {
		depth++
	}
	if depth != cfjson.MaxDepth {
		t.Fatalf("decoded %d levels", depth)
	}
	err := decode(nested(cfjson.MaxDepth+1), new(All), 0)
	if err == nil {
		t.Fatal("no error for a value nested deeper than MaxDepth")
	}
	var de *cfjson.DecodeError
	if !errors.As(err, &de) || de.Syntax {
		t.Fatalf("got %v, want a DecodeError which is not a syntax error", err)
	}
	// Much deeper input must fail the same way rather than overflow the stack.
	if err := decode(nested(2000000), new(All), 0); err == nil {
		t.Fatal("no error for a very deep value")
	}
	// Siblings do not add up, only nesting does.
	wide := []byte(`{"leaves":[` + strings.Repeat(`{"id":1},`, 50000) + `{}]}`)
	if err := decode(wide, new(All), 0); err != nil {
		t.Fatal(err)
	}
	// Neither does nesting inside values which are skipped or kept raw.
	deepRaw := strings.Repeat(`[`, 100000) + strings.Repeat(`]`, 100000)
	if err := decode([]byte(`{"raw":`+deepRaw+`,"unknown":`+deepRaw+`}`), &a, 0); err != nil {
		t.Fatal(err)
	}
	if string(a.Raw) != deepRaw {
		t.Fatal("raw value mismatch")
	}
}

func TestUnmarshal(t *testing.T) {
	var leaf Leaf
	if err := cfjson.Unmarshal([]byte(" {\"id\":7,\"text\":\"t\"} \n"), &leaf, 0); err != nil {
		t.Fatal(err)
	}
	if leaf != (Leaf{ID: 7, Text: "t"}) {
		t.Fatalf("got %+v", leaf)
	}
	for _, input := range []string{``, ` `, `{`, `{"id":"x"}`, `[]`, `"s"`} {
		var de *cfjson.DecodeError
		if err := cfjson.Unmarshal([]byte(input), new(Leaf), 0); !reflectAs(err, &de) {
			t.Errorf("%q: got %v, want a DecodeError", input, err)
		}
	}
	for _, input := range []string{`{}x`, `{} {}`, `{"id":1},`} {
		if err := cfjson.Unmarshal([]byte(input), new(Leaf), 0); !errors.Is(err, cfjson.ErrTrailingData) {
			t.Errorf("%q: got %v, want ErrTrailingData", input, err)
		}
	}
	// Every generated type is both an Appender and a Decoder.
	var _ cfjson.Appender = (*All)(nil)
	var _ cfjson.Decoder = (*All)(nil)
}
