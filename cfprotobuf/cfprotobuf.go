// Package cfprotobuf is the runtime for code produced by the cfprotobuf
// generator (see cmd/cfprotobuf): the few primitives of the Protobuf wire
// format which generated marshalers and unmarshalers are built from.
//
// It is not a general-purpose package: it is built for the needs of the
// Centrifugal ecosystem (Centrifugo, centrifuge, their SDKs and tools) and
// changes as they need it to, in any release. Do not depend on it outside of
// that ecosystem.
//
// The generator writes, for structs which protoc-gen-go produced, the methods
// MarshalCF, MarshalToCF, MarshalToSizedBufferCF, SizeCF and UnmarshalCF.
// They do what the methods github.com/planetscale/vtprotobuf writes do, with
// the same output, and can be given the names of those with the -suffix
// option. The messages stay regular Protobuf messages: nothing here replaces
// google.golang.org/protobuf.
//
// The package depends on the standard library only.
//
// Decoding is meant for untrusted input. It follows the Protobuf encoding
// specification (https://protobuf.dev/programming-guides/encoding/) the way
// google.golang.org/protobuf does: a varint which does not fit 64 bits, a
// field number outside of 1..2^29-1, a length which goes past the end of the
// input and a proto3 string which is not valid UTF-8 are errors. A field a
// message does not have, or has with another wire type, is skipped and kept
// with the message.
package cfprotobuf

import (
	"encoding/binary"
	"errors"
	"math"
	"math/bits"
	"unicode/utf8"
)

// Wire types of the Protobuf encoding.
const (
	WireVarint     = 0
	WireFixed64    = 1
	WireBytes      = 2
	WireStartGroup = 3
	WireEndGroup   = 4
	WireFixed32    = 5
)

const (
	// maxFieldNumber is the largest field number the specification allows.
	maxFieldNumber = 1<<29 - 1

	// maxGroupDepth is how many groups may be open inside of a group which
	// is skipped. It is the limit of google.golang.org/protobuf, so that
	// what is acceptable input stays the same as there.
	maxGroupDepth = 10000

	// MaxDepth is how deep messages of recursive types may be nested, more is
	// an error. A recursive type is a tree of some kind, a filter expression
	// for example, and a tree written by a person or by a program is nowhere
	// near this deep. google.golang.org/protobuf allows 10000 levels of
	// nesting.
	MaxDepth = 1024
)

var (
	// ErrTruncated is returned when the input ends in the middle of a value,
	// or a length points past its end.
	ErrTruncated = errors.New("proto: unexpected end of input")
	// ErrIntOverflow is returned for a varint which does not fit 64 bits.
	ErrIntOverflow = errors.New("proto: integer overflow")
	// ErrInvalidTag is returned for a field number outside of 1..2^29-1, and
	// for an end of a group which has not been started.
	ErrInvalidTag = errors.New("proto: invalid field tag")
	// ErrInvalidUTF8 is returned for a string field which is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("proto: string field contains invalid UTF-8")
	// ErrTooDeep is returned for messages of a recursive type nested deeper
	// than MaxDepth.
	ErrTooDeep = errors.New("proto: messages are nested too deep")
)

// SizeOfVarint returns the number of bytes the varint encoding of x takes.
func SizeOfVarint(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}

// Zigzag returns the zigzag encoding of x, which is what sint32 and sint64
// fields are written as: small negative numbers become small positive ones.
func Zigzag(x int64) uint64 {
	return uint64(x<<1) ^ uint64(x>>63) //nolint:gosec // G115: a bit pattern is converted, not a number.
}

// Unzigzag is the reverse of Zigzag.
func Unzigzag(x uint64) int64 {
	return int64(x>>1) ^ -int64(x&1) //nolint:gosec // G115: a bit pattern is converted, not a number.
}

// PutVarint writes the varint encoding of v so that it ends right before
// b[end], and returns the index it starts at. Generated marshalers fill a
// buffer from its end, which is what lets them write the length of a nested
// message after the message itself.
func PutVarint(b []byte, end int, v uint64) int {
	if v < 0x80 {
		end--
		b[end] = byte(v)
		return end
	}
	start := end - SizeOfVarint(v)
	i := start
	for v >= 0x80 {
		b[i] = byte(v) | 0x80
		v >>= 7
		i++
	}
	b[i] = byte(v)
	return start
}

// Integer is the constraint of the Go types varint fields have.
type Integer interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64
}

// SizeOfInt returns the number of bytes the varint encoding of v takes. As
// the wire format has it, a negative number is sign-extended to 64 bits and
// takes ten bytes.
func SizeOfInt[T Integer](v T) int {
	return SizeOfVarint(uint64(v)) //nolint:gosec // G115: the wire format defines this conversion.
}

// PutInt is PutVarint for the Go types varint fields have.
func PutInt[T Integer](b []byte, end int, v T) int {
	return PutVarint(b, end, uint64(v)) //nolint:gosec // G115: the wire format defines this conversion.
}

// Int converts a decoded varint to the Go type of its field, dropping the
// bits which do not fit, as the wire format has it.
func Int[T Integer](v uint64) T {
	return T(v) //nolint:gosec // G115: the wire format defines this conversion.
}

// SizeOfLen returns the number of bytes the length prefix of a value of n
// bytes takes.
func SizeOfLen(n int) int {
	return SizeOfVarint(uint64(n)) //nolint:gosec // G115: a length is never negative.
}

// SizeOfMessage returns the number of bytes a nested message of n bytes
// takes together with its length prefix.
func SizeOfMessage(n int) int {
	return n + SizeOfLen(n)
}

// PutLen is PutVarint for the length of a value.
func PutLen(b []byte, end, n int) int {
	return PutVarint(b, end, uint64(n)) //nolint:gosec // G115: a length is never negative.
}

// AppendUnknown appends a field a message does not have to the fields it
// keeps: the tag, encoded in the shortest form whatever form it came in, and
// the bytes of the value as they are. This is what
// google.golang.org/protobuf keeps.
func AppendUnknown(dst []byte, tag uint64, value []byte) []byte {
	for tag >= 0x80 {
		dst = append(dst, byte(tag)|0x80)
		tag >>= 7
	}
	dst = append(dst, byte(tag))
	return append(dst, value...)
}

// Varint decodes the varint at b[i] and returns it with the index after it.
// A negative index reports an error, see Error.
func Varint(b []byte, i int) (uint64, int) {
	if i < len(b) {
		if c := b[i]; c < 0x80 {
			return uint64(c), i + 1
		}
	}
	return varintSlow(b, i)
}

// The negative indexes decoding functions report errors with.
const (
	errTruncated = -1 - iota
	errOverflow
	errTag
	errDepth
)

func varintSlow(b []byte, i int) (uint64, int) {
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if i >= len(b) {
			return 0, errTruncated
		}
		c := b[i]
		i++
		if shift == 63 && c > 1 {
			// The tenth byte has room for one bit only.
			return 0, errOverflow
		}
		v |= uint64(c&0x7F) << shift
		if c < 0x80 {
			return v, i
		}
	}
	return 0, errOverflow
}

// Error converts a negative index returned by a decoding function into an
// error.
func Error(i int) error {
	switch i {
	case errOverflow:
		return ErrIntOverflow
	case errTag:
		return ErrInvalidTag
	case errDepth:
		return ErrTooDeep
	}
	return ErrTruncated
}

// Length decodes the length prefix at b[i] and returns the index of the first
// byte after the prefix and the index after the last byte of the value, which
// is within b. A negative first index reports an error, see Error.
func Length(b []byte, i int) (start, end int) {
	n, i := Varint(b, i)
	if i < 0 {
		return i, 0
	}
	if n > uint64(len(b)-i) { //nolint:gosec // G115: i <= len(b), so the difference is not negative.
		return errTruncated, 0
	}
	return i, i + int(n) //nolint:gosec // G115: n <= len(b), checked above.
}

const highBits = 0x8080808080808080

// ShortASCII reports whether b is a string of up to 16 bytes of nothing but
// ASCII, which is what most strings of most messages are: names, identifiers,
// channels. It is small enough to be inlined, which saves the call
// ValidString takes. A false result says nothing, ValidString has to be asked
// then.
func ShortASCII(b []byte) bool {
	n := len(b)
	if n >= 8 {
		// 8 to 16 bytes are covered by two words which may overlap.
		if n <= 16 {
			return (binary.LittleEndian.Uint64(b)|binary.LittleEndian.Uint64(b[n-8:]))&highBits == 0
		}
		return false
	}
	if n >= 4 {
		return (binary.LittleEndian.Uint32(b)|binary.LittleEndian.Uint32(b[n-4:]))&0x80808080 == 0
	}
	// Up to three bytes: the first, the middle and the last one.
	return n == 0 || (b[0]|b[n>>1]|b[n-1]) < utf8.RuneSelf
}

// ValidString reports whether the bytes of a proto3 string field are valid
// UTF-8, which the specification requires them to be.
func ValidString(b []byte) bool {
	// Most strings are plain ASCII, which takes a look at the high bit of
	// every byte, eight bytes at a time.
	n := len(b)
	if n < 8 {
		var all byte
		for _, c := range b {
			all |= c
		}
		return all < utf8.RuneSelf || utf8.Valid(b)
	}
	// The last word overlaps the one before it unless n is a multiple of 8.
	all := binary.LittleEndian.Uint64(b[n-8:])
	p := b
	// Four words at a time: a long string is a token more often than not,
	// and one load per iteration is several times slower for it.
	for len(p) >= 32 {
		all |= binary.LittleEndian.Uint64(p) | binary.LittleEndian.Uint64(p[8:]) |
			binary.LittleEndian.Uint64(p[16:]) | binary.LittleEndian.Uint64(p[24:])
		p = p[32:]
	}
	for len(p) >= 8 {
		all |= binary.LittleEndian.Uint64(p)
		p = p[8:]
	}
	return all&highBits == 0 || utf8.Valid(b)
}

// Skip returns the index after the field value at b[i], where tag is the tag
// which came before it. It is used for fields a message type does not have. A
// negative index reports an error, see Error.
//
// Groups, which proto3 does not have but a proto2 sender may use, are skipped
// together with everything inside of them. Skip does not recurse.
func Skip(b []byte, i int, tag uint64) int {
	if tag>>3 == 0 || tag>>3 > maxFieldNumber {
		return errTag
	}
	// groups holds the field numbers of the groups which are open.
	var groups []uint64
	for {
		switch tag & 7 {
		case WireVarint:
			_, i = Varint(b, i)
		case WireFixed64:
			i += 8
		case WireBytes:
			start, end := Length(b, i)
			if start < 0 {
				return start
			}
			i = end
		case WireStartGroup:
			if len(groups) > maxGroupDepth {
				return errDepth
			}
			groups = append(groups, tag>>3)
		case WireEndGroup:
			if len(groups) == 0 || groups[len(groups)-1] != tag>>3 {
				return errTag
			}
			groups = groups[:len(groups)-1]
		case WireFixed32:
			i += 4
		default:
			return errTag
		}
		if i < 0 {
			return i
		}
		if i > len(b) {
			return errTruncated
		}
		if len(groups) == 0 {
			return i
		}
		// Inside of a group: the next field.
		if tag, i = Varint(b, i); i < 0 {
			return i
		}
		// google.golang.org/protobuf holds the fields of a group it skips to
		// the range of int32 only, not to maxFieldNumber, and what is
		// acceptable input is kept the same as there.
		if tag>>3 == 0 || tag>>3 > math.MaxInt32 {
			return errTag
		}
	}
}
