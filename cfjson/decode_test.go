package cfjson

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

var validCases = []string{
	`null`, `true`, `false`, `0`, `-0`, `1`, `-1`, `1.5`, `1e5`, `1E+5`, `1.5e-5`, `""`, `"a"`, `"\""`, `"\\"`, `"\/\b\f\n\r\t"`,
	`"\u0041"`, `"\ud83d\ude00"`, `"\ud83d"`, "\"\x7f\"", "\"h\xc3\xa9llo \xf0\x9f\x98\x80\"", "{\"k\xc3\xa9y\":\"\\n\xe2\x9c\x93\"}", `[]`, `[1]`, `[1,2]`, `[[],[[]]]`, `{}`, `{"a":1}`,
	`{"a":1,"b":[true,null,{"c":"d"}]}`, " \t\r\n{ \"a\" : [ 1 , 2 ] } \n", `{"a":{"b":{"c":{}}}}`, `{"":""}`, `[{},{}]`,
	`"12345678"`, `"1234567\""`, `"12345678\\"`, `"123456789012345678901234567890"`,
}

var invalidCases = []string{
	``, ` `, `nul`, `nulll`, `tru`, `fals`, `True`, `-`, `+1`, `01`, `1.`, `.5`, `1e`, `1e+`, `1.e5`, `0x1`, `--1`, `1 2`,
	`"`, `"a`, `"\"`, `"\x"`, `"\u12"`, `"\u12g4"`, `"\u`, "\"a\nb\"", "\"a\x00b\"", "\"a\x1fb\"", `'a'`,
	// Invalid UTF-8, in a value and in a key, alone and next to an escape.
	"\"\xff\"", "\"abc\xc3\"", "\"\xc3(\"", "\"\xed\xa0\x80\"", "\"a\\n\xff\"", "{\"\xff\":1}", "[\"ok\",\"\x80\"]",
	`[`, `]`, `[1`, `[1,`, `[1,]`, `[,1]`, `[1 2]`, `[1}`, `{`, `}`, `{"a"`, `{"a":`, `{"a":1`, `{"a":1,`, `{"a":1,}`, `{,}`,
	`{"a" 1}`, `{a:1}`, `{"a":1]`, `{"a":1 "b":2}`, `{1:1}`, `{null:1}`, `{"a"}`, `[{]`, `{"a":[}`, `{}{}`, `[]]`, `{}}`, `{"a":tru}`,
}

func TestValid(t *testing.T) {
	for _, s := range validCases {
		if !Valid([]byte(s)) {
			t.Errorf("Valid(%q) = false", s)
		}
	}
	for _, s := range invalidCases {
		if Valid([]byte(s)) {
			t.Errorf("Valid(%q) = true", s)
		}
	}
}

// Skip keeps a bit per open container in a word, and spills into a slice
// every 64 levels.
func TestSkip_DeepNesting(t *testing.T) {
	for _, depth := range []int{1, 2, 63, 64, 65, 66, 127, 128, 129, 1000, 1 << 20} {
		var sb strings.Builder
		for i := 0; i < depth; i++ {
			if i%3 == 0 {
				sb.WriteString(`{"a":`)
			} else {
				sb.WriteString(`[1,`)
			}
		}
		sb.WriteString(`null`)
		for i := depth - 1; i >= 0; i-- {
			if i%3 == 0 {
				sb.WriteString(`}`)
			} else {
				sb.WriteString(`,2]`)
			}
		}
		good := sb.String()
		if !Valid([]byte(good)) {
			t.Fatalf("depth %d: valid document rejected", depth)
		}
		// Swap the closing bracket of one level for the wrong one.
		for _, level := range []int{0, depth / 2, depth - 1} {
			bad := []byte(good)
			for i, n := len(bad)-1, 0; i >= 0; i-- {
				if bad[i] == '}' || bad[i] == ']' {
					if n == level {
						bad[i] ^= '}' ^ ']'
						break
					}
					n++
				}
			}
			if Valid(bad) {
				t.Fatalf("depth %d: mismatched bracket at level %d accepted", depth, level)
			}
		}
		if Valid([]byte(good[:len(good)-1])) {
			t.Fatalf("depth %d: truncated document accepted", depth)
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct{ in, want string }{
		{`""`, ""}, {`"a"`, "a"}, {`"1234567"`, "1234567"}, {`"12345678"`, "12345678"}, {`"123456789"`, "123456789"},
		{`"a\"b"`, `a"b`}, {`"\\"`, `\`}, {`"\/"`, "/"}, {`"\b\f\n\r\t"`, "\b\f\n\r\t"}, {`"\u0041\u00e9"`, "A\xc3\xa9"},
		{`"\ud83d\ude00"`, "\xf0\x9f\x98\x80"}, {`"\ud83d"`, "\xef\xbf\xbd"}, {`"\ude00"`, "\xef\xbf\xbd"},
		{`"\ud83dx"`, "\xef\xbf\xbdx"}, {`"\ud83d\u0041"`, "\xef\xbf\xbdA"}, {`"\ud83d\ud83d\ude00"`, "\xef\xbf\xbd\xf0\x9f\x98\x80"},
		{"\"h\xc3\xa9llo\"", "h\xc3\xa9llo"}, {"\"a\\n\xc3\xa9b\"", "a\n\xc3\xa9b"}, {"\"\x7f\"", "\x7f"},
	}
	for _, tt := range tests {
		// Trailing data makes sure decoding stops at the closing quote.
		in := []byte(tt.in + `,"next"`)
		for _, f := range []Flags{0, Prescan(in, 0)} {
			got := "unset"
			n := String(in, 0, f, &got)
			if n != len(tt.in) || got != tt.want {
				t.Errorf("String(%s, %d) = %q, %d; want %q, %d", tt.in, f, got, n, tt.want, len(tt.in))
			}
		}
	}
	for _, in := range []string{``, `"`, `"a`, `"\`, `"\x"`, `"\u12"`, `"\u00g0"`, "\"\n\"", `1`, `nul`, `{}`,
		"\"bad\xff\"", "\"\xc3\"", "\"a\\n\xffb\"", "\"\xed\xa0\x80\""} {
		var got string
		if n := String([]byte(in), 0, 0, &got); n >= 0 {
			t.Errorf("String(%q) = %q, %d; want an error", in, got, n)
		}
	}
	got := "kept"
	if n := String([]byte(`null`), 0, 0, &got); n != 4 || got != "kept" {
		t.Errorf("String(null) = %q, %d", got, n)
	}
}

// A decoded string never points into the input, plain ASCII included.
func TestString_Copies(t *testing.T) {
	for _, in := range []string{`"plain"`, "\"h\xc3\xa9llo\"", `"es\\caped"`, `"a plain string long enough to be prescanned"`} {
		data := []byte(in)
		for _, f := range []Flags{0, Prescan(data, 0)} {
			var got, want string
			String(data, 0, f, &got)
			String([]byte(in), 0, 0, &want)
			data[1] = 'X'
			if got != want {
				t.Errorf("%s: decoded string changed with the input", in)
			}
			data = []byte(in)
		}
	}
}

func TestNumbers(t *testing.T) {
	var (
		i8  int8
		i32 int32
		i64 int64
		u8  uint8
		u32 uint32
		u64 uint64
		f64 float64
		f32 float32
		b   bool
	)
	ok := func(name string, n, want int, good bool) {
		t.Helper()
		if n != want || !good {
			t.Errorf("%s: returned %d, want %d, value ok: %v", name, n, want, good)
		}
	}
	ok("int8 min", Int([]byte(`-128,`), 0, &i8), 4, i8 == -128)
	ok("int8 max", Int([]byte(`127`), 0, &i8), 3, i8 == 127)
	ok("int32 min", Int([]byte(`-2147483648`), 0, &i32), 11, i32 == -2147483648)
	ok("int64 min", Int([]byte(`-9223372036854775808`), 0, &i64), 20, i64 == -9223372036854775808)
	ok("int64 max", Int([]byte(`9223372036854775807`), 0, &i64), 19, i64 == 9223372036854775807)
	ok("int64 -0", Int([]byte(`-0`), 0, &i64), 2, i64 == 0)
	ok("uint8 max", Uint([]byte(`255}`), 0, &u8), 3, u8 == 255)
	ok("uint32 max", Uint([]byte(`4294967295`), 0, &u32), 10, u32 == 4294967295)
	ok("uint64 max", Uint([]byte(`18446744073709551615`), 0, &u64), 20, u64 == 18446744073709551615)
	ok("uint64 zero", Uint([]byte(`0`), 0, &u64), 1, u64 == 0)
	ok("uint64 null", Uint([]byte(`null`), 0, &u64), 4, u64 == 0)
	ok("float64", Float([]byte(`-1.5e2,`), 0, &f64), 6, f64 == -150)
	ok("float32", Float([]byte(`0.25`), 0, &f32), 4, f32 == 0.25)
	ok("float64 int", Float([]byte(`7`), 0, &f64), 1, f64 == 7)
	ok("bool true", Bool([]byte(`true`), 0, &b), 4, b)
	ok("bool false", Bool([]byte(`false`), 0, &b), 5, !b)
	b = true
	ok("bool null", Bool([]byte(`null`), 0, &b), 4, b)

	bad := func(name string, n int) {
		t.Helper()
		if n >= 0 {
			t.Errorf("%s: returned %d, want an error", name, n)
		}
	}
	bad("int8 overflow", Int([]byte(`128`), 0, &i8))
	bad("int8 underflow", Int([]byte(`-129`), 0, &i8))
	bad("int32 overflow", Int([]byte(`2147483648`), 0, &i32))
	bad("int64 overflow", Int([]byte(`9223372036854775808`), 0, &i64))
	bad("int64 underflow", Int([]byte(`-9223372036854775809`), 0, &i64))
	bad("int64 huge", Int([]byte(`99999999999999999999999`), 0, &i64))
	bad("int64 float", Int([]byte(`1.0`), 0, &i64))
	bad("int64 exponent", Int([]byte(`1e2`), 0, &i64))
	bad("int64 leading zero", Int([]byte(`01`), 0, &i64))
	bad("int64 negative leading zero", Int([]byte(`-01`), 0, &i64))
	bad("int64 minus", Int([]byte(`-`), 0, &i64))
	bad("int64 plus", Int([]byte(`+1`), 0, &i64))
	bad("int64 string", Int([]byte(`"1"`), 0, &i64))
	bad("int64 empty", Int([]byte(``), 0, &i64))
	bad("uint8 overflow", Uint([]byte(`256`), 0, &u8))
	bad("uint32 overflow", Uint([]byte(`4294967296`), 0, &u32))
	bad("uint64 overflow", Uint([]byte(`18446744073709551616`), 0, &u64))
	bad("uint64 overflow 20 digits", Uint([]byte(`21000000000000000000`), 0, &u64))
	bad("uint64 huge", Uint([]byte(`184467440737095516150`), 0, &u64))
	bad("uint64 negative", Uint([]byte(`-1`), 0, &u64))
	bad("uint64 float", Uint([]byte(`1.5`), 0, &u64))
	bad("float64 overflow", Float([]byte(`1e400`), 0, &f64))
	bad("float32 overflow", Float([]byte(`1e39`), 0, &f32))
	bad("float64 no digits", Float([]byte(`.5`), 0, &f64))
	bad("bool number", Bool([]byte(`1`), 0, &b))
	bad("bool truncated", Bool([]byte(`tru`), 0, &b))
}

// Valid must agree with encoding/json, except that encoding/json does not
// require the input to be valid UTF-8.
func FuzzValid(f *testing.F) {
	for _, s := range validCases {
		f.Add([]byte(s))
	}
	for _, s := range invalidCases {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if got, want := Valid(data), stdjson.Valid(data) && utf8.Valid(data); got != want {
			t.Fatalf("Valid(%q) = %v, want %v", data, got, want)
		}
	})
}

// String must decode a JSON string to what encoding/json decodes it to.
func FuzzString(f *testing.F) {
	for _, s := range validCases {
		f.Add([]byte(s))
	}
	f.Add([]byte("\"\\ud83d\\ude00 \xff \\n\""))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 || data[0] != '"' {
			return
		}
		var want string
		wantErr := stdjson.Unmarshal(data, &want)
		if wantErr == nil && !utf8.Valid(data) {
			// encoding/json replaces invalid UTF-8, cfjson rejects it.
			wantErr = errors.New("invalid UTF-8")
		}
		for _, flags := range []Flags{0, Prescan(data, 0)} {
			var got string
			n := String(data, 0, flags, &got)
			if n >= 0 && SkipSpace(data, n) != len(data) {
				n = -1
			}
			if (n < 0) != (wantErr != nil) {
				t.Fatalf("String(%q) returned %d, encoding/json error: %v", data, n, wantErr)
			}
			if n >= 0 && got != want {
				t.Fatalf("String(%q) = %q, encoding/json says %q", data, got, want)
			}
		}
	})
}

// Integers must decode to what encoding/json decodes them to, and be
// rejected when it rejects them.
func FuzzInt(f *testing.F) {
	for _, s := range []string{`0`, `-1`, `127`, `128`, `9223372036854775807`, `18446744073709551616`, `1e2`, `1.0`, `01`, `null`, `1 `, "7\n", `1 2`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 || data[0] <= ' ' {
			return
		}
		check := func(name string, n int, got any, want any, wantErr error) {
			// The decoders stop right after the number. encoding/json is
			// given the whole input, where whitespace may follow.
			if n >= 0 && SkipSpace(data, n) != len(data) {
				n = -1
			}
			if (n < 0) != (wantErr != nil) {
				t.Fatalf("%s(%q) returned %d, encoding/json error: %v", name, data, n, wantErr)
			}
			if n >= 0 && got != want {
				t.Fatalf("%s(%q) = %v, encoding/json says %v", name, data, got, want)
			}
		}
		var i8, wi8 int8
		n := Int(data, 0, &i8)
		err := stdjson.Unmarshal(data, &wi8)
		check("int8", n, i8, wi8, err)
		var i64, wi64 int64
		n = Int(data, 0, &i64)
		err = stdjson.Unmarshal(data, &wi64)
		check("int64", n, i64, wi64, err)
		var u32, wu32 uint32
		n = Uint(data, 0, &u32)
		err = stdjson.Unmarshal(data, &wu32)
		check("uint32", n, u32, wu32, err)
		var u64, wu64 uint64
		n = Uint(data, 0, &u64)
		err = stdjson.Unmarshal(data, &wu64)
		check("uint64", n, u64, wu64, err)
		var f64, wf64 float64
		n = Float(data, 0, &f64)
		err = stdjson.Unmarshal(data, &wf64)
		check("float64", n, f64, wf64, err)
	})
}

// scanString has a path for short strings, a windowed one for long strings
// and a tail, all of which must stop exactly where a plain loop does.
func TestScanString(t *testing.T) {
	naive := func(b []byte, i int, ascii bool) int {
		for ; i < len(b); i++ {
			if c := b[i]; c == '"' || c == '\\' || c < 0x20 || (ascii && c >= 0x80) {
				break
			}
		}
		return i
	}
	sizes := []int{0, 1, 7, 8, 9, 31, 32, 33, 39, 40, 41, 63, 64, 100, 127, 128, 129, 159, 160, 161, 200, 287, 288, 300, 420, 1000}
	for _, size := range sizes {
		for _, stop := range []byte{'"', '\\', 0, 0x1f, '\n', 0x80, 0xff, 'a'} {
			for pos := 0; pos <= size; pos++ {
				b := []byte(strings.Repeat("a", size))
				if pos < size {
					b[pos] = stop
				}
				for _, start := range []int{0, 1, 5} {
					if start > size {
						continue
					}
					for _, ascii := range []bool{false, true} {
						if got, want := scanString(b, start, ascii), naive(b, start, ascii); got != want {
							t.Fatalf("size %d, %#x at %d, start %d, ascii %v: got %d, want %d", size, stop, pos, start, ascii, got, want)
						}
					}
				}
			}
		}
	}
	// Two stop bytes: the first one wins, whichever kind comes first.
	for _, size := range []int{64, 300} {
		for first := 0; first < size; first += 7 {
			for second := first + 1; second < size; second += 13 {
				for _, pair := range []string{"\"\\", "\\\"", "\n\"", "\"\n", "\\\n", "\n\\"} {
					b := []byte(strings.Repeat("a", size))
					b[first], b[second] = pair[0], pair[1]
					if got := scanString(b, 0, false); got != first {
						t.Fatalf("size %d, %q at %d and %d: got %d", size, pair, first, second, got)
					}
				}
			}
		}
	}
}

// A string made of escape sequences with its closing quote far away must not
// be scanned again and again: decoding stays linear in the input size.
func TestSkip_EscapeHeavyStringIsLinear(t *testing.T) {
	build := func(n int) []byte {
		return []byte(`"` + strings.Repeat(strings.Repeat("a", 40)+`\\`, n) + `"`)
	}
	small, large := build(2000), build(64000)
	if !Valid(small) || !Valid(large) {
		t.Fatal("valid string rejected")
	}
	perByte := func(b []byte) float64 {
		res := testing.Benchmark(func(tb *testing.B) {
			for i := 0; i < tb.N; i++ {
				Skip(b, 0, 0)
			}
		})
		return float64(res.NsPerOp()) / float64(len(b))
	}
	// 32 times the input would take 32 times longer per byte if the work was
	// quadratic. Leave a lot of room for noise.
	if s, l := perByte(small), perByte(large); l > s*4 {
		t.Fatalf("time per byte grows with the input size: %.3fns for %d bytes, %.3fns for %d bytes", s, len(small), l, len(large))
	}
}

func TestUnmarshalString(t *testing.T) {
	for _, s := range []string{"", "plain", `q"uo\te`, "line\nbreak\ttab", "<b>&</b>", "h\xc3\xa9llo \xf0\x9f\x98\x80", "\x00\x1f", "\xe2\x80\xa8"} {
		encoded := AppendString(nil, s)
		if !Valid(encoded) {
			t.Fatalf("%q encoded to invalid JSON %q", s, encoded)
		}
		decoded, err := UnmarshalString(encoded)
		if err != nil || decoded != s {
			t.Fatalf("UnmarshalString(%q) = %q, %v; want %q", encoded, decoded, err, s)
		}
		// encoding/json must read it as the same string.
		var std string
		if err := stdjson.Unmarshal(encoded, &std); err != nil || std != s {
			t.Fatalf("encoding/json decoded %q to %q, %v", encoded, std, err)
		}
	}
	if s, err := UnmarshalString([]byte(" \"a\\n\" \n")); err != nil || s != "a\n" {
		t.Fatalf("got %q, %v", s, err)
	}
	for _, input := range []string{``, `null`, `1`, `"a`, `{}`, `"\x"`, `a`} {
		if s, err := UnmarshalString([]byte(input)); err == nil {
			t.Errorf("UnmarshalString(%q) = %q, want an error", input, s)
		}
	}
	for _, input := range []string{`"a"x`, `"a" "b"`} {
		if _, err := UnmarshalString([]byte(input)); !errors.Is(err, ErrTrailingData) {
			t.Errorf("UnmarshalString(%q): got %v, want ErrTrailingData", input, err)
		}
	}
	// The result is a copy.
	data := []byte(`"payload"`)
	s, _ := UnmarshalString(data)
	data[1] = 'X'
	if s != "payload" {
		t.Fatal("the result points into the input")
	}
}

// The prescan decides whether strings are checked at all, so it must never
// call input plain which is not.
func TestPrescan(t *testing.T) {
	for _, size := range []int{0, 1, 7, 8, 9, 63, 64, 65, 127, 128, 129, 200, 1000} {
		clean := []byte(strings.Repeat("a~b", size)[:size])
		if size >= 24 && Prescan(clean, 0)&plain == 0 {
			t.Fatalf("size %d: plain input not recognized", size)
		}
		for pos := 0; pos < size; pos++ {
			for _, c := range []byte{0, 1, 0x1f, '\n', '\\', 0x80, 0xc3, 0xff} {
				b := bytes.Clone(clean)
				b[pos] = c
				// Whitespace at the ends is not a part of the value.
				if c == '\n' && (pos == 0 || pos == size-1) {
					continue
				}
				if Prescan(b, 0)&plain != 0 {
					t.Fatalf("size %d, %#x at %d: called plain", size, c, pos)
				}
				if Prescan(b, plain) != 0 {
					t.Fatalf("size %d, %#x at %d: flags not reset", size, c, pos)
				}
			}
		}
	}
	// Whitespace around the value does not make it less plain.
	if Prescan([]byte(" \t\r\n{\"a\":1,\"bcdefghijklmnopq\":2}\r\n "), 0)&plain == 0 {
		t.Fatal("surrounding whitespace is counted")
	}
	if Prescan([]byte("{\"a\":\n1,\"bcdefghijklmnopq\":2}"), 0)&plain != 0 {
		t.Fatal("a newline inside the value is ignored")
	}
}

// With and without the prescan the outcome must be the same, for valid and
// for invalid input alike - including the offset of an error.
func TestPrescanDoesNotChangeResults(t *testing.T) {
	inputs := append(append([]string{}, validCases...), invalidCases...)
	// The prescan treats trailing whitespace as outside any string; in an
	// unterminated string it is inside one, and a control character there is
	// what the error must point at.
	inputs = append(inputs,
		`"an unterminated string ending in a control character`+"\r  ",
		`"an unterminated string ending in a tab and spaces`+"\t  ",
		`{"an unterminated key ending in a control character`+"\r\n",
	)
	// The prescan is skipped for short input, so every case is also tried
	// inside of a document which is long enough.
	for _, s := range append([]string{}, inputs...) {
		inputs = append(inputs, `["a string to make the input long enough",`+s+`]`, `"padding padding padding padding `+strings.Trim(s, `"`)+`"`)
	}
	plainInputs := 0
	for _, s := range inputs {
		if Prescan([]byte(s), 0)&plain != 0 {
			plainInputs++
		}
		b := []byte(s)
		i := SkipSpace(b, 0)
		slow, fast := Skip(b, i, 0), Skip(b, i, Prescan(b, 0))
		if slow != fast {
			t.Errorf("Skip(%q): %d without the prescan, %d with it", s, slow, fast)
		}
		var slowString, fastString string
		slow, fast = String(b, i, 0, &slowString), String(b, i, Prescan(b, 0), &fastString)
		if slow != fast || slowString != fastString {
			t.Errorf("String(%q): %d %q without the prescan, %d %q with it", s, slow, slowString, fast, fastString)
		}
		if i < len(b) && b[i] == '{' {
			k := SkipSpace(b, i+1)
			slowKey, slow := Key(b, k, 0)
			fastKey, fast := Key(b, k, Prescan(b, 0))
			if slow != fast || !bytes.Equal(slowKey, fastKey) {
				t.Errorf("Key(%q): %d %q without the prescan, %d %q with it", s, slow, slowKey, fast, fastKey)
			}
		}
		if got, want := Valid(b), stdjson.Valid(b) && utf8.Valid(b); got != want {
			t.Errorf("Valid(%q) = %v, want %v", s, got, want)
		}
	}
	if plainInputs < 50 {
		t.Fatalf("only %d inputs are plain, the fast path is barely tested", plainInputs)
	}
}

func TestFoldKey(t *testing.T) {
	for in, want := range map[string]string{"": "", "id": "id", "ID": "id", "Sub_Refresh": "sub_refresh", "\xc5\xbfub": "\xc5\xbfub"} {
		key := []byte(in)
		got, ok := FoldKey(key)
		if string(got) != want || ok != (in != want) {
			t.Errorf("FoldKey(%q) = %q, %v", in, got, ok)
		}
		if string(key) != in {
			t.Errorf("FoldKey(%q) modified its argument", in)
		}
	}
}

// Error tells broken JSON from a value of the wrong type also when the input
// is several values and the error is not in the first one.
func TestError_SeveralValues(t *testing.T) {
	tests := []struct {
		input  string
		offset int
		syntax bool
	}{
		{"{\"id\":1}\n{\"id\":", 15, true},
		{"{\"id\":1}\n{\"id\":\"x\"}", 14, false},
		{"{\"id\":\"x\"}\n{\"id\":", 6, false},
		{"", 0, true},
		{"{\"id\":1}\n", 9, true},
	}
	for _, tt := range tests {
		var e *DecodeError
		if err := Error([]byte(tt.input), ^tt.offset); !errors.As(err, &e) {
			t.Fatalf("%q: %v", tt.input, err)
		}
		if e.Syntax != tt.syntax || e.Offset != tt.offset {
			t.Errorf("%q: Syntax = %v at %d, want %v at %d", tt.input, e.Syntax, e.Offset, tt.syntax, tt.offset)
		}
	}
}

type indented struct{}

func (indented) MarshalJSON() ([]byte, error) {
	return []byte("{\n  \"a\": [\n    1\n  ]\n}\n"), nil
}

// What a MarshalJSON method returns goes through the same rules as a raw
// value: valid JSON, and no newlines, which delimit messages.
func TestAppendMarshaler_NoNewlines(t *testing.T) {
	got := AppendMarshaler(nil, indented{})
	if string(got) != `{  "a": [    1  ]}` {
		t.Fatalf("%q", got)
	}
}

type failing struct{ err error }

func (f failing) MarshalJSON() ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte(`{"a":`), nil
}

type withFailing struct{ f failing }

func (w *withFailing) AppendJSON(b []byte) []byte {
	b = append(b, `{"f":`...)
	b = AppendMarshaler(b, w.f)
	return append(b, '}')
}

// A MarshalJSON which fails is reported, as encoding/json reports it: not
// written over with something else.
func TestMarshal_MarshalerError(t *testing.T) {
	boom := errors.New("boom")
	for _, tt := range []struct {
		f    failing
		want error
	}{{failing{boom}, boom}, {failing{}, nil}} {
		func() {
			defer func() {
				if _, ok := recover().(*MarshalerError); !ok {
					t.Errorf("AppendMarshaler of %v did not panic with a *MarshalerError", tt.f)
				}
			}()
			AppendMarshaler(nil, tt.f)
		}()

		data, err := Marshal(&withFailing{tt.f})
		var me *MarshalerError
		if data != nil || !errors.As(err, &me) {
			t.Fatalf("Marshal = %q, %v", data, err)
		}
		if me.Type != "cfjson.failing" || (tt.want != nil && !errors.Is(err, tt.want)) {
			t.Fatalf("error %v of type %q", err, me.Type)
		}
	}

	// Without a failing method Marshal is AppendJSON.
	data, err := Marshal(appenderFunc(func(b []byte) []byte { return append(b, `{"a":1}`...) }))
	if err != nil || string(data) != `{"a":1}` {
		t.Fatalf("Marshal = %q, %v", data, err)
	}

	// Other panics are not Marshal's business.
	defer func() {
		if r := recover(); r != "other" {
			t.Fatalf("recovered %v", r)
		}
	}()
	_, _ = Marshal(appenderFunc(func([]byte) []byte { panic("other") }))
}

type appenderFunc func([]byte) []byte

func (f appenderFunc) AppendJSON(b []byte) []byte { return f(b) }
