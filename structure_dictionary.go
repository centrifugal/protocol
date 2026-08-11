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

// StructureDictionaryID identifies the default structure dictionary.
var StructureDictionaryID = DictionaryID(StructureDictionary)

// StructureDictionary is a dictionary of protocol structure only - envelope,
// field names, and the JSON conventions most payloads happen to share. It
// contains no application data of any kind, so it is safe for every connection
// regardless of what it subscribes to.
//
// Unlike a channel dictionary it is not learned from traffic, so it contains no
// application data and needs no per-channel disclosure decision. It is sent once
// per connection and cached by clients that support it, keyed by the id above.
//
// It exists for connections a learned dictionary can never reach: those on
// channels nobody opted in, and quiet ones that never carry enough traffic to
// earn one. Measured on three unrelated applications it takes them from roughly
// 1.12-1.28x to 1.29-1.46x. That is modest, and deliberately so - the value is
// that it costs nothing to transfer and works from the first frame.
//
// Size is chosen from where the ratio stops improving: most of the gain is in
// the first 256 bytes, and past ~1 KB there is nothing left an SDK can honestly
// know without guessing at someone's payload schema.
var StructureDictionary = []byte(
	// The push envelope, which every frame carries, and the reply envelope for
	// command results. Ordered so the most repeated fragments sit last: DEFLATE
	// encodes nearer matches in fewer bits, and the end of the window is nearest.
	`"subscribe":{"recoverable":"connect":{"version":"","ttl":,"ping":,"session":"",` +
		`"unsubscribe":{"code":,"reason":"","join":{"leave":{"presence":{"history":{` +
		`"error":{"temporary":true,"method":"rpc":{"message":{"state":{"dictionary":{` +
		`"epoch":"","recovered":,"positioned":,"was_recovering":,"expires":,"delta":true,` +
		`"info":{"user":"","client":"","tags":{"time":,"id":,"offset":,` +
		`{"push":{"channel":"","pub":{"data":{`)
