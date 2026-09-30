package cfjson

import (
	"bytes"
	"errors"
)

// Appender is implemented by types with a generated encoder.
type Appender interface {
	// AppendJSON appends the JSON encoding of the value to b.
	AppendJSON(b []byte) []byte
}

// Decoder is implemented by types with a generated decoder.
type Decoder interface {
	// DecodeJSON decodes the JSON value at b[i] into the receiver and returns
	// the index after the value, or a negative number on error, see Error.
	DecodeJSON(b []byte, i int, f Flags) int
}

// ErrTrailingData is returned by Unmarshal and UnmarshalString when the input
// goes on after the value.
var ErrTrailingData = errors.New("cfjson: unexpected data after the JSON value")

// Unmarshal decodes data, which must be a single JSON value optionally
// surrounded by whitespace, into v.
//
// Fields of v which are not present in data are left as they are, so pass a
// new value unless merging is what is needed.
func Unmarshal(data []byte, v Decoder, f Flags) error {
	n := v.DecodeJSON(data, SkipSpace(data, 0), Prescan(data, f))
	if n < 0 {
		return Error(data, n)
	}
	if SkipSpace(data, n) != len(data) {
		return ErrTrailingData
	}
	return nil
}

// UnmarshalString returns the string which data, a single JSON string
// optionally surrounded by whitespace, stands for. It is the reverse of
// AppendString. The result is a copy, it does not point into data.
//
// Anything but a string is an error, null included.
func UnmarshalString(data []byte) (string, error) {
	i := SkipSpace(data, 0)
	if i >= len(data) || data[i] != '"' {
		return "", &DecodeError{Offset: i, Syntax: !Valid(data)}
	}
	var s string
	n := String(data, i, 0, &s)
	if n < 0 {
		return "", &DecodeError{Offset: ^n, Syntax: true}
	}
	if SkipSpace(data, n) != len(data) {
		return "", ErrTrailingData
	}
	return s, nil
}

// Marshaler is implemented by types which produce their JSON themselves. It
// is the interface of the same name in encoding/json.
type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

// Unmarshaler is implemented by types which read their JSON themselves. It is
// the interface of the same name in encoding/json: UnmarshalJSON is given a
// valid JSON value and must copy what it keeps of it.
type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

// AppendMarshaler appends the JSON a type produces for itself to b.
// Generated code calls it for fields of types which have a MarshalJSON
// method.
//
// Appending cannot fail, so if MarshalJSON returns an error, or something
// which is not valid JSON, null is written in place of the value. Newlines
// are dropped, as AppendRaw does.
func AppendMarshaler(b []byte, m Marshaler) []byte {
	data, err := m.MarshalJSON()
	if err != nil || !Valid(data) {
		return append(b, "null"...)
	}
	// Like AppendRaw: what is written has no newlines in it. In valid JSON
	// they are whitespace between tokens, which may go.
	for {
		n := bytes.IndexByte(data, '\n')
		if n < 0 {
			return append(b, data...)
		}
		b = append(b, data[:n]...)
		data = data[n+1:]
	}
}

// DecodeUnmarshaler hands the JSON value at b[i] to u and returns the index
// after it. Generated code calls it for fields of types which have an
// UnmarshalJSON method. An error returned by u is reported at i.
func DecodeUnmarshaler(b []byte, i int, f Flags, u Unmarshaler) int {
	j := Skip(b, i, f)
	if j < 0 {
		return j
	}
	if u.UnmarshalJSON(b[i:j]) != nil {
		return ^i
	}
	return j
}
