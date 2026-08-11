package protocol

import (
	"crypto/sha256"
	"encoding/base64"
)

// DictionaryID is the identity of a dictionary: SHA-256 of its content, first
// 12 bytes, base64url without padding.
//
// Only the serving side computes one - a client treats an id as opaque and hands
// it back - but the derivation is still required rather than conventional, for a
// reason that has nothing to do with decoding.
//
// A client stores this value and advertises it on every later connect,
// indefinitely, which makes it cookie-shaped. An id chosen freely could be
// unique per client, and would then be a tracking identifier the client keeps
// alive on the server's behalf. A hash of content cannot be: it is identical for
// everyone holding those bytes, and a client can check that by hashing what it
// holds - which centrifuge-js does before reusing a dictionary it persisted.
//
// It lives here rather than with a particular server because centrifuge exposes
// an interface for supplying dictionaries, so the set of callers is open. An
// implementation re-deriving this from an SDK is one transcription error away
// from a client that quietly stops caching.
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
