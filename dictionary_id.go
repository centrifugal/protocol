package protocol

import (
	"crypto/sha256"
	"encoding/base64"
)

// DictionaryID is the identity of a dictionary: SHA-256 of its content, first
// 12 bytes, base64url without padding.
//
// This exact derivation is required rather than conventional. Clients verify it
// and discard a dictionary whose bytes do not hash to the id they were given, so
// anything identifying dictionaries another way - a version counter, a random
// token - has them silently rejected while the serving side sees nothing wrong.
//
// Only the serving side computes an id, so this is here rather than with a
// particular server: centrifuge exposes an interface for supplying dictionaries,
// which makes the set of callers open. An implementation that had to re-derive
// this from an SDK's source would be one transcription error away from a failure
// that reports itself nowhere.
//
// Deriving the id from the bytes is what makes caching one safe at both ends. A
// client advertising an id and a server holding that id necessarily have byte
// identical dictionaries, so there is no way to decode against content the
// encoder did not use. It also lets a client check bytes it loaded from its own
// storage, which an opaque token would not: a corrupted cache entry is caught
// rather than used to misread every frame that follows. Changing a dictionary
// changes its id automatically, which turns an upgrade into an ordinary cache
// miss rather than corruption.
func DictionaryID(dict []byte) string {
	sum := sha256.Sum256(dict)
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}
