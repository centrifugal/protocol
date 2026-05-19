package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/segmentio/encoding/json"
)

// CommandDecoder ...
type CommandDecoder interface {
	Reset([]byte) error
	Decode() (*Command, error)
}

// JSONCommandDecoder ...
type JSONCommandDecoder struct {
	data   []byte
	offset int
}

// NewJSONCommandDecoder ...
func NewJSONCommandDecoder(data []byte) *JSONCommandDecoder {
	return &JSONCommandDecoder{data: data}
}

// Reset ...
func (d *JSONCommandDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode ...
func (d *JSONCommandDecoder) Decode() (*Command, error) {
	if d.offset >= len(d.data) {
		return nil, io.ErrUnexpectedEOF
	}
	rest := d.data[d.offset:]
	var msg []byte
	if idx := bytes.IndexByte(rest, '\n'); idx >= 0 {
		msg = rest[:idx]
		d.offset += idx + 1
	} else {
		msg = rest
		d.offset = len(d.data)
	}
	var c Command
	if _, err := json.Parse(msg, &c, json.ZeroCopy); err != nil {
		return nil, err
	}
	if d.offset >= len(d.data) {
		return &c, io.EOF
	}
	return &c, nil
}

// ProtobufCommandDecoder ...
type ProtobufCommandDecoder struct {
	data   []byte
	offset int
}

// NewProtobufCommandDecoder ...
func NewProtobufCommandDecoder(data []byte) *ProtobufCommandDecoder {
	return &ProtobufCommandDecoder{
		data: data,
	}
}

// Reset ...
func (d *ProtobufCommandDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode ...
func (d *ProtobufCommandDecoder) Decode() (*Command, error) {
	if d.offset < len(d.data) {
		var c Command
		l, n := binary.Uvarint(d.data[d.offset:])
		if n <= 0 {
			return nil, io.EOF
		}
		from := d.offset + n
		to := d.offset + n + int(l)
		if from <= to && to <= len(d.data) {
			cmdBytes := d.data[from:to]
			err := c.UnmarshalVT(cmdBytes) // Check whether UnmarshalVTUnsafe here is OK.
			if err != nil {
				return nil, err
			}
			d.offset = to
			if d.offset == len(d.data) {
				err = io.EOF
			}
			return &c, err
		} else {
			return nil, io.ErrShortBuffer
		}
	}
	return nil, io.EOF
}

// ReplyDecoder ...
type ReplyDecoder interface {
	Reset([]byte) error
	Decode() (*Reply, error)
}

var _ ReplyDecoder = NewJSONReplyDecoder(nil)

// JSONReplyDecoder ...
type JSONReplyDecoder struct {
	decoder *json.Decoder
}

// NewJSONReplyDecoder ...
func NewJSONReplyDecoder(data []byte) *JSONReplyDecoder {
	return &JSONReplyDecoder{
		decoder: json.NewDecoder(bytes.NewReader(data)),
	}
}

// Reset ...
func (d *JSONReplyDecoder) Reset(data []byte) error {
	d.decoder = json.NewDecoder(bytes.NewReader(data))
	return nil
}

// Decode ...
func (d *JSONReplyDecoder) Decode() (*Reply, error) {
	var c Reply
	err := d.decoder.Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

var _ ReplyDecoder = NewProtobufReplyDecoder(nil)

// ProtobufReplyDecoder ...
type ProtobufReplyDecoder struct {
	data   []byte
	offset int
}

// NewProtobufReplyDecoder ...
func NewProtobufReplyDecoder(data []byte) *ProtobufReplyDecoder {
	return &ProtobufReplyDecoder{
		data: data,
	}
}

// Reset ...
func (d *ProtobufReplyDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode ...
func (d *ProtobufReplyDecoder) Decode() (*Reply, error) {
	if d.offset < len(d.data) {
		var c Reply
		l, n := binary.Uvarint(d.data[d.offset:])
		replyBytes := d.data[d.offset+n : d.offset+n+int(l)]
		err := c.UnmarshalVT(replyBytes)
		if err != nil {
			return nil, err
		}
		d.offset = d.offset + n + int(l)
		return &c, nil
	}
	return nil, io.EOF
}
