package cfjson

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"unicode/utf8"
)

const (
	swarLo = 0x0101010101010101
	swarHi = 0x8080808080808080
)

// swarUnsafe reports (as a non-zero value) whether any of the 8 bytes of x
// needs escaping inside an HTML-safe JSON string: control characters, '"',
// '\\', '<', '>', '&' and everything outside ASCII.
func swarUnsafe(x uint64) uint64 {
	// Bytes below 0x20.
	m := x - swarLo*0x20
	// Bytes equal to '"', '\\', '<', '>', '&'.
	q := x ^ (swarLo * '"')
	m |= q - swarLo
	q = x ^ (swarLo * '\\')
	m |= q - swarLo
	q = x ^ (swarLo * '<')
	m |= q - swarLo
	q = x ^ (swarLo * '>')
	m |= q - swarLo
	q = x ^ (swarLo * '&')
	m |= q - swarLo
	// The subtractions above set the high bit of a byte which matched, and may
	// also set it for bytes >= 0x80 or because of a borrow out of a lower byte
	// which matched. Both only ever happen when the word really has a byte
	// needing escaping or a non-ASCII byte, which is handled below anyway.
	return (m | x) & swarHi
}

// AppendString appends s to b as a JSON string.
//
// Escaping matches encoding/json with HTML escaping enabled: '<', '>' and '&'
// are written as \u00XX, U+2028 and U+2029 are escaped, and invalid UTF-8 is
// replaced by \ufffd.
func AppendString(b []byte, s string) []byte {
	// Strings of 8 to 16 bytes, which is what most identifiers and channel
	// names are, fit two words which may overlap. The same two words serve
	// to check that nothing needs escaping and to write the string, which
	// saves the call copying a string of unknown length takes.
	if n, l := len(s), len(b); n >= 8 && n <= 16 && cap(b)-l >= n+2 {
		x0, x1 := loadString64(s, 0), loadString64(s, n-8)
		if swarUnsafe(x0)|swarUnsafe(x1) == 0 {
			b = b[:l+n+2]
			b[l] = '"'
			binary.LittleEndian.PutUint64(b[l+1:], x0)
			binary.LittleEndian.PutUint64(b[l+n-7:], x1)
			b[l+n+1] = '"'
			return b
		}
	} else if n >= 4 && n < 8 && cap(b)-l >= n+2 {
		// And with two halves of a word for the shortest ones.
		x0, x1 := loadString32(s, 0), loadString32(s, n-4)
		if swarUnsafe(uint64(x0)|uint64(x1)<<32) == 0 {
			b = b[:l+n+2]
			b[l] = '"'
			binary.LittleEndian.PutUint32(b[l+1:], x0)
			binary.LittleEndian.PutUint32(b[l+n-3:], x1)
			b[l+n+1] = '"'
			return b
		}
	} else if n > 16 && n <= 32 && cap(b)-l >= n+2 {
		// The same with four words for strings up to 32 bytes: identifiers
		// like UUIDs.
		x0, x1, x2, x3 := loadString64(s, 0), loadString64(s, 8), loadString64(s, n-16), loadString64(s, n-8)
		if swarUnsafe(x0)|swarUnsafe(x1)|swarUnsafe(x2)|swarUnsafe(x3) == 0 {
			b = b[:l+n+2]
			b[l] = '"'
			binary.LittleEndian.PutUint64(b[l+1:], x0)
			binary.LittleEndian.PutUint64(b[l+9:], x1)
			binary.LittleEndian.PutUint64(b[l+n-15:], x2)
			binary.LittleEndian.PutUint64(b[l+n-7:], x3)
			b[l+n+1] = '"'
			return b
		}
	}
	b = append(b, '"')
	i := 0
	for ; i+8 <= len(s); i += 8 {
		x := uint64(s[i]) | uint64(s[i+1])<<8 | uint64(s[i+2])<<16 | uint64(s[i+3])<<24 |
			uint64(s[i+4])<<32 | uint64(s[i+5])<<40 | uint64(s[i+6])<<48 | uint64(s[i+7])<<56
		if swarUnsafe(x) != 0 {
			return appendStringSlow(b, s)
		}
	}
	for ; i < len(s); i++ {
		if c := s[i]; c >= utf8.RuneSelf || !htmlSafe[c] {
			return appendStringSlow(b, s)
		}
	}
	b = append(b, s...)
	return append(b, '"')
}

const hexChars = "0123456789abcdef"

// appendStringSlow appends s with escaping, the opening quote is already in b.
func appendStringSlow(b []byte, s string) []byte {
	start := 0
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if htmlSafe[c] {
				i++
				continue
			}
			b = append(b, s[start:i]...)
			switch c {
			case '\\', '"':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				// Control characters other than \t, \n and \r, plus '<', '>'
				// and '&' which are unsafe to embed into HTML.
				b = append(b, '\\', 'u', '0', '0', hexChars[c>>4], hexChars[c&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			b = append(b, s[start:i]...)
			b = append(b, `\ufffd`...)
			i += size
			start = i
			continue
		}
		// U+2028 LINE SEPARATOR and U+2029 PARAGRAPH SEPARATOR are valid in
		// JSON strings but not in JavaScript ones, so escape them like
		// encoding/json does.
		if c == 0x2028 || c == 0x2029 {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', hexChars[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}

// htmlSafe holds true for ASCII characters which may be put into a JSON
// string as is, even when that JSON is embedded into HTML.
var htmlSafe = func() (t [utf8.RuneSelf]bool) {
	for c := 0x20; c < utf8.RuneSelf; c++ {
		switch c {
		case '"', '\\', '<', '>', '&':
		default:
			t[c] = true
		}
	}
	return t
}()

// AppendRaw appends an already encoded JSON value to b.
//
// A value with nothing in it is written as null. Raw newlines are dropped: generated code
// is used for protocols which delimit messages with '\n', and in valid JSON a
// raw newline may only be insignificant whitespace.
func AppendRaw(b, raw []byte) []byte {
	start := len(b)
	for len(raw) > 0 {
		n := bytes.IndexByte(raw, '\n')
		if n < 0 {
			return append(b, raw...)
		}
		b = append(b, raw[:n]...)
		raw = raw[n+1:]
	}
	if len(b) == start {
		// Nothing but newlines, or nothing at all.
		return append(b, "null"...)
	}
	return b
}

// AppendBool appends true or false to b.
func AppendBool(b []byte, v bool) []byte {
	if v {
		return append(b, "true"...)
	}
	return append(b, "false"...)
}

// AppendUint appends the decimal form of v to b.
func AppendUint(b []byte, v uint64) []byte {
	if v < 10 {
		return append(b, '0'+byte(v))
	}
	return strconv.AppendUint(b, v, 10)
}

// AppendInt appends the decimal form of v to b.
func AppendInt(b []byte, v int64) []byte {
	if v >= 0 && v < 10 {
		return append(b, '0'+byte(v))
	}
	return strconv.AppendInt(b, v, 10)
}

// AppendFloat64 appends v to b in the shortest form which round-trips, using
// the same format as easyjson ('g').
//
// NaN and infinities have no JSON representation and are written as null,
// which is also what JavaScript does.
func AppendFloat64(b []byte, v float64) []byte {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return append(b, "null"...)
	}
	return strconv.AppendFloat(b, v, 'g', -1, 64)
}

// AppendFloat32 is AppendFloat64 for float32 values.
func AppendFloat32(b []byte, v float32) []byte {
	if v != v || v > math.MaxFloat32 || v < -math.MaxFloat32 {
		return append(b, "null"...)
	}
	return strconv.AppendFloat(b, float64(v), 'g', -1, 32)
}

// loadString64 reads 8 bytes of s starting at i as a little-endian word.
func loadString64(s string, i int) uint64 {
	_ = s[i+7]
	return uint64(s[i]) | uint64(s[i+1])<<8 | uint64(s[i+2])<<16 | uint64(s[i+3])<<24 |
		uint64(s[i+4])<<32 | uint64(s[i+5])<<40 | uint64(s[i+6])<<48 | uint64(s[i+7])<<56
}

// loadString32 reads 4 bytes of s starting at i as a little-endian word.
func loadString32(s string, i int) uint32 {
	_ = s[i+3]
	return uint32(s[i]) | uint32(s[i+1])<<8 | uint32(s[i+2])<<16 | uint32(s[i+3])<<24
}

// load64 reads 8 bytes of b starting at i as a little-endian word.
func load64(b []byte, i int) uint64 {
	return binary.LittleEndian.Uint64(b[i:])
}
