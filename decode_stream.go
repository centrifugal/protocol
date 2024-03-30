package protocol

import (
	"bufio"
	"encoding/binary"
	"io"
	"sync"

	"github.com/segmentio/encoding/json"
)

var (
	streamJsonCommandDecoderPool     sync.Pool
	streamProtobufCommandDecoderPool sync.Pool
)

func GetStreamCommandDecoder(protoType Type, reader io.Reader) StreamCommandDecoder {
	if protoType == TypeJSON {
		e := streamJsonCommandDecoderPool.Get()
		if e == nil {
			return NewJSONStreamCommandDecoder(reader)
		}
		commandDecoder := e.(*JSONStreamCommandDecoder)
		commandDecoder.Reset(reader)
		return commandDecoder
	}
	e := streamProtobufCommandDecoderPool.Get()
	if e == nil {
		return NewProtobufStreamCommandDecoder(reader)
	}
	commandDecoder := e.(*ProtobufStreamCommandDecoder)
	commandDecoder.Reset(reader)
	return commandDecoder
}

func PutStreamCommandDecoder(protoType Type, e StreamCommandDecoder) {
	e.Reset(nil)
	if protoType == TypeJSON {
		streamJsonCommandDecoderPool.Put(e)
		return
	}
	streamProtobufCommandDecoderPool.Put(e)
}

type StreamCommandDecoder interface {
	Decode() (*Command, int, error)
	Reset(reader io.Reader)
}

type JSONStreamCommandDecoder struct {
	reader *bufio.Reader
}

func NewJSONStreamCommandDecoder(reader io.Reader) *JSONStreamCommandDecoder {
	return &JSONStreamCommandDecoder{reader: bufio.NewReader(reader)}
}

func (d *JSONStreamCommandDecoder) Decode() (*Command, int, error) {
	cmdBytes, err := d.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF && len(cmdBytes) > 0 {
			var c Command
			_, parseErr := json.Parse(cmdBytes, &c, 0)
			if parseErr != nil {
				return nil, 0, parseErr
			}
			return &c, len(cmdBytes), err
		}
		return nil, 0, err
	}
	var c Command
	_, err = json.Parse(cmdBytes, &c, 0)
	if err != nil {
		return nil, 0, err
	}
	return &c, len(cmdBytes), nil
}

func (d *JSONStreamCommandDecoder) Reset(reader io.Reader) {
	d.reader.Reset(reader)
}

type ProtobufStreamCommandDecoder struct {
	reader *bufio.Reader
}

func NewProtobufStreamCommandDecoder(reader io.Reader) *ProtobufStreamCommandDecoder {
	return &ProtobufStreamCommandDecoder{reader: bufio.NewReader(reader)}
}

func (d *ProtobufStreamCommandDecoder) Decode() (*Command, int, error) {
	msgLength, err := binary.ReadUvarint(d.reader)
	if err != nil {
		return nil, 0, err
	}
	bb := getByteBuffer(int(msgLength))
	defer putByteBuffer(bb)

	n, err := io.ReadFull(d.reader, bb.B[:int(msgLength)])
	if err != nil {
		return nil, 0, err
	}
	if uint64(n) != msgLength {
		return nil, 0, io.ErrShortBuffer
	}
	var c Command
	err = c.UnmarshalVT(bb.B[:int(msgLength)])
	if err != nil {
		return nil, 0, err
	}
	return &c, int(msgLength) + 8, nil
}

func (d *ProtobufStreamCommandDecoder) Reset(reader io.Reader) {
	d.reader.Reset(reader)
}
