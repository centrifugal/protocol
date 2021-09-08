package protocol

import (
	"bytes"
	"errors"
	"sync"

	"github.com/segmentio/encoding/json"
)

var bufferPool = sync.Pool{
	// New is called when a new instance is needed
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

// Raw type used by Centrifuge protocol as a type for fields in structs which
// value we want to stay untouched. For example custom application specific JSON
// payload data in published message. This is very similar to json.RawMessage
// type but have some extra methods to fit gogo/protobuf custom type interface.
type Raw []byte

// MarshalJSON returns *r as the JSON encoding of r.
func (r Raw) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	buf := getBuffer()
	err := json.Compact(buf, r)
	if err != nil {
		return nil, err
	}
	res := append([]byte(nil), buf.Bytes()...)
	putBuffer(buf)
	return res, nil
}

// UnmarshalJSON sets *r to a copy of data.
func (r *Raw) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("unmarshal Raw: UnmarshalJSON on nil pointer")
	}
	*r = append((*r)[0:0], data...)
	return nil
}
