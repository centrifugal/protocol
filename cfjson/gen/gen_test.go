package gen

import (
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
// not only in the files given to the generator. Generated files do not count:
// what is there was not written for the type by somebody.
func TestGenerate_MethodsInOtherFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	types := write("types.go", "package p\n\ntype Level int\n\ntype Old struct{ N int }\n\ntype T struct {\n\tL Level\n\tO Old\n}\n")
	write("methods.go", "package p\n\nfunc (l Level) MarshalJSON() ([]byte, error) { return nil, nil }\n\nfunc (l *Level) UnmarshalJSON([]byte) error { return nil }\n")
	write("old_gen.go", "// Code generated by another generator. DO NOT EDIT.\n\npackage p\n\nfunc (o Old) MarshalJSON() ([]byte, error) { return nil, nil }\n\nfunc (o *Old) UnmarshalJSON([]byte) error { return nil }\n")
	write("other.go", "package main\n\nfunc (t T) MarshalJSON() ([]byte, error) { return nil, nil }\n")

	out, err := Generate(Config{Files: []string{types}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"cfjson.AppendMarshaler(b, &m.L)",
		"cfjson.DecodeUnmarshaler(b, i, f, &m.L)",
		"func (m *Old) AppendJSON(",
		"m.O.AppendJSON(b)",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in generated code", want)
		}
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
