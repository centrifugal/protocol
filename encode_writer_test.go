package protocol

import (
	stdjson "encoding/json"
	"errors"
	"math"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// buildString runs s through the writer and returns the encoded JSON string.
func buildString(t *testing.T, s string, noEscapeHTML bool) string {
	t.Helper()
	w := newWriter()
	w.NoEscapeHTML = noEscapeHTML
	w.String(s)
	out, err := w.BuildBytes()
	require.NoError(t, err)
	return string(out)
}

// writer.String escapes every string field of every JSON message this package
// encodes - channel names, client IDs, error messages - and SDKs in other
// languages decode the result with their own JSON parsers.
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
func TestWriter_String(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", `""`},
		{"plain", "hello world", `"hello world"`},
		{"quote and backslash", `he said "hi" \ there`, `"he said \"hi\" \\ there"`},
		{"short escapes", "\n\r\t", `"\n\r\t"`},
		{"control chars", "\b\f\x00\x01\x0b\x1f", `"\u0008\u000c\u0000\u0001\u000b\u001f"`},
		// <, > and & are escaped by default so an encoded frame stays safe to
		// embed into an HTML page.
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
			got := buildString(t, tt.in, false)
			require.Equal(t, tt.want, got)

			// Whichever spelling is used, the result must be a JSON string that
			// decodes back to the input - with invalid UTF-8 replaced, which is
			// the one transformation the writer is allowed to make.
			var back string
			require.NoError(t, stdjson.Unmarshal([]byte(got), &back))
			if utf8.ValidString(tt.in) {
				require.Equal(t, tt.in, back)
			} else {
				require.True(t, utf8.ValidString(back))
			}
		})
	}
}

// With NoEscapeHTML the writer switches from htmlSafeSet to safeSet, so <, >
// and & are passed through while everything JSON itself requires escaping still
// is.
func TestWriter_String_NoEscapeHTML(t *testing.T) {
	const in = "<script>&</script>\n\"q\""
	got := buildString(t, in, true)
	require.Equal(t, `"<script>&</script>\n\"q\""`, got)

	var back string
	require.NoError(t, stdjson.Unmarshal([]byte(got), &back))
	require.Equal(t, in, back)
}

// Raw is how an already encoded payload (Publication data, an RPC result)
// reaches the frame. An absent payload must become a JSON null rather than
// nothing at all, which would leave a malformed message on the wire.
func TestWriter_Raw(t *testing.T) {
	tests := []struct {
		name string
		src  []byte
		want string
	}{
		{"nil becomes null", nil, "null"},
		{"empty becomes null", []byte{}, "null"},
		{"payload passed through as is", []byte(`{"a":  1}`), `{"a":  1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter()
			w.Raw(tt.src, nil)
			out, err := w.BuildBytes()
			require.NoError(t, err)
			require.Equal(t, tt.want, string(out))
		})
	}
}

// A MarshalJSON error handed to Raw must surface from BuildBytes instead of
// being dropped, otherwise a half-built frame would go out on the wire.
func TestWriter_Raw_Error(t *testing.T) {
	marshalErr := errors.New("marshal failed")

	w := newWriter()
	w.RawString(`{"data":`)
	w.Raw(nil, marshalErr)
	out, err := w.BuildBytes()
	require.ErrorIs(t, err, marshalErr)
	require.Nil(t, out)
}

// Once the writer holds an error it must stay on it: later Raw calls must
// neither overwrite the first error nor append to the buffer.
func TestWriter_Raw_KeepsFirstError(t *testing.T) {
	first := errors.New("first")
	second := errors.New("second")

	w := newWriter()
	w.Raw(nil, first)
	w.Raw([]byte(`{"a":1}`), second)
	w.Raw([]byte(`{"b":2}`), nil)
	_, err := w.BuildBytes()
	require.ErrorIs(t, err, first)
}

func TestWriter_Scalars(t *testing.T) {
	tests := []struct {
		name  string
		write func(*writer)
		want  string
	}{
		{"uint32 zero", func(w *writer) { w.Uint32(0) }, "0"},
		{"uint32 max", func(w *writer) { w.Uint32(math.MaxUint32) }, "4294967295"},
		{"uint64 max", func(w *writer) { w.Uint64(math.MaxUint64) }, "18446744073709551615"},
		{"int32 negative", func(w *writer) { w.Int32(-1) }, "-1"},
		{"int32 min", func(w *writer) { w.Int32(math.MinInt32) }, "-2147483648"},
		{"int64 min", func(w *writer) { w.Int64(math.MinInt64) }, "-9223372036854775808"},
		{"int64 max", func(w *writer) { w.Int64(math.MaxInt64) }, "9223372036854775807"},
		{"bool true", func(w *writer) { w.Bool(true) }, "true"},
		{"bool false", func(w *writer) { w.Bool(false) }, "false"},
		{"raw byte", func(w *writer) { w.RawByte('[') }, "["},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter()
			tt.write(w)
			out, err := w.BuildBytes()
			require.NoError(t, err)
			require.Equal(t, tt.want, string(out))
		})
	}
}

// BuildBytes writes into the caller supplied buffer when it is large enough,
// which is what the reuse argument of the push encoders relies on to avoid an
// allocation per subscriber.
func TestWriter_BuildBytes_Reuse(t *testing.T) {
	w := newWriter()
	w.RawString(`{"id":1}`)
	reuse := make([]byte, 0, 64)
	out, err := w.BuildBytes(reuse)
	require.NoError(t, err)
	require.Equal(t, `{"id":1}`, string(out))
	require.Same(t, &reuse[:1][0], &out[0], "expected the result to be written into the reuse buffer")

	// A buffer which is too small must be left alone and a new one allocated.
	w = newWriter()
	w.RawString(`{"id":1}`)
	small := make([]byte, 0, 2)
	out, err = w.BuildBytes(small)
	require.NoError(t, err)
	require.Equal(t, `{"id":1}`, string(out))
	require.Equal(t, 2, cap(small))
}

// BuildBytesNoCopy hands out the writer buffer itself, so it must return the
// same bytes BuildBytes would have copied out, and must still report an error
// the writer collected along the way.
func TestWriter_BuildBytesNoCopy(t *testing.T) {
	w := newWriter()
	w.RawString(`{"id":`)
	w.Uint32(7)
	w.RawByte('}')
	out, err := w.BuildBytesNoCopy()
	require.NoError(t, err)
	require.Equal(t, `{"id":7}`, string(out))

	w = newWriter()
	w.Raw(nil, errors.New("boom"))
	out, err = w.BuildBytesNoCopy()
	require.Error(t, err)
	require.Nil(t, out)
}
