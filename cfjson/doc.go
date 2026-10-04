// Package cfjson is the runtime for code produced by the cfjson generator (see
// cmd/cfjson): a small set of append-style encoding helpers and index-based
// decoding helpers the generated methods are built from.
//
// It is not a general-purpose package: it is built for the needs of the
// Centrifugal ecosystem (Centrifugo, centrifuge, their SDKs and tools) and
// changes as they need it to, in any release. Do not depend on it outside of
// that ecosystem.
//
// The package depends on the standard library only and knows nothing about
// the types it is used with, so it can be moved to a module of its own by
// changing a single import path (the generator's -runtime flag).
//
// # Encoding
//
// Generated encoders have the shape
//
//	func (m *T) AppendJSON(b []byte) []byte
//
// and append the JSON encoding of m to b. There is no writer object and no
// error: every supported type always has a JSON encoding. The output is valid
// JSON, and byte-compatible with what easyjson produces for the same struct
// tags: `omitempty` semantics, `null` for nil maps, slices and pointers which
// are not omitted, HTML-safe string escaping, invalid UTF-8 replaced by U+FFFD.
//
// # Decoding
//
// Generated decoders have the shape
//
//	func (m *T) DecodeJSON(b []byte, i int, f Flags) int
//
// They decode the value starting at b[i] into m and return the index right
// after it. A negative result reports an error and encodes its offset, turn it
// into an error value with Error. The top level call looks like
//
//	if n := m.DecodeJSON(data, cfjson.SkipSpace(data, 0), 0); n < 0 {
//		return cfjson.Error(data, n)
//	}
//
// Passing the input as a slice plus an index, and reporting errors in the
// returned index, keeps the whole decoder state in registers – there is no
// lexer object to load from and store to on every token.
//
// The input must be valid JSON as RFC 8259 defines it, valid UTF-8 included.
// How values get into Go types follows encoding/json: unknown fields are
// skipped, `null` leaves scalars untouched and resets pointers, maps and
// slices, a key must match the name of a field exactly and may come once,
// numbers which do not fit the destination are an error.
package cfjson
