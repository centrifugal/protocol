package protocol

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/segmentio/encoding/json"
)

// StreamCommandDecoder ...
type StreamCommandDecoder interface {
	Decode() (*Command, error)
}

type JSONStreamCommandDecoder struct {
	reader *bufio.Reader
}

func NewJSONStreamCommandDecoder(reader io.Reader) *JSONStreamCommandDecoder {
	return &JSONStreamCommandDecoder{reader: bufio.NewReader(reader)}
}

func (d *JSONStreamCommandDecoder) Decode() (*Command, error) {
	cmdBytes, err := d.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var c Command
	_, err = json.Parse(cmdBytes, &c, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

type ProtobufStreamCommandDecoder struct {
	reader *bufio.Reader
}

func NewProtobufStreamCommandDecoder(reader io.Reader) *ProtobufStreamCommandDecoder {
	return &ProtobufStreamCommandDecoder{reader: bufio.NewReader(reader)}
}

func (d *ProtobufStreamCommandDecoder) Decode() (*Command, error) {
	msgLength, err := binary.ReadUvarint(d.reader)
	if err != nil {
		return nil, err
	}
	cmdBytes := make([]byte, msgLength)
	_, err = d.reader.Read(cmdBytes)
	if err != nil {
		return nil, err
	}
	var c Command
	err = c.UnmarshalVT(cmdBytes)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
