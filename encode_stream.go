package protocol

import (
	"bufio"
	"encoding/binary"
	"io"
)

// StreamCommandEncoder ...
type StreamCommandEncoder interface {
	Encode(io.Writer, []byte) error
}

type JSONStreamCommandEncoder struct{}

func NewJSONStreamCommandEncoder() *JSONStreamCommandEncoder {
	return &JSONStreamCommandEncoder{}
}

func (d *JSONStreamCommandEncoder) Encode(stream io.Writer, cmd []byte) error {
	// TODO: reuse write buffer here.
	n, err := stream.Write(cmd)
	if err != nil {
		return err
	}
	if n != len(cmd) {
		return io.ErrShortWrite
	}
	n, err = stream.Write([]byte("\n"))
	if err != nil {
		return err
	}
	if n != 1 {
		return io.ErrShortWrite
	}
	return nil
}

type ProtobufStreamCommandEncoder struct {
	reader *bufio.Reader
}

func NewProtobufStreamCommandEncoder() *ProtobufStreamCommandEncoder {
	return &ProtobufStreamCommandEncoder{}
}

func (d *ProtobufStreamCommandEncoder) Encode(stream io.Writer, cmd []byte) error {
	// TODO: reuse write buffer here.
	// TODO: reuse slices for varint encoding here.
	bs := make([]byte, 8)
	n := binary.PutUvarint(bs, uint64(len(cmd)))
	written, err := stream.Write(bs[:n])
	if err != nil {
		return err
	}
	if written != n {
		return io.ErrShortWrite
	}
	n, err = stream.Write(cmd)
	if err != nil {
		return err
	}
	if n != len(cmd) {
		return io.ErrShortWrite
	}
	return nil
}
