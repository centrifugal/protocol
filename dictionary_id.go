package protocol

import (
	"crypto/sha256"
	"encoding/base64"
)

// DictionaryID is the identity of a dictionary: a hash of its content.
//
// Deriving the id from the bytes is what makes client-side caching safe. A
// client advertising an id and a server holding that id necessarily have byte
// identical dictionaries, so there is no way to decode against content the
// encoder did not use. Changing a dictionary changes its id automatically,
// which turns an upgrade into an ordinary cache miss rather than corruption.
func DictionaryID(dict []byte) string {
	sum := sha256.Sum256(dict)
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}
