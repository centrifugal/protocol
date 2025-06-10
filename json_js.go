//go:build js

package protocol

import (
	"io"

	"encoding/json"
)

type JSONDecoder = json.Decoder

func NewJSONDecoder(r io.Reader) *JSONDecoder { return json.NewDecoder(r) }

const JSONZeroCopy = 0 // NOT USED in JS.

func ParseJSON(b []byte, x interface{}, _ uint32) ([]byte, error) {
	err := json.Unmarshal(b, x)
	return b, err
}

func ValidJSON(data []byte) bool {
	return json.Valid(data)
}
