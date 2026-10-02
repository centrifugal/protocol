package cfjson

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// implementationDefined says what Valid does with the inputs of JSONTestSuite
// for which RFC 8259 leaves the choice to the parser (the i_ files). The rule
// behind every entry: input which is not valid UTF-8 is rejected, everything
// else which follows the grammar is accepted, however large a number is or
// however deep a value is nested. Whether a number fits the field it is
// decoded into is for the decoder of that field to say, not for Valid.
var implementationDefined = map[string]bool{
	"i_number_double_huge_neg_exp.json":                   true,
	"i_number_huge_exp.json":                              true,
	"i_number_neg_int_huge_exp.json":                      true,
	"i_number_pos_double_huge_exp.json":                   true,
	"i_number_real_neg_overflow.json":                     true,
	"i_number_real_pos_overflow.json":                     true,
	"i_number_real_underflow.json":                        true,
	"i_number_too_big_neg_int.json":                       true,
	"i_number_too_big_pos_int.json":                       true,
	"i_number_very_big_negative_int.json":                 true,
	"i_object_key_lone_2nd_surrogate.json":                true,
	"i_string_1st_surrogate_but_2nd_missing.json":         true,
	"i_string_1st_valid_surrogate_2nd_invalid.json":       true,
	"i_string_incomplete_surrogate_and_escape_valid.json": true,
	"i_string_incomplete_surrogate_pair.json":             true,
	"i_string_incomplete_surrogates_escape_valid.json":    true,
	"i_string_invalid_lonely_surrogate.json":              true,
	"i_string_invalid_surrogate.json":                     true,
	"i_string_inverted_surrogates_U+1D11E.json":           true,
	"i_string_lone_second_surrogate.json":                 true,
	"i_structure_500_nested_arrays.json":                  true,
}

// JSONTestSuite (https://github.com/nst/JSONTestSuite, "Parsing JSON is a
// Minefield") is the usual yardstick for JSON parsers: documents which must be
// accepted (y_), which must be rejected (n_), and which a parser may treat
// either way (i_).
func TestJSONTestSuite(t *testing.T) {
	files, err := filepath.Glob("testdata/JSONTestSuite/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 300 {
		t.Fatalf("only %d files found", len(files))
	}
	sort.Strings(files)
	for _, file := range files {
		name := filepath.Base(file)
		data, err := os.ReadFile(file) //nolint:gosec // G304: the files of testdata.
		if err != nil {
			t.Fatal(err)
		}
		got := Valid(data)
		var want bool
		switch {
		case strings.HasPrefix(name, "y_"):
			want = true
		case strings.HasPrefix(name, "n_"):
			want = false
		default:
			want = implementationDefined[name]
			// The rule stated above must explain every rejection. There is
			// one more: a byte order mark in front of the document, which
			// RFC 8259 tells senders not to add, is not skipped.
			if !want && utf8.Valid(data) && name != "i_structure_UTF-8_BOM_empty_object.json" {
				t.Errorf("%s: rejected although it is valid UTF-8", name)
			}
		}
		if got != want {
			t.Errorf("%s: Valid = %v, want %v", name, got, want)
		}
		// Skipping a value, with and without the prescan, must agree with
		// Valid.
		i := SkipSpace(data, 0)
		for _, f := range []Flags{0, Prescan(data, 0)} {
			end := Skip(data, i, f)
			if ok := end >= 0 && SkipSpace(data, end) == len(data); ok != want {
				t.Errorf("%s: Skip with flags %d says %v, want %v", name, f, ok, want)
			}
		}
	}
}

// The documents of the suite which hold one string or one number in an array
// are also decoded, and must give what encoding/json gives: Valid accepting a
// document says nothing about the value a decoder reads from it.
func TestJSONTestSuite_Values(t *testing.T) {
	files, err := filepath.Glob("testdata/JSONTestSuite/*.json")
	if err != nil {
		t.Fatal(err)
	}
	var stringsSeen, numbersSeen int
	for _, file := range files {
		name := filepath.Base(file)
		data, err := os.ReadFile(file) //nolint:gosec // G304: the files of testdata.
		if err != nil {
			t.Fatal(err)
		}
		if !Valid(data) {
			continue
		}
		var doc []any
		if stdjson.Unmarshal(data, &doc) != nil || len(doc) != 1 {
			continue
		}
		// The value starts after the bracket.
		i := SkipSpace(data, SkipSpace(data, 0)+1)
		switch want := doc[0].(type) {
		case string:
			stringsSeen++
			for _, f := range []Flags{0, Prescan(data, 0)} {
				var got string
				if n := String(data, i, f, &got); n < 0 {
					t.Errorf("%s: String with flags %d: %v", name, f, Error(data, n))
				} else if got != want {
					t.Errorf("%s: String with flags %d = %q, encoding/json says %q", name, f, got, want)
				}
			}
		case float64:
			numbersSeen++
			var got float64
			if n := Float(data, i, &got); n < 0 {
				// A number out of the range of float64 is an error for
				// encoding/json too, which is why doc would not be there.
				t.Errorf("%s: Float: %v", name, Error(data, n))
			} else if got != want {
				t.Errorf("%s: Float = %v, encoding/json says %v", name, got, want)
			}
		}
	}
	if stringsSeen < 40 || numbersSeen < 15 {
		t.Fatalf("only %d strings and %d numbers decoded", stringsSeen, numbersSeen)
	}
}
