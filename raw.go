package protocol

import (
	"bytes"
	"errors"
)

// Raw is the type used by the Centrifugal protocol for fields whose value we
// want to stay untouched – for example the application-specific JSON payload of
// a published message. It's very similar to json.RawMessage, but its encoding
// also accounts for the `\n` delimiter used to put several messages into a
// single transport frame.
//
// Generated code uses Raw instead of []byte for all bytes fields, see
// generate.sh.
type Raw []byte

// MarshalJSON returns r as the JSON encoding of r.
//
// Raw payloads are passed through as is, with one exception: raw newlines are
// stripped, since a `\n` delimits messages inside a transport frame. Valid JSON
// only contains raw newlines as formatting whitespace between tokens – newlines
// within JSON strings are escaped – so removing them doesn't change the payload
// a subscriber decodes.
//
// The returned slice may alias r, so it must not be modified.
func (r Raw) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	if !bytes.Contains(r, []byte("\n")) {
		return r, nil
	}
	return bytes.ReplaceAll(r, []byte("\n"), []byte("")), nil
}

// UnmarshalJSON sets *r to a copy of data.
func (r *Raw) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("unmarshal Raw: UnmarshalJSON on nil pointer")
	}
	*r = append((*r)[0:0], data...)
	return nil
}
