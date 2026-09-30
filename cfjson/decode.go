package cfjson

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Flags alter the way generated decoders work.
type Flags uint

const (
	// ZeroCopy makes decoded strings (including map keys) point into the input
	// instead of being copied out of it, when the JSON string is plain ASCII
	// without escape sequences. The input must then stay unmodified for as
	// long as the decoded value is in use. Raw values are always copied.
	ZeroCopy Flags = 1 << iota

	// plain is set by Prescan for input which has nothing but printable
	// ASCII in it and no backslash. A string then ends at the next quote,
	// whatever is in front of it, which makes finding its end all there is
	// to do.
	plain
)

// Prescan looks at the whole input once and returns f with what it has found
// out added to it, for decoders to use. Calling it is optional, and only
// worth it right before decoding all of b: Unmarshal and Valid do.
//
// Most messages are plain ASCII without escape sequences. Checking that for
// the whole input at once is much cheaper than checking every string on its
// own, since the byte search of the Go runtime is vectorized.
func Prescan(b []byte, f Flags) Flags {
	// Not worth two passes for something this short.
	if len(b) < 24 {
		return f &^ plain
	}
	// Whitespace around the value (a trailing newline, typically) is not a
	// part of any string.
	start := SkipSpace(b, 0)
	end := len(b)
	for end > start {
		if c := b[end-1]; c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			break
		}
		end--
	}
	b = b[start:end]
	if bytes.IndexByte(b, '\\') < 0 && printableASCII(b) {
		return f | plain
	}
	return f &^ plain
}

// printableASCII reports whether every byte of s is in the range 0x20..0x7F:
// no control characters and nothing outside of ASCII.
func printableASCII(s []byte) bool {
	const c = swarLo * 0x20
	// Subtracting 0x20 from a byte below 0x20 borrows, which sets its high
	// bit, and a byte outside of ASCII has the bit set to begin with. A
	// borrow may spill into the next byte, but only when there is a byte
	// below 0x20 already, so as a test for "any" this is exact. 64 bytes are
	// checked per iteration, with a single branch.
	for len(s) >= 64 {
		x0, x1 := binary.LittleEndian.Uint64(s), binary.LittleEndian.Uint64(s[8:])
		x2, x3 := binary.LittleEndian.Uint64(s[16:]), binary.LittleEndian.Uint64(s[24:])
		x4, x5 := binary.LittleEndian.Uint64(s[32:]), binary.LittleEndian.Uint64(s[40:])
		x6, x7 := binary.LittleEndian.Uint64(s[48:]), binary.LittleEndian.Uint64(s[56:])
		m := (x0 - c) | (x1 - c) | (x2 - c) | (x3 - c) | (x4 - c) | (x5 - c) | (x6 - c) | (x7 - c) |
			x0 | x1 | x2 | x3 | x4 | x5 | x6 | x7
		if m&swarHi != 0 {
			return false
		}
		s = s[64:]
	}
	for len(s) >= 8 {
		x := binary.LittleEndian.Uint64(s)
		if ((x-c)|x)&swarHi != 0 {
			return false
		}
		s = s[8:]
	}
	for _, ch := range s {
		if ch < 0x20 || ch >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// The bits of Flags above the options count how deep decoding is inside
// values of recursive types, which is what bounds the stack decoders use:
// everything else nests only as deep as the types do.
const (
	depthShift = 8

	// MaxDepth is how deep values of recursive types may be nested, more is
	// an error. A recursive type is a tree of some kind, a filter expression
	// for example, and a tree written by a person or by a program is nowhere
	// near this deep. encoding/json allows 10000 levels of nesting.
	MaxDepth = 1024

	// DepthStep is added to the flags by the generated decoder of a
	// recursive type, which then fails if the result reached DepthLimit.
	DepthStep  Flags = 1 << depthShift
	DepthLimit Flags = (MaxDepth + 1) << depthShift
)

// DecodeError is returned by Error.
type DecodeError struct {
	// Offset is the position in the input where decoding stopped.
	Offset int
	// Syntax is true if the input is not valid JSON, and false if it is valid
	// but is not accepted for what it is decoded into: a value of another
	// type or out of range, a key which comes twice, values of a recursive
	// type nested deeper than MaxDepth.
	Syntax bool
}

func (e *DecodeError) Error() string {
	if e.Syntax {
		return "cfjson: invalid JSON at offset " + strconv.Itoa(e.Offset)
	}
	return "cfjson: JSON value does not match the destination type at offset " + strconv.Itoa(e.Offset)
}

// Error converts a negative index returned by a decoding function into an
// error. b must be the input which was being decoded.
func Error(b []byte, n int) error {
	if n >= 0 {
		return nil
	}
	// Decoders stop at the first value they cannot use, so telling a syntax
	// error from a type mismatch takes another look at the input. This is the
	// error path, it does not have to be fast.
	// The input may be several values one after another, the one the error
	// is in has to be found.
	offset, syntax := ^n, true
	for i := SkipSpace(b, 0); i < len(b); {
		end := Skip(b, i, 0)
		if end < 0 {
			break
		}
		if end > offset {
			// The value is fine as JSON.
			syntax = false
			break
		}
		i = SkipSpace(b, end)
	}
	return &DecodeError{Offset: offset, Syntax: syntax}
}

// Valid reports whether b is a single valid JSON value, optionally surrounded
// by whitespace. As RFC 8259 requires, it must be valid UTF-8.
func Valid(b []byte) bool {
	i := Skip(b, SkipSpace(b, 0), Prescan(b, 0))
	return i >= 0 && SkipSpace(b, i) == len(b)
}

// SkipSpace returns the index of the first non-whitespace byte at or after i,
// or len(b) if there is none.
func SkipSpace(b []byte, i int) int {
	if i < len(b) && b[i] > ' ' {
		return i
	}
	return skipSpaceSlow(b, i)
}

func skipSpaceSlow(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// IsNull reports whether the value at i is null.
func IsNull(b []byte, i int) bool {
	return i+4 <= len(b) && b[i] == 'n' && b[i+1] == 'u' && b[i+2] == 'l' && b[i+3] == 'l'
}

// Null skips a null at i. It is what decoders fall back to when a value does
// not start the way the destination type needs: null is accepted everywhere
// and leaves the destination untouched, anything else is an error.
func Null(b []byte, i int) int {
	if IsNull(b, i) {
		return i + 4
	}
	return ^i
}

// swarStringEnd flags the bytes of x at which scanning a JSON string has to
// stop: '"', '\\' and control characters. Only the lowest flag is exact.
func swarStringEnd(x uint64) uint64 {
	q := x ^ (swarLo * '"')
	m := (q - swarLo) &^ q
	q = x ^ (swarLo * '\\')
	m |= (q - swarLo) &^ q
	m |= (x - swarLo*0x20) &^ x
	return m & swarHi
}

// scanQuote returns the index of the first '"' at or after i, or len(b) if
// there is none. It is what scanning a string comes down to for plain input.
func scanQuote(b []byte, i int) int {
	// Most strings are short: object keys, channel names, identifiers.
	for n := 0; n < 2 && i+8 <= len(b); n++ {
		q := load64(b, i) ^ (swarLo * '"')
		if m := (q - swarLo) &^ q & swarHi; m != 0 {
			return i + bits.TrailingZeros64(m)>>3
		}
		i += 8
	}
	if n := bytes.IndexByte(b[i:], '"'); n >= 0 {
		return i + n
	}
	return len(b)
}

// scanString returns the index of the first byte at or after i which is '"',
// '\\', a control character or (if ascii is set) a non-ASCII byte. It returns
// len(b) if there is no such byte.
func scanString(b []byte, i int, ascii bool) int {
	var hi uint64
	if ascii {
		hi = swarHi
	}
	// Most strings are short: object keys, channel names, identifiers.
	for n := 0; n < 4 && i+8 <= len(b); n++ {
		x := load64(b, i)
		if m := swarStringEnd(x) | x&hi; m != 0 {
			return i + bits.TrailingZeros64(m)>>3
		}
		i += 8
	}
	if i+8 <= len(b) {
		return scanLongString(b, i, hi)
	}
	for ; i < len(b); i++ {
		c := b[i]
		if c == '"' || c == '\\' || c < 0x20 || (ascii && c >= utf8.RuneSelf) {
			return i
		}
	}
	return i
}

// scanLongString is scanString for strings which turned out to be long, such
// as payloads: it looks for the quote and the backslash with bytes.IndexByte,
// which is vectorized, and checks what is in front of them for control
// characters several words at a time.
//
// The input is processed in windows which start small and double while
// nothing is found. A string with a backslash every few bytes and the closing
// quote far away would otherwise have the whole rest of the input searched
// for a quote again after every escape sequence.
func scanLongString(b []byte, i int, hi uint64) int {
	for window := 256; i < len(b); window = min(2*window, 1<<30) {
		span := b[i : i+min(window, len(b)-i)]
		n := len(span)
		if q := bytes.IndexByte(span, '"'); q >= 0 {
			span = span[:q]
		}
		if q := bytes.IndexByte(span, '\\'); q >= 0 {
			span = span[:q]
		}
		if c := scanControl(span, hi); c >= 0 {
			return i + c
		}
		if len(span) < n {
			return i + len(span)
		}
		i += n
	}
	return i
}

// scanControl returns the index of the first byte of s which is a control
// character or has a bit of hi set (hi is swarHi to look for non-ASCII bytes,
// or zero), and -1 if there is none.
func scanControl(s []byte, hi uint64) int {
	const c = swarLo * 0x20
	n := len(s)
	// Subtracting 0x20 from a byte below 0x20 borrows, which sets its high
	// bit; and-not with the byte itself drops bytes which had the bit set
	// already. 64 bytes are checked per iteration, with a single branch.
	for len(s) >= 64 {
		x0, x1 := binary.LittleEndian.Uint64(s), binary.LittleEndian.Uint64(s[8:])
		x2, x3 := binary.LittleEndian.Uint64(s[16:]), binary.LittleEndian.Uint64(s[24:])
		x4, x5 := binary.LittleEndian.Uint64(s[32:]), binary.LittleEndian.Uint64(s[40:])
		x6, x7 := binary.LittleEndian.Uint64(s[48:]), binary.LittleEndian.Uint64(s[56:])
		m := (x0-c)&^x0 | (x1-c)&^x1 | (x2-c)&^x2 | (x3-c)&^x3 |
			(x4-c)&^x4 | (x5-c)&^x5 | (x6-c)&^x6 | (x7-c)&^x7 |
			(x0|x1|x2|x3|x4|x5|x6|x7)&hi
		if m&swarHi != 0 {
			break
		}
		s = s[64:]
	}
	for len(s) >= 8 {
		x := binary.LittleEndian.Uint64(s)
		if m := ((x-c)&^x | x&hi) & swarHi; m != 0 {
			return n - len(s) + bits.TrailingZeros64(m)>>3
		}
		s = s[8:]
	}
	for i, ch := range s {
		if ch < 0x20 || (hi != 0 && ch >= utf8.RuneSelf) {
			return n - len(s) + i
		}
	}
	return -1
}

// skipString validates the string starting at b[i] == '"' and returns the
// index after its closing quote.
func skipString(b []byte, i int, f Flags) int {
	i++
	if f&plain != 0 {
		if i = scanQuote(b, i); i >= len(b) {
			return ^i
		}
		return i + 1
	}
	start := i
	// Plain ASCII strings, which is most of them, are done in one scan.
	i = scanString(b, i, true)
	if i < len(b) && b[i] == '"' {
		return i + 1
	}
	for {
		if i >= len(b) {
			return ^i
		}
		switch c := b[i]; {
		case c == '"':
			// Escape sequences are ASCII, so the string is valid UTF-8 if
			// and only if what is between the quotes is.
			if !utf8.Valid(b[start:i]) {
				return ^start
			}
			return i + 1
		case c == '\\':
			if i = skipEscape(b, i); i < 0 {
				return i
			}
		case c < 0x20:
			return ^i
		default:
			i++
		}
		i = scanString(b, i, false)
	}
}

// skipEscape validates the escape sequence starting at b[i] == '\\' and
// returns the index after it.
func skipEscape(b []byte, i int) int {
	if i+1 >= len(b) {
		return ^len(b)
	}
	switch b[i+1] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return i + 2
	case 'u':
		if i+6 > len(b) {
			return ^len(b)
		}
		if hexDigit[b[i+2]]|hexDigit[b[i+3]]|hexDigit[b[i+4]]|hexDigit[b[i+5]] > 0xF {
			return ^(i + 2)
		}
		return i + 6
	}
	return ^(i + 1)
}

// hexDigit maps a hexadecimal digit to its value, and anything else to 0xFF.
var hexDigit = func() (t [256]byte) {
	for i := range t {
		t[i] = 0xFF
	}
	for c := '0'; c <= '9'; c++ {
		t[c] = byte(c - '0')
	}
	for c := 'a'; c <= 'f'; c++ {
		t[c] = byte(c - 'a' + 10)
		t[c-'a'+'A'] = byte(c - 'a' + 10)
	}
	return t
}()

// skipNumber validates the number starting at b[i] and returns the index
// after it.
func skipNumber(b []byte, i int) int {
	if i < len(b) && b[i] == '-' {
		i++
	}
	if i >= len(b) {
		return ^i
	}
	switch c := b[i]; {
	case c == '0':
		i++
	case c >= '1' && c <= '9':
		i++
		for i < len(b) && b[i]-'0' <= 9 {
			i++
		}
	default:
		return ^i
	}
	if i < len(b) && b[i] == '.' {
		i++
		if i >= len(b) || b[i]-'0' > 9 {
			return ^i
		}
		for i < len(b) && b[i]-'0' <= 9 {
			i++
		}
	}
	if i < len(b) && b[i]|0x20 == 'e' {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || b[i]-'0' > 9 {
			return ^i
		}
		for i < len(b) && b[i]-'0' <= 9 {
			i++
		}
	}
	return i
}

func skipLiteral(b []byte, i int, lit string) int {
	if i+len(lit) <= len(b) && string(b[i:i+len(lit)]) == lit {
		return i + len(lit)
	}
	return ^i
}

// Skip validates the JSON value starting at b[i] and returns the index after
// it. It is used for unknown object fields and for raw values.
//
// Skip does not recurse, so deeply nested input cannot exhaust the stack.
func Skip(b []byte, i int, f Flags) int {
	if i >= len(b) {
		return ^i
	}
	switch b[i] {
	case '"':
		return skipString(b, i, f)
	case '{', '[':
		return skipContainer(b, i, f)
	case 't':
		return skipLiteral(b, i, "true")
	case 'f':
		return skipLiteral(b, i, "false")
	case 'n':
		return skipLiteral(b, i, "null")
	}
	return skipNumber(b, i)
}

// skipContainer validates the object or array starting at b[i] and returns
// the index after it.
func skipContainer(b []byte, i int, f Flags) int {
	// stack has a bit per open container, the innermost one in the lowest
	// bit: 1 for an object, 0 for an array. Words which do not fit are moved
	// to deep, so nesting is only limited by the input size.
	var stack uint64
	var deep []uint64
	depth := 0

open:
	if depth != 0 && depth&63 == 0 {
		deep = append(deep, stack)
	}
	stack <<= 1
	depth++
	if b[i] == '{' {
		stack |= 1
		i = SkipSpace(b, i+1)
		if i >= len(b) {
			return ^i
		}
		if b[i] == '}' {
			goto closed
		}
		goto key
	}
	i = SkipSpace(b, i+1)
	if i >= len(b) {
		return ^i
	}
	if b[i] == ']' {
		goto closed
	}
	goto value

key:
	if i >= len(b) || b[i] != '"' {
		return ^i
	}
	if i = skipString(b, i, f); i < 0 {
		return i
	}
	i = SkipSpace(b, i)
	if i >= len(b) || b[i] != ':' {
		return ^i
	}
	i = SkipSpace(b, i+1)

value:
	if i >= len(b) {
		return ^i
	}
	switch b[i] {
	case '{', '[':
		goto open
	case '"':
		i = skipString(b, i, f)
	case 't':
		i = skipLiteral(b, i, "true")
	case 'f':
		i = skipLiteral(b, i, "false")
	case 'n':
		i = skipLiteral(b, i, "null")
	default:
		i = skipNumber(b, i)
	}
	if i < 0 {
		return i
	}

next:
	i = SkipSpace(b, i)
	if i >= len(b) {
		return ^i
	}
	switch b[i] {
	case ',':
		i = SkipSpace(b, i+1)
		if stack&1 != 0 {
			goto key
		}
		goto value
	case '}':
		if stack&1 == 0 {
			return ^i
		}
	case ']':
		if stack&1 != 0 {
			return ^i
		}
	default:
		return ^i
	}

closed:
	i++
	depth--
	stack >>= 1
	if depth == 0 {
		return i
	}
	if depth&63 == 0 {
		stack = deep[len(deep)-1]
		deep = deep[:len(deep)-1]
	}
	goto next
}

// Key decodes an object key starting at b[i] together with the colon which
// follows it, and returns the key and the index of the value.
//
// The returned key is a part of b unless it had to be unescaped: it is meant
// to be compared with field names (switch string(key) does not allocate), not
// to be stored or modified.
func Key(b []byte, i int, f Flags) ([]byte, int) {
	if i >= len(b) || b[i] != '"' {
		return nil, ^i
	}
	i++
	var key []byte
	var j int
	if f&plain != 0 {
		j = scanQuote(b, i)
	} else {
		j = scanString(b, i, true)
	}
	if j < len(b) && b[j] == '"' {
		key = b[i:j]
		j++
	} else {
		var s string
		if j = stringSlow(b, i, j, &s); j < 0 {
			return nil, j
		}
		key = []byte(s)
	}
	j = SkipSpace(b, j)
	if j >= len(b) || b[j] != ':' {
		return nil, ^j
	}
	return key, SkipSpace(b, j+1)
}

// MapKey is like Key for keys which are stored: the key is decoded into *p
// the way String does it.
func MapKey(b []byte, i int, f Flags, p *string) int {
	if i >= len(b) || b[i] != '"' {
		return ^i
	}
	i = String(b, i, f, p)
	if i < 0 {
		return i
	}
	i = SkipSpace(b, i)
	if i >= len(b) || b[i] != ':' {
		return ^i
	}
	return SkipSpace(b, i+1)
}

// FoldKey returns key with ASCII letters in lower case, and whether that is
// different from key. It is what decoders generated with the option to match
// keys without regard to letter case use when a key is not the name of a
// field. The result is a new slice when it differs, key is never modified.
func FoldKey(key []byte) (folded []byte, ok bool) {
	for i := 0; i < len(key); i++ {
		if c := key[i]; c >= 'A' && c <= 'Z' {
			folded = append([]byte(nil), key...)
			for ; i < len(folded); i++ {
				if c := folded[i]; c >= 'A' && c <= 'Z' {
					folded[i] = c + ('a' - 'A')
				}
			}
			return folded, true
		}
	}
	return key, false
}

// String decodes the JSON string at b[i] into *p and returns the index after
// it. A null leaves *p untouched.
func String(b []byte, i int, f Flags, p *string) int {
	if i >= len(b) || b[i] != '"' {
		return Null(b, i)
	}
	i++
	var j int
	if f&plain != 0 {
		j = scanQuote(b, i)
	} else {
		j = scanString(b, i, true)
	}
	if j < len(b) && b[j] == '"' {
		if f&ZeroCopy != 0 {
			*p = aliasString(b[i:j])
		} else {
			*p = string(b[i:j])
		}
		return j + 1
	}
	return stringSlow(b, i, j, p)
}

// stringSlow decodes a string which is not plain ASCII up to its closing
// quote: start is the index after the opening quote, and b[start:j] is known
// to need no processing.
func stringSlow(b []byte, start, j int, p *string) int {
	escaped := false
	for {
		j = scanString(b, j, false)
		if j >= len(b) {
			return ^j
		}
		c := b[j]
		if c == '"' {
			break
		}
		if c != '\\' {
			return ^j
		}
		escaped = true
		if j = skipEscape(b, j); j < 0 {
			return j
		}
	}
	s := b[start:j]
	switch {
	case !utf8.Valid(s):
		return ^start
	case escaped:
		*p = unescape(s)
	default:
		// Strings which are not plain ASCII are copied even with ZeroCopy.
		// There is no technical need for it, but it is what the decoder
		// this one replaced did, and code may have come to rely on such
		// strings outliving the input.
		*p = string(s)
	}
	return j + 1
}

// unescape returns the string the body s of a JSON string stands for. s is
// valid UTF-8 and its escape sequences have been validated by skipEscape.
func unescape(s []byte) string {
	// Unescaping never makes a string longer: the shortest escape sequence
	// is two bytes for one, and \uXXXX is six bytes for at most three.
	dst := make([]byte, 0, len(s))
	for len(s) > 0 {
		n := 0
		for n < len(s) && s[n] != '\\' {
			n++
		}
		dst = append(dst, s[:n]...)
		s = s[n:]
		if len(s) == 0 {
			break
		}
		c := s[1]
		if c != 'u' {
			switch c {
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			}
			dst = append(dst, c)
			s = s[2:]
			continue
		}
		r := hex4(s[2:])
		s = s[6:]
		if utf16.IsSurrogate(r) {
			// A valid pair becomes one rune. Anything else becomes U+FFFD,
			// and what follows is then decoded on its own.
			r2 := rune(-1)
			if len(s) >= 6 && s[0] == '\\' && s[1] == 'u' {
				r2 = hex4(s[2:])
			}
			if r = utf16.DecodeRune(r, r2); r != utf8.RuneError {
				s = s[6:]
			}
		}
		dst = utf8.AppendRune(dst, r)
	}
	return string(dst)
}

func hex4(s []byte) rune {
	return rune(hexDigit[s[0]])<<12 | rune(hexDigit[s[1]])<<8 | rune(hexDigit[s[2]])<<4 | rune(hexDigit[s[3]])
}

// Raw decodes the JSON value at b[i] as is: the value is validated and copied
// into *p, reusing its capacity. Unlike for other types, a null is not
// special: it is stored as the four bytes of `null`, which is what a
// json.Unmarshaler such as json.RawMessage gets from encoding/json.
func Raw(b []byte, i int, f Flags, p *[]byte) int {
	j := Skip(b, i, f)
	if j < 0 {
		return j
	}
	*p = append((*p)[:0], b[i:j]...)
	return j
}

// Bool decodes the JSON boolean at b[i] into *p and returns the index after
// it. A null leaves *p untouched.
func Bool[T ~bool](b []byte, i int, p *T) int {
	if i+4 <= len(b) {
		if b[i] == 't' && b[i+1] == 'r' && b[i+2] == 'u' && b[i+3] == 'e' {
			*p = true
			return i + 4
		}
		if i+5 <= len(b) && b[i] == 'f' && b[i+1] == 'a' && b[i+2] == 'l' && b[i+3] == 's' && b[i+4] == 'e' {
			*p = false
			return i + 5
		}
	}
	return Null(b, i)
}

// Signed is the constraint of types Int decodes into.
type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Unsigned is the constraint of types Uint decodes into.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Floating is the constraint of types Float decodes into.
type Floating interface {
	~float32 | ~float64
}

// Uint decodes the JSON number at b[i] into *p and returns the index after
// it. A null leaves *p untouched. Numbers with a sign, a fraction or an
// exponent, and numbers which do not fit T are an error.
func Uint[T Unsigned](b []byte, i int, p *T) int {
	v, j := parseUint(b, i)
	if j < 0 {
		return Null(b, i)
	}
	if uint64(T(v)) != v {
		return ^i
	}
	*p = T(v)
	return j
}

// Int is like Uint for signed integers.
func Int[T Signed](b []byte, i int, p *T) int {
	if i < len(b) && b[i] == '-' {
		v, j := parseUint(b, i+1)
		if j < 0 {
			return ^i
		}
		var n int64
		switch {
		case v <= math.MaxInt64:
			n = -int64(v)
		case v == 1<<63:
			n = math.MinInt64
		default:
			return ^i
		}
		if int64(T(n)) != n {
			return ^i
		}
		*p = T(n)
		return j
	}
	v, j := parseUint(b, i)
	if j < 0 {
		return Null(b, i)
	}
	if v > math.MaxInt64 || int64(T(v)) != int64(v) {
		return ^i
	}
	*p = T(v)
	return j
}

// parseUint parses the unsigned integer at b[i]. It returns a negative index
// if there is none, if it overflows uint64, or if what follows makes it a
// float.
func parseUint(b []byte, i int) (uint64, int) {
	if i >= len(b) {
		return 0, ^i
	}
	c := b[i] - '0'
	if c > 9 {
		return 0, ^i
	}
	v := uint64(c)
	j := i + 1
	if c == 0 {
		// No leading zeros.
		if j < len(b) && b[j]-'0' <= 9 {
			return 0, ^j
		}
	} else {
		for ; j < len(b); j++ {
			c = b[j] - '0'
			if c > 9 {
				break
			}
			// 19 digits always fit uint64.
			if j-i >= 19 {
				hi, lo := bits.Mul64(v, 10)
				sum, carry := bits.Add64(lo, uint64(c), 0)
				if hi != 0 || carry != 0 {
					return 0, ^i
				}
				v = sum
				continue
			}
			v = v*10 + uint64(c)
		}
	}
	if j < len(b) {
		if c := b[j]; c == '.' || c|0x20 == 'e' {
			return 0, ^j
		}
	}
	return v, j
}

// Float decodes the JSON number at b[i] into *p and returns the index after
// it. A null leaves *p untouched.
func Float[T Floating](b []byte, i int, p *T) int {
	j := skipNumber(b, i)
	if j < 0 {
		return Null(b, i)
	}
	// 2^24+1 is the smallest integer a float32 cannot hold.
	bitSize := 64
	if x := float64(1<<24 + 1); float64(T(x)) != x {
		bitSize = 32
	}
	v, err := strconv.ParseFloat(string(b[i:j]), bitSize)
	if err != nil {
		return ^i
	}
	*p = T(v)
	return j
}
