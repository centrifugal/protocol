package cfjson

import (
	stdjson "encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// stdlibTestLiterals returns every string literal of the tests of
// encoding/json in the Go installation the test runs with, the packages
// inside of it (jsontext, v2) included. The people who wrote those tests put
// the inputs a JSON parser gets wrong there, and the set grows with every
// release of Go: reading it from the installation keeps up with that without
// a copy to maintain.
func stdlibTestLiterals(t *testing.T) []string {
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("no go command: %v", err)
	}
	root := filepath.Join(strings.TrimSpace(string(goroot)), "src", "encoding", "json")
	if _, err = os.Stat(root); err != nil {
		t.Skipf("no source of encoding/json in this Go installation: %v", err)
	}
	seen := map[string]bool{}
	var literals []string
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if _, isImport := n.(*ast.ImportSpec); isImport {
				return false
			}
			// The tag of a struct field is a literal too, and not an input.
			if field, isField := n.(*ast.Field); isField && field.Tag != nil {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil && s != "" && !seen[s] {
				seen[s] = true
				literals = append(literals, s)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return literals
}

// Every input the tests of encoding/json have is run through what this
// package has for it, and encoding/json is asked what is right.
func TestStdlibTestInputs(t *testing.T) {
	literals := stdlibTestLiterals(t)
	var valid, stringsSeen, numbers int
	for _, s := range literals {
		data := []byte(s)
		// encoding/json does not require the input to be valid UTF-8.
		want := stdjson.Valid(data) && utf8.Valid(data)
		if got := Valid(data); got != want {
			t.Errorf("Valid(%q) = %v, want %v", s, got, want)
			continue
		}
		start := SkipSpace(data, 0)
		for _, f := range []Flags{0, Prescan(data, 0)} {
			end := Skip(data, start, f)
			if ok := end >= 0 && SkipSpace(data, end) == len(data); ok != want {
				t.Errorf("Skip(%q) with flags %d says %v, want %v", s, f, ok, want)
			}
		}
		if !want {
			// A literal which is not JSON is still a Go string to encode.
			encoded := AppendString(nil, s)
			var back string
			if err := stdjson.Unmarshal(encoded, &back); err != nil {
				t.Errorf("AppendString(%q) = %s: %v", s, encoded, err)
			} else if utf8.ValidString(s) && back != s {
				t.Errorf("AppendString(%q) = %s, which is %q", s, encoded, back)
			}
			continue
		}
		valid++
		switch data[start] {
		case '"':
			stringsSeen++
			var wantStr string
			if err := stdjson.Unmarshal(data, &wantStr); err != nil {
				t.Fatalf("%q: %v", s, err)
			}
			for _, f := range []Flags{0, ZeroCopy, Prescan(data, 0), Prescan(data, ZeroCopy)} {
				var got string
				if n := String(data, start, f, &got); n < 0 {
					t.Errorf("String(%q) with flags %d: %v", s, f, Error(data, n))
				} else if got != wantStr {
					t.Errorf("String(%q) with flags %d = %q, want %q", s, f, got, wantStr)
				}
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			numbers++
			var wantFloat, gotFloat float64
			wantErr := stdjson.Unmarshal(data, &wantFloat)
			n := Float(data, start, &gotFloat)
			if (n < 0) != (wantErr != nil) || (n >= 0 && gotFloat != wantFloat) {
				t.Errorf("Float(%q) = %v (%d), encoding/json says %v (%v)", s, gotFloat, n, wantFloat, wantErr)
			}
			var wantInt, gotInt int64
			wantErr = stdjson.Unmarshal(data, &wantInt)
			n = Int(data, start, &gotInt)
			if (n < 0) != (wantErr != nil) || (n >= 0 && gotInt != wantInt) {
				t.Errorf("Int(%q) = %v (%d), encoding/json says %v (%v)", s, gotInt, n, wantInt, wantErr)
			}
			var wantUint, gotUint uint64
			wantErr = stdjson.Unmarshal(data, &wantUint)
			n = Uint(data, start, &gotUint)
			if (n < 0) != (wantErr != nil) || (n >= 0 && gotUint != wantUint) {
				t.Errorf("Uint(%q) = %v (%d), encoding/json says %v (%v)", s, gotUint, n, wantUint, wantErr)
			}
		}
	}
	t.Logf("%d literals: %d valid JSON, %d strings, %d numbers", len(literals), valid, stringsSeen, numbers)
	if len(literals) < 1000 || valid < 300 {
		t.Fatalf("only %d literals, %d of them valid JSON: the tests of encoding/json were not found", len(literals), valid)
	}
}
