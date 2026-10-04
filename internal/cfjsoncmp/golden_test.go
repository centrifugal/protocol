package cfjsoncmp

import (
	"bytes"
	"flag"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/centrifugal/protocol/cfjson/gen"
)

var updateGolden = flag.Bool("update", false, "rewrite types_cfjson.go and ../../testdata/json_golden.txt")

const goldenFile = "../../testdata/json_golden.txt"

// goldenLines returns messages encoded by easyjson, one per line in the form
// "TypeName<TAB>JSON". The protocol module checks its encoders against them,
// see TestJSONGolden there: that test has to work without easyjson, which
// protocol does not depend on anymore.
//
// The messages are restricted to what survives a round trip through
// encoding/json, which is how that test gets a message to encode: strings
// are valid UTF-8, and raw values have no whitespace around them.
func goldenLines() []byte {
	r := rand.New(rand.NewSource(7))
	f := filler{r: r, maxMapLen: 1}
	for _, s := range sampleStrings {
		if utf8.ValidString(s) {
			f.strings = append(f.strings, s)
		}
	}
	for _, s := range sampleRaws {
		if strings.TrimSpace(s) == s && utf8.ValidString(s) {
			f.raws = append(f.raws, s)
		}
	}
	var out bytes.Buffer
	seen := map[string]bool{}
	add := func(m message) {
		line := reflect.TypeOf(m).Elem().Name() + "\t" + string(oldEncode(m)) + "\n"
		if !seen[line] {
			seen[line] = true
			out.WriteString(line)
		}
	}
	for _, bm := range benchMessages {
		if reply, ok := bm.msg.(*Reply); ok && reply.Presence != nil {
			continue // Several keys in a map: the order is random.
		}
		add(bm.msg)
	}
	for _, newMsg := range allTypes {
		add(newMsg())
		for n := 0; n < 40; n++ {
			m := newMsg()
			f.fill(reflect.ValueOf(m).Elem(), 0)
			add(m)
		}
	}
	return out.Bytes()
}

// The golden file protocol tests its encoders with must be what easyjson
// produces.
func TestGoldenFile(t *testing.T) {
	want := goldenLines()
	if *updateGolden {
		if err := os.WriteFile(goldenFile, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is out of date, run make json-compat-update in the repository root", goldenFile)
	}
}

// types_cfjson.go must be what the generator produces today, otherwise the
// comparisons in this module say nothing about the current generator.
func TestGeneratedCodeIsUpToDate(t *testing.T) {
	want, err := gen.Generate(gen.Config{Files: []string{"types.go"}, RawTypes: []string{"Raw"}})
	if err != nil {
		t.Fatal(err)
	}
	if *updateGolden {
		if err := os.WriteFile("types_cfjson.go", want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile("types_cfjson.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("types_cfjson.go is out of date, run make json-compat-update in the repository root")
	}
}
