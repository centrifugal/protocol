//go:build !js

package protocol

import (
	"io"

	"github.com/segmentio/encoding/json"
)

type JSONDecoder = json.Decoder

func NewJSONDecoder(r io.Reader) *JSONDecoder { return json.NewDecoder(r) }

const JSONZeroCopy = uint32(json.ZeroCopy)

func ParseJSON(b []byte, x interface{}, flags uint32) ([]byte, error) {
	return json.Parse(b, x, json.ParseFlags(flags))
}

func ValidJSON(data []byte) bool {
	return json.Valid(data)
}
