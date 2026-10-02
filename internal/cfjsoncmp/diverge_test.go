package cfjsoncmp

import (
	"bytes"
	"regexp"
	"unicode/utf8"

	"github.com/centrifugal/protocol/cfjson"
)

// The decoders are expected to agree on every input, with the exceptions
// below. Each one is a deliberate difference, documented in README.md.

// knownDivergence reports whether the two decoders are expected to disagree
// on whether data is acceptable. It is always cfjson which is stricter:
//
//   - Data after the value. segmentio's Parse returns it to the caller, and
//     protocol ignored it. Now it is an error.
//   - Invalid UTF-8 in a string. segmentio replaces it by U+FFFD in strings
//     it decodes and keeps it in raw values. cfjson rejects the input: JSON
//     is UTF-8.
//   - Values of a recursive type (FilterNode) nested deeper than
//     cfjson.MaxDepth. segmentio recurses as deep as the input goes.
//   - A key which comes more than once in an object. segmentio decodes all
//     of them into the field, one after another.
//   - Integers which overflow 64 bits. segmentio misses most of them and
//     stores the wrapped around value: 21000000000000000000 decodes into a
//     uint64 as 2553255926290448384.
func knownDivergence(data []byte, oldErr, newErr error) bool {
	if oldErr != nil || newErr == nil {
		return false
	}
	if newErr == cfjson.ErrTrailingData || !utf8.Valid(data) {
		return true
	}
	if hasRepeatedKey(data) {
		return true
	}
	// Values of a recursive type nested deeper than cfjson allows.
	if bytes.Count(data, []byte("{")) > cfjson.MaxDepth {
		return true
	}
	digits := 0
	for _, c := range data {
		if c < '0' || c > '9' {
			digits = 0
			continue
		}
		if digits++; digits >= 19 {
			return true
		}
	}
	return false
}

// keyPattern matches what looks like an object key: a string followed by a
// colon.
var keyPattern = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"\s*:`)

// hasRepeatedKey reports whether some key occurs twice in data. It does not
// look at which object a key belongs to, so it says yes more often than a
// key is really repeated: good enough to explain why cfjson rejected an
// input, which is all it is used for.
func hasRepeatedKey(data []byte) bool {
	seen := map[string]bool{}
	for _, m := range keyPattern.FindAllSubmatch(data, -1) {
		key := string(m[1])
		// An escaped key may spell the same name differently.
		if bytes.IndexByte(m[1], '\\') >= 0 {
			if s, err := cfjson.UnmarshalString(m[0][:bytes.LastIndexByte(m[0], '"')+1]); err == nil {
				key = s
			}
		}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}

// validDiffers reports whether cfjson.Valid and segmentio's Valid are
// expected to disagree on data: only when it is not valid UTF-8, which
// segmentio does not check.
func validDiffers(data []byte, cfjsonValid, segmentioValid bool) bool {
	return segmentioValid && !cfjsonValid && !utf8.Valid(data)
}
