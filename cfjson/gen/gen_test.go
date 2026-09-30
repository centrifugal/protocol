package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generate(t *testing.T, cfg Config, src string) (string, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Files = []string{file}
	out, err := Generate(cfg)
	return string(out), err
}

// What the generator cannot handle must be an error which names the field,
// not code which silently skips it.
func TestGenerate_Unsupported(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"bytes", "package p\ntype T struct{ B []byte }", "T.B: unsupported type []byte"},
		{"interface", "package p\ntype T struct{ V any }", "T.V: unsupported type any"},
		{"interface of another package", "package p\nimport \"io\"\ntype T struct{ R io.Reader }", "T.R: unsupported type"},
		{"type another package does not have", "package p\nimport \"time\"\ntype T struct{ At time.Missing }", "T.At: unsupported type time.Missing"},
		{"unexported type of another package", "package p\nimport \"time\"\ntype T struct{ Z time.zone }", "is not exported"},
		{"array", "package p\ntype T struct{ A [4]int }", "T.A: unsupported array type [4]int"},
		{"map key", "package p\ntype T struct{ M map[int]string }", "T.M: unsupported map key type"},
		{"embedded type which is not a struct", "package p\ntype E []string\ntype T struct{ E }", "T.E: only structs without JSON methods of their own can be embedded"},
		{"embedded type with JSON methods", "package p\nimport \"time\"\ntype T struct{ time.Time }", "T.Time: only structs without JSON methods of their own can be embedded"},
		{"string option", "package p\ntype T struct{ N int `json:\"n,string\"` }", `T: unsupported json tag option "string"`},
		{"duplicate name", "package p\ntype T struct{ A int `json:\"x\"`; B int `json:\"x\"` }", `T: duplicate JSON field name "x"`},
		{"type which encodes itself as text", "package p\nimport \"net/netip\"\ntype T struct{ A netip.Addr }", "T.A: type netip.Addr encodes itself as text"},
		{"struct of another package without exported fields", "package p\nimport \"sync\"\ntype T struct{ M sync.Mutex }", "type sync.Mutex has no exported fields"},
		{"slice of a named byte type", "package p\ntype B uint8\ntype T struct{ V []B }", "T.V: unsupported type []B, a slice of bytes"},
		{"nested unsupported", "package p\ntype T struct{ M map[string][]chan int }", "T.M: unsupported type chan int"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generate(t, Config{}, tt.src)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got error %v, want one with %q", err, tt.want)
			}
		})
	}
}

func TestGenerate_Options(t *testing.T) {
	src := `package p

type Payload []byte

type A struct {
	Data Payload ` + "`json:\"data\"`" + `
	B    *B
	skip int
	Omit string ` + "`json:\"-\"`" + `
}

type B struct{ N int }

type Generic[T any] struct{ V T }
`
	out, err := generate(t, Config{
		Types: []string{"A"}, RawTypes: []string{"Payload"}, AppendMethod: "appendJSON", DecodeMethod: "parseJSON", Runtime: "example.com/cfjson",
	}, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`cfjson "example.com/cfjson"`,
		"func (m *A) appendJSON(b []byte) []byte {",
		"func (m *A) parseJSON(b []byte, i int, f cfjson.Flags) int {",
		"cfjson.AppendRaw(b, m.Data)",
		"m.B.parseJSON(b, i, f)",
		"func (m *B) parseJSON(",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the output", want)
		}
	}
	for _, unwanted := range []string{"Generic", "skip", "Omit"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("%q in the output", unwanted)
		}
	}

	if _, err := generate(t, Config{Types: []string{"Missing"}}, src); err == nil {
		t.Error("no error for a type which is not declared")
	}
	if _, err := Generate(Config{}); err == nil {
		t.Error("no error without input files")
	}
}

// Only recursive types pay for the depth check.
func TestGenerate_Recursive(t *testing.T) {
	out, err := generate(t, Config{}, `package p

type Tree struct {
	Children []*Tree
	Leaf     Leaf
}

type Leaf struct{ N int }

type Ping struct{ Pong *Pong }

type Pong struct{ Pings map[string]Ping }
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out, "cfjson.DepthStep"); got != 3 {
		t.Fatalf("%d depth checks, want 3 (Tree, Ping, Pong)", got)
	}
	_, leaf, found := strings.Cut(out, "func (m *Leaf) DecodeJSON")
	if !found {
		t.Fatal("no decoder for Leaf")
	}
	leaf, _, _ = strings.Cut(leaf, "\n}\n")
	if strings.Contains(leaf, "DepthStep") {
		t.Fatal("depth check in a type which is not recursive")
	}
}

// Types which are not asked for, of this package and of other ones, get code
// generated when a type which is asked for needs it.
func TestGenerate_Referenced(t *testing.T) {
	src := `package p

import (
	"encoding/json"
	"image"
	"time"
)

type Top struct {
	Inner  Inner
	Manual *Manual
	At     time.Time
	Point  image.Point
	Raw    json.RawMessage
	image.Rectangle
}

type Inner struct{ N int }

type Manual struct{ N int }

func (m *Manual) AppendJSON(b []byte) []byte { return b }

func (m *Manual) DecodeJSON(b []byte, i int, f uint32) int { return i }

type Unused struct{ N int }
`
	out, err := generate(t, Config{Types: []string{"Top"}}, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func (m *Top) AppendJSON(",
		// A struct of this package gets methods.
		"func (m *Inner) AppendJSON(",
		"func (m *Inner) DecodeJSON(",
		// A struct of another package gets functions.
		"func appendJSONImagePoint(b []byte, m *image.Point) []byte",
		"func decodeJSONImagePoint(m *image.Point, b []byte, i int, f cfjson.Flags) int",
		// A type with JSON methods of its own is left to them.
		"cfjson.AppendMarshaler(b, &m.At)",
		"cfjson.DecodeUnmarshaler(b, i, f, &m.At)",
		"cfjson.AppendRaw(b, m.Raw)",
		// The fields of an embedded struct are fields of the struct.
		"m.Rectangle.Min",
		`"image"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in generated code", want)
		}
	}
	for _, not := range []string{
		// Methods written by hand are used, not generated again.
		"func (m *Manual)",
		"func (m *Unused)",
		// Nothing is generated for a struct which is only embedded.
		"ImageRectangle",
		// Packages the generated code does not name are not imported.
		`"encoding/json"`,
		`"time"`,
	} {
		if strings.Contains(out, not) {
			t.Errorf("%q in generated code", not)
		}
	}
}

// A method makes a type encode itself wherever in the package it is written,
// not only in the files given to the generator, and whoever wrote it: a tool
// like enumer generates exactly the methods a type is meant to have. What
// easyjson generated does not count, it is what this generator replaces.
func TestGenerate_MethodsInOtherFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	const methods = "func (v %[1]s) MarshalJSON() ([]byte, error) { return nil, nil }\n\nfunc (v *%[1]s) UnmarshalJSON([]byte) error { return nil }\n"
	types := write("types.go", "package p\n\ntype Level int\n\ntype Color int\n\ntype Old struct{ N int }\n\ntype T struct {\n\tL Level\n\tC Color\n\tO Old\n}\n")
	write("methods.go", "package p\n\n"+fmt.Sprintf(methods, "Level"))
	write("color_enumer.go", "// Code generated by \"enumer -type=Color -json\"; DO NOT EDIT.\n\npackage p\n\n"+fmt.Sprintf(methods, "Color"))
	write("old_easyjson.go", "// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.\n\npackage p\n\n"+fmt.Sprintf(methods, "Old"))
	write("other.go", "package main\n\n"+fmt.Sprintf(methods, "T"))

	out, err := Generate(Config{Files: []string{types}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"cfjson.AppendMarshaler(b, &m.L)",
		"cfjson.DecodeUnmarshaler(b, i, f, &m.L)",
		"cfjson.AppendMarshaler(b, &m.C)",
		"func (m *Old) AppendJSON(",
		"m.O.AppendJSON(b)",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in generated code", want)
		}
	}
}

// Code may be generated for a package in several runs, each with its types
// and its file. A run does not generate again what another one has, and does
// not take what it generated itself the last time for something which is
// there.
func TestGenerate_SeveralOutputs(t *testing.T) {
	dir := t.TempDir()
	types := filepath.Join(dir, "types.go")
	if err := os.WriteFile(types, []byte("package p\n\ntype In struct{ N int }\n\ntype S struct{ In *In }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	inOut, sOut := filepath.Join(dir, "in_cfjson.go"), filepath.Join(dir, "s_cfjson.go")
	run := func(typeName, out string) string {
		src, err := Generate(Config{Files: []string{types}, Types: []string{typeName}, Out: out})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, src, 0o600); err != nil {
			t.Fatal(err)
		}
		return string(src)
	}
	// Twice: the second time the files of the first are there.
	for range 2 {
		if in := run("In", inOut); !strings.Contains(in, "func (m *In) AppendJSON(") {
			t.Fatal("no methods for In in its own file")
		}
		s := run("S", sOut)
		if strings.Contains(s, "func (m *In)") {
			t.Fatal("methods for In generated again")
		}
		if !strings.Contains(s, "func (m *S) AppendJSON(") || !strings.Contains(s, "m.In.AppendJSON(b)") {
			t.Fatal("S does not use the methods of In")
		}
	}
}

// One of the two methods written by hand cannot be completed by generating
// the other.
func TestGenerate_HalfOfTheMethods(t *testing.T) {
	_, err := generate(t, Config{Types: []string{"S"}}, "package p\n\ntype In struct{ N int }\n\nfunc (m *In) AppendJSON(b []byte) []byte { return b }\n\ntype S struct{ In In }\n")
	if err == nil || !strings.Contains(err.Error(), "must have both or none") {
		t.Fatalf("got error %v", err)
	}
}

// An alias is another name for a type, methods and all.
func TestGenerate_Alias(t *testing.T) {
	out, err := generate(t, Config{Types: []string{"S"}}, `package p

type Only int

func (Only) MarshalJSON() ([]byte, error) { return nil, nil }

type O = Only

type Both int

func (Both) MarshalJSON() ([]byte, error) { return nil, nil }

func (*Both) UnmarshalJSON([]byte) error { return nil }

type B = Both

type Plain struct{ N int }

type P = Plain

type S struct {
	O O
	B B
	P P
}
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"cfjson.AppendMarshaler(b, &m.O)",
		"cfjson.AppendMarshaler(b, &m.B)",
		"cfjson.DecodeUnmarshaler(b, i, f, &m.B)",
		"func (m *Plain) AppendJSON(",
		"m.P.AppendJSON(b)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in generated code", want)
		}
	}
	if strings.Contains(out, "func (m *P)") {
		t.Error("methods declared for an alias")
	}
}

// A struct which has one of the two JSON methods is a struct for omitempty:
// never empty.
func TestGenerate_OneJSONMethod(t *testing.T) {
	out, err := generate(t, Config{}, "package p\n\ntype M struct{ N int }\n\nfunc (m M) MarshalJSON() ([]byte, error) { return nil, nil }\n\ntype T struct {\n\tF M `json:\"f,omitempty\"`\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `m.F != ""`) {
		t.Fatal("a struct compared with a string")
	}
}

// With NoNilElements a null element is an empty struct, and only an element:
// a field which is a pointer is nil for null either way.
func TestGenerate_NoNilElements(t *testing.T) {
	src := "package p\n\ntype E struct{ N int }\n\ntype T struct {\n\tList []*E\n\tMap  map[string]*E\n\tOne  *E\n\tNums []*int\n}\n"
	off, err := generate(t, Config{}, src)
	if err != nil {
		t.Fatal(err)
	}
	on, err := generate(t, Config{NoNilElements: true}, src)
	if err != nil {
		t.Fatal(err)
	}
	// One for the slice, one for the map, on top of the two which decode an
	// element which is there.
	if got := strings.Count(on, "= new(E)") - strings.Count(off, "= new(E)"); got != 2 {
		t.Fatalf("%d more allocations of E with the option, want 2", got)
	}
	if strings.Count(on, "m.One = nil") != 1 || strings.Count(on, "new(int)") != strings.Count(off, "new(int)") {
		t.Fatal("the option changed more than elements which are structs")
	}
}

// The method which checks raw values is generated for the structs which have
// them, however deep, and only for those.
func TestGenerate_ValidRaw(t *testing.T) {
	src := `package p

import (
	"encoding/json"
	"image"
)

type Payload []byte

type Leaf struct {
	Data Payload
	Std  json.RawMessage
}

type Plain struct{ N int }

type Holder struct {
	One   Leaf
	Ptr   *Leaf
	List  []*Leaf
	Map   map[string]Leaf
	Deep  map[string][]*Leaf
	Plain *Plain
	Point image.Point
}

type Top struct {
	*Holder
	Name string
}
`
	out, err := generate(t, Config{RawTypes: []string{"Payload"}, ValidRawMethod: "validRaw"}, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func (m *Leaf) validRaw() bool {",
		"if !cfjson.ValidRaw(m.Data) {",
		"if !cfjson.ValidRaw(m.Std) {",
		"func (m *Holder) validRaw() bool {",
		"if !m.One.validRaw() {",
		"if m.Ptr != nil {",
		"for _, e1 := range m.List {",
		"func (m *Top) validRaw() bool {",
		// The fields of an embedded struct, when it is there.
		"if m.Holder != nil {",
		"if !m.Holder.One.validRaw() {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in generated code", want)
		}
	}
	for _, unwanted := range []string{"func (m *Plain) validRaw", "validRawImagePoint", "m.Plain.validRaw", "m.Name"} {
		_, validators, _ := strings.Cut(out, "validRaw() bool {")
		if strings.Contains(out, unwanted) && unwanted != "m.Name" || strings.Contains(validators, "ValidRaw(m.Name") {
			t.Errorf("%q in generated code", unwanted)
		}
	}

	// Without the option there is no such method.
	out, err = generate(t, Config{RawTypes: []string{"Payload"}}, src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "ValidRaw") {
		t.Error("raw values are checked without the option")
	}

	// A struct with methods written by hand has to have this one too: what
	// is in it is not known.
	_, err = generate(t, Config{Types: []string{"S"}, ValidRawMethod: "validRaw"}, "package p\n\ntype In struct{ N int }\n\nfunc (m *In) AppendJSON(b []byte) []byte { return b }\n\nfunc (m *In) DecodeJSON(b []byte, i int, f uint32) int { return i }\n\ntype S struct{ In In }\n")
	if err == nil || !strings.Contains(err.Error(), "no method validRaw") {
		t.Fatalf("got error %v", err)
	}
}

// Methods declared on an alias are methods of the type it names: that is
// where Go puts them, and where the generator must look for them.
func TestGenerate_MethodsOnAlias(t *testing.T) {
	out, err := generate(t, Config{Types: []string{"T"}}, `package p

type B struct{ N int }

type A = B

func (v A) MarshalJSON() ([]byte, error) { return nil, nil }

func (v *(A)) UnmarshalJSON([]byte) error { return nil }

type T struct {
	B B
	A A
}
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"cfjson.AppendMarshaler(b, &m.B)", "cfjson.AppendMarshaler(b, &m.A)", "cfjson.DecodeUnmarshaler(b, i, f, &m.B)"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in generated code", want)
		}
	}
	if strings.Contains(out, "func (m *B)") {
		t.Error("methods generated for a type which has its own")
	}

	// An alias in -types is the struct it names.
	out, err = generate(t, Config{Types: []string{"P"}}, "package p\n\ntype S struct{ N int }\n\ntype P = S\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "func (m *S) AppendJSON(") || strings.Contains(out, "func (m *P)") {
		t.Fatalf("wrong methods for an alias in Types:\n%s", out)
	}
}
