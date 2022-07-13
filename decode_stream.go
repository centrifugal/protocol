package protocol

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/segmentio/encoding/json"
)

// StreamCommandDecoder is EXPERIMENTAL.
type StreamCommandDecoder interface {
	Decode() (*Command, []byte, error)
}

// JSONStreamCommandDecoder is EXPERIMENTAL.
type JSONStreamCommandDecoder struct {
	reader *bufio.Reader
}

// NewJSONStreamCommandDecoder is EXPERIMENTAL.
func NewJSONStreamCommandDecoder(reader io.Reader) *JSONStreamCommandDecoder {
	return &JSONStreamCommandDecoder{reader: bufio.NewReader(reader)}
}

func (d *JSONStreamCommandDecoder) Decode() (*Command, []byte, error) {
	cmdBytes, err := d.reader.ReadBytes('\n')
	if err != nil {
		return nil, nil, err
	}
	var c Command
	_, err = json.Parse(cmdBytes, &c, json.ZeroCopy)
	if err != nil {
		return nil, nil, err
	}
	return &c, cmdBytes, nil
}

// ProtobufStreamCommandDecoder is EXPERIMENTAL.
type ProtobufStreamCommandDecoder struct {
	reader *bufio.Reader
}

// NewProtobufStreamCommandDecoder is EXPERIMENTAL.
func NewProtobufStreamCommandDecoder(reader io.Reader) *ProtobufStreamCommandDecoder {
	return &ProtobufStreamCommandDecoder{reader: bufio.NewReader(reader)}
}

func (d *ProtobufStreamCommandDecoder) Decode() (*Command, []byte, error) {
	msgLength, err := binary.ReadUvarint(d.reader)
	if err != nil {
		return nil, nil, err
	}
	cmdBytes := make([]byte, msgLength)
	n, err := d.reader.Read(cmdBytes)
	if err != nil {
		return nil, nil, err
	}
	if uint64(n) != msgLength {
		return nil, nil, io.ErrShortBuffer
	}
	var c Command
	err = c.UnmarshalVT(cmdBytes)
	if err != nil {
		return nil, nil, err
	}
	return &c, cmdBytes, nil
}

//
//// StreamReplyDecoder ...
//type StreamReplyDecoder interface {
//	Decode() (*Reply, error)
//}
//
//type JSONStreamReplyDecoder struct {
//	reader *bufio.Reader
//}
//
//func NewJSONStreamReplyDecoder(reader io.Reader) *JSONStreamReplyDecoder {
//	return &JSONStreamReplyDecoder{reader: bufio.NewReader(reader)}
//}
//
//func (d *JSONStreamReplyDecoder) Decode() (*Reply, error) {
//	cmdBytes, err := d.reader.ReadBytes('\n')
//	if err != nil {
//		return nil, err
//	}
//	var c Reply
//	_, err = json.Parse(cmdBytes, &c, json.ZeroCopy)
//	if err != nil {
//		return nil, err
//	}
//	return &c, nil
//}
//
//type ProtobufStreamReplyDecoder struct {
//	reader *bufio.Reader
//}
//
//func NewProtobufStreamReplyDecoder(reader io.Reader) *ProtobufStreamReplyDecoder {
//	return &ProtobufStreamReplyDecoder{reader: bufio.NewReader(reader)}
//}
//
//func (d *ProtobufStreamReplyDecoder) Decode() (*Reply, error) {
//	msgLength, err := binary.ReadUvarint(d.reader)
//	if err != nil {
//		return nil, err
//	}
//	cmdBytes := make([]byte, msgLength)
//	n, err := d.reader.Read(cmdBytes)
//	if err != nil {
//		return nil, err
//	}
//	if uint64(n) != msgLength {
//		return nil, io.ErrShortBuffer
//	}
//	var c Reply
//	err = c.UnmarshalVT(cmdBytes)
//	if err != nil {
//		return nil, err
//	}
//	return &c, nil
//}
