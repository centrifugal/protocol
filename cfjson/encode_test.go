package cfjson

import (
	"bytes"
	stdjson "encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

// AppendString escapes every string field of every generated encoder, and the
// result is decoded by JSON parsers written in other languages.
//
// The expectations are the exact bytes rather than a comparison against
// encoding/json: an escape often has more than one valid spelling and the
// standard library has already changed which one it picks (Go 1.26 emits \f
// where earlier versions emitted \u000c), so only pinning the output here
// catches a real drift. Each case is also decoded back with encoding/json, so
// the pinned bytes cannot drift into something merely self-consistent.
//
// Note the asymmetry in the table below: an `in` written with a Go escape is
// the character itself, while a `want` in a raw string literal is the escape
// sequence as it appears in the encoded output.
func TestAppendString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", `""`},
		{"plain", "hello world", `"hello world"`},
		{"plain longer than a word", "news:2024-01-01_sports.football", `"news:2024-01-01_sports.football"`},
		{"quote and backslash", `he said "hi" \ there`, `"he said \"hi\" \\ there"`},
		{"short escapes", "\n\r\t", `"\n\r\t"`},
		{"control chars", "\b\f\x00\x01\x0b\x1f", `"\u0008\u000c\u0000\u0001\u000b\u001f"`},
		// <, > and & are escaped so an encoded frame stays safe to embed into
		// an HTML page.
		{"html unsafe", "<script>alert(1)&amp;</script>", `"\u003cscript\u003ealert(1)\u0026amp;\u003c/script\u003e"`},
		{"del is not escaped", "\x7f", "\"\x7f\""},
		{"multibyte passes through", "h\u00e9llo \u2713 \U0001F600", "\"h\u00e9llo \u2713 \U0001F600\""},
		// U+2028 and U+2029 are legal inside a JSON string but break JSONP, so
		// they are escaped even though JSON does not require it.
		{"line and paragraph separators", "a\u2028b\u2029c", `"a\u2028b\u2029c"`},
		{"encoded replacement char passes through", "\ufffd", "\"\ufffd\""},
		{"invalid utf8 leading byte", "a" + string([]byte{0x80}) + "b", `"a\ufffdb"`},
		{"invalid utf8 truncated rune", "tail\xc3", `"tail\ufffd"`},
		{"invalid utf8 only", string([]byte{0xff, 0xfe}), `"\ufffd\ufffd"`},
		{"escape at start and end", `"middle"`, `"\"middle\""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(AppendString(nil, tt.in))
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
			if roomy := string(AppendString(make([]byte, 0, 256), tt.in)); roomy != tt.want {
				t.Fatalf("with room in the buffer: got %s, want %s", roomy, tt.want)
			}
			// Whichever spelling is used, the result must be a JSON string that
			// decodes back to the input - with invalid UTF-8 replaced, which is
			// the one transformation allowed.
			var back string
			if err := stdjson.Unmarshal([]byte(got), &back); err != nil {
				t.Fatal(err)
			}
			if utf8.ValidString(tt.in) {
				if back != tt.in {
					t.Fatalf("decoded back to %q", back)
				}
			} else if !utf8.ValidString(back) {
				t.Fatalf("decoded back to invalid UTF-8 %q", back)
			}
		})
	}
}

// The fast path looks at 8 or 32 bytes at a time, so a byte which needs escaping
// must be noticed at every position of a word, and in the tail after the last
// full word.
func TestAppendString_EveryByteEveryPosition(t *testing.T) {
	for c := 0; c < 256; c++ {
		// Long strings are looked at four words at a time: the sizes go
		// through three such steps and every tail after them.
		for size := 1; size <= 110; size++ {
			for pos := 0; pos < size; pos++ {
				in := []byte(strings.Repeat("a", size))
				in[pos] = byte(c)
				want := appendStringSlow([]byte(`x"`), string(in))
				// Short strings are written in another way when the buffer
				// has room for them, so try with and without room.
				for _, room := range []int{0, 128} {
					buf := make([]byte, 1, 1+room)
					buf[0] = 'x'
					if got := AppendString(buf, string(in)); !bytes.Equal(got, want) {
						t.Fatalf("byte %#x at %d of %d, room %d: got %s, want %s", c, pos, size, room, got, want)
					}
				}
			}
		}
	}
}

// Raw is how an already encoded payload (Publication data, an RPC result)
// reaches the frame. An absent payload must become a JSON null rather than
// nothing at all, which would leave a malformed message on the wire.
func TestAppendRaw(t *testing.T) {
	tests := []struct {
		name string
		src  []byte
		want string
	}{
		{"nil becomes null", nil, "null"},
		{"empty becomes null", []byte{}, "null"},
		{"payload passed through as is", []byte(`{"a":  1}`), `{"a":  1}`},
		{"newlines are dropped", []byte("{\n\"a\":\n1\n}\n"), `{"a":1}`},
		{"only newlines becomes null", []byte("\n\n"), `null`},
		{"appended after other data", []byte("1"), `1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(AppendRaw(nil, tt.src)); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendScalars(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		want string
	}{
		{"uint zero", AppendUint(nil, 0), "0"},
		{"uint one digit", AppendUint(nil, 9), "9"},
		{"uint two digits", AppendUint(nil, 10), "10"},
		{"uint32 max", AppendUint(nil, math.MaxUint32), "4294967295"},
		{"uint64 max", AppendUint(nil, math.MaxUint64), "18446744073709551615"},
		{"int zero", AppendInt(nil, 0), "0"},
		{"int one digit", AppendInt(nil, 7), "7"},
		{"int negative", AppendInt(nil, -1), "-1"},
		{"int32 min", AppendInt(nil, math.MinInt32), "-2147483648"},
		{"int64 min", AppendInt(nil, math.MinInt64), "-9223372036854775808"},
		{"int64 max", AppendInt(nil, math.MaxInt64), "9223372036854775807"},
		{"bool true", AppendBool(nil, true), "true"},
		{"bool false", AppendBool(nil, false), "false"},
		{"float64", AppendFloat64(nil, 1.5), "1.5"},
		{"float64 large", AppendFloat64(nil, 1000000), "1e+06"},
		{"float32", AppendFloat32(nil, 0.1), "0.1"},
		{"float64 NaN", AppendFloat64(nil, math.NaN()), "null"},
		{"float64 +Inf", AppendFloat64(nil, math.Inf(1)), "null"},
		{"float64 -Inf", AppendFloat64(nil, math.Inf(-1)), "null"},
		{"float64 max", AppendFloat64(nil, math.MaxFloat64), "1.7976931348623157e+308"},
		{"float32 NaN", AppendFloat32(nil, float32(math.NaN())), "null"},
		{"float32 Inf", AppendFloat32(nil, float32(math.Inf(-1))), "null"},
		{"float32 max", AppendFloat32(nil, math.MaxFloat32), "3.4028235e+38"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, tt.got, tt.want)
		}
	}
}
