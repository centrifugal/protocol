package cfjson

import "unsafe"

// aliasString returns a string which shares its memory with b, for decoding
// with the ZeroCopy flag.
//
// This is the only use of unsafe in the package, and it is only reachable
// when a caller asks for ZeroCopy. What makes it safe is a promise that
// caller gives: strings are immutable, so b must not be modified for as long
// as the string is in use. Without the flag nothing decoded shares memory
// with the input.
func aliasString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b)) //nolint:gosec // G103: see the comment above.
}
