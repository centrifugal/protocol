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
// not code which silently skips it: a field left out of the encoding is data
// lost on the wire.
func TestGenerate_Unsupported(t *testing.T) {
	tests := []struct {
		name, fields, want string
	}{
		{"oneof", "A isT_A `protobuf_oneof:\"a\"`\nB string `protobuf:\"bytes,1,opt,name=b,proto3,oneof\"`", `option "oneof" is not supported`},
		{"enum", "E int32 `protobuf:\"varint,1,opt,name=e,proto3,enum=pkg.E\"`", `option "enum=pkg.E" is not supported`},
		{"packed", "N []int32 `protobuf:\"varint,1,rep,packed,name=n,proto3\"`", `option "packed" is not supported`},
		{"repeated scalar", "N []int32 `protobuf:\"varint,1,rep,name=n,proto3\"`", "repeated fields of type int32 are not supported"},
		{"required", "N int32 `protobuf:\"varint,1,req,name=n,proto3\"`", `label "req" is not supported`},
		{"proto2", "N *int32 `protobuf:\"varint,1,opt,name=n\"`", "only proto3 fields are supported"},
		{"map with enum values", "M map[string]int32 `protobuf:\"bytes,1,rep,name=m,proto3\" protobuf_key:\"bytes,1,opt,name=key\" protobuf_val:\"varint,2,opt,name=value,enum=pkg.E\"`", "enum map values are not supported"},
		{"sint32", "N int32 `protobuf:\"zigzag32,1,opt,name=n,proto3\"`", `unsupported type int32 with encoding "zigzag32"`},
		{"fixed integer", "N uint64 `protobuf:\"fixed64,1,opt,name=n,proto3\"`", `unsupported type uint64 with encoding "fixed64"`},
		{"type and encoding mismatch", "S string `protobuf:\"varint,1,opt,name=s,proto3\"`", `unsupported type string with encoding "varint"`},
		{"message from another package", "M *other.M `protobuf:\"bytes,1,opt,name=m,proto3\"`", "unsupported type *other.M"},
		{"message by value", "M T `protobuf:\"bytes,1,opt,name=m,proto3\"`", "unsupported type T"},
		{"map with integer keys", "M map[int32]string `protobuf:\"bytes,1,rep,name=m,proto3\" protobuf_key:\"varint,1,opt,name=key\" protobuf_val:\"bytes,2,opt,name=value\"`", "unsupported map tags"},
		{"field number zero", "S string `protobuf:\"bytes,0,opt,name=s,proto3\"`", "bad field number"},
		{"field number too large", "S string `protobuf:\"bytes,536870912,opt,name=s,proto3\"`", "bad field number"},
		{"duplicate field number", "A string `protobuf:\"bytes,1,opt,name=a,proto3\"`\nB string `protobuf:\"bytes,1,opt,name=b,proto3\"`", "field number 1 is also used by A"},
		{"bad tag", "S string `protobuf:\"bytes\"`", "bad protobuf tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generate(t, Config{}, "package p\n\ntype T struct {\n"+tt.fields+"\n}\n")
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
	Data Payload ` + "`protobuf:\"bytes,1,opt,name=data,proto3\"`" + `
	B    *B      ` + "`protobuf:\"bytes,2,opt,name=b,proto3\"`" + `
	NotAField int
}

type B struct {
	N int64 ` + "`protobuf:\"varint,1,opt,name=n,proto3\"`" + `
}

// Not a message: no field has a protobuf tag.
type Plain struct{ X int }
`
	out, err := generate(t, Config{Suffix: "PB", Runtime: "example.com/cfprotobuf"}, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`cfprotobuf "example.com/cfprotobuf"`,
		"func (m *A) MarshalPB() ([]byte, error) {",
		"func (m *A) MarshalToPB(b []byte) (int, error) {",
		"func (m *A) MarshalToSizedBufferPB(b []byte) (int, error) {",
		"func (m *A) SizePB() (n int) {",
		"func (m *A) UnmarshalPB(b []byte) error {",
		"func (m *B) unmarshalPB(b []byte, depth int) error {",
		"m.Data = Payload{}",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the output", want)
		}
	}
	for _, unwanted := range []string{"Plain", "NotAField", "VT(", "unknownFields", "encoding/binary", `"math"`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("%q in the output", unwanted)
		}
	}

	out, err = generate(t, Config{Types: []string{"B"}}, src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "func (m *A)") || !strings.Contains(out, "func (m *B) MarshalVT()") {
		t.Error("the Types option is not respected")
	}
	if _, err := generate(t, Config{Types: []string{"Missing"}}, src); err == nil {
		t.Error("no error for a type which is not declared")
	}
	if _, err := Generate(Config{}); err == nil {
		t.Error("no error without input files")
	}
}

// With DropUnknown a decoder keeps nothing of the fields it does not know.
// Marshaling still writes what a message has of them: it may have been
// decoded by other means.
func TestGenerate_DropUnknown(t *testing.T) {
	src := "package p\n\ntype T struct {\n\tS string `protobuf:\"bytes,1,opt,name=s,proto3\"`\n\tunknownFields []byte\n}\n"
	kept, err := generate(t, Config{}, src)
	if err != nil {
		t.Fatal(err)
	}
	dropped, err := generate(t, Config{DropUnknown: true}, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kept, "cfprotobuf.AppendUnknown") || strings.Contains(dropped, "cfprotobuf.AppendUnknown") {
		t.Fatal("DropUnknown does not decide whether unknown fields are kept")
	}
	for _, out := range []string{kept, dropped} {
		if !strings.Contains(out, "copy(b[i:], m.unknownFields)") || !strings.Contains(out, "n += len(m.unknownFields)") {
			t.Fatal("unknown fields a message has are not marshaled")
		}
	}
}

// Only recursive types pay for the depth check.
func TestGenerate_Recursive(t *testing.T) {
	out, err := generate(t, Config{}, `package p

type Tree struct {
	Children []*Tree `+"`protobuf:\"bytes,1,rep,name=children,proto3\"`"+`
	Leaf     *Leaf   `+"`protobuf:\"bytes,2,opt,name=leaf,proto3\"`"+`
}

type Leaf struct {
	N int64 `+"`protobuf:\"varint,1,opt,name=n,proto3\"`"+`
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out, "cfprotobuf.ErrTooDeep"); got != 1 {
		t.Fatalf("%d depth checks, want 1", got)
	}
	_, leaf, found := strings.Cut(out, "func (m *Leaf) unmarshalVT")
	if !found || strings.Contains(leaf, "ErrTooDeep") {
		t.Fatal("depth check in a type which is not recursive")
	}
}
