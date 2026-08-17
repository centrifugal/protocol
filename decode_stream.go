package protocol

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"

	"github.com/segmentio/encoding/json"
)

// maxMessageLength is a hard ceiling on the message length a Protobuf stream may
// declare. The length prefix comes from the other side before any of the message
// body is read, so it must never be used as an allocation size unchecked - it
// also must fit into an int on 32-bit platforms.
//
// This is a backstop, not a substitute for a real limit: configure an explicit
// one with GetStreamCommandDecoderLimited when reading untrusted input.
const maxMessageLength = math.MaxInt32

var (
	streamJsonCommandDecoderPool     sync.Pool
	streamProtobufCommandDecoderPool sync.Pool
)

// errNonPositiveMessageSizeLimit is the panic value used when a stream decoder
// is constructed without a positive message size limit. The limit prefix of a
// Protobuf stream frame is attacker-controlled and used as an allocation size, so
// an unbounded decoder reading untrusted input is a memory-exhaustion hazard.
// Requiring a positive limit at construction turns that misconfiguration into a
// loud, immediate failure instead of a silent one. See GHSA-4r3x-2rwr-6w65.
const errNonPositiveMessageSizeLimit = "protocol: stream command decoder requires a positive messageSizeLimit"

// GetStreamCommandDecoderLimited returns a StreamCommandDecoder for the given
// protocol type, taking it from a pool and resetting it to read commands from
// reader. Return it with PutStreamCommandDecoder once the stream is processed.
//
// Commands larger than messageSizeLimit bytes are rejected with
// ErrMessageTooLarge. messageSizeLimit must be positive - a zero or negative
// limit panics, since an unbounded decoder over untrusted input can be driven to
// allocate arbitrary memory by a single frame. Any type other than TypeJSON is
// treated as TypeProtobuf.
func GetStreamCommandDecoderLimited(protoType Type, reader io.Reader, messageSizeLimit int64) StreamCommandDecoder {
	if messageSizeLimit <= 0 {
		panic(errNonPositiveMessageSizeLimit)
	}
	if protoType == TypeJSON {
		e := streamJsonCommandDecoderPool.Get()
		if e == nil {
			return NewJSONStreamCommandDecoder(reader, messageSizeLimit)
		}
		commandDecoder := e.(*JSONStreamCommandDecoder)
		commandDecoder.Reset(reader, messageSizeLimit)
		return commandDecoder
	}
	e := streamProtobufCommandDecoderPool.Get()
	if e == nil {
		return NewProtobufStreamCommandDecoder(reader, messageSizeLimit)
	}
	commandDecoder := e.(*ProtobufStreamCommandDecoder)
	commandDecoder.Reset(reader, messageSizeLimit)
	return commandDecoder
}

// PutStreamCommandDecoder returns a StreamCommandDecoder obtained with
// GetStreamCommandDecoderLimited to the pool. The decoder must not be used after
// that.
func PutStreamCommandDecoder(protoType Type, e StreamCommandDecoder) {
	e.Reset(nil, 0)
	if protoType == TypeJSON {
		streamJsonCommandDecoderPool.Put(e)
		return
	}
	streamProtobufCommandDecoderPool.Put(e)
}

// StreamCommandDecoder decodes commands from an io.Reader. Unlike CommandDecoder,
// which works on a frame already read into memory, it's meant for streaming
// transports where commands arrive one after another and the size of an
// individual command must be bounded.
//
// A StreamCommandDecoder is not safe for concurrent use. Use
// GetStreamCommandDecoderLimited and PutStreamCommandDecoder to take one from a
// pool and return it back when done.
type StreamCommandDecoder interface {
	// Decode returns the next Command from the stream together with the number
	// of bytes attributed to it, or an error. It returns io.EOF when the stream
	// is over and ErrMessageTooLarge if the command exceeds the configured
	// message size limit.
	Decode() (*Command, int, error)
	// Reset makes the decoder read from the given reader, applying the given
	// message size limit. It is used internally to reuse a pooled decoder;
	// obtain a decoder via GetStreamCommandDecoderLimited, which enforces a
	// positive limit, rather than resetting one with a non-positive limit.
	Reset(reader io.Reader, messageSizeLimit int64)
}

// ErrMessageTooLarge is returned by a StreamCommandDecoder when a command in the
// stream exceeds the configured message size limit.
var ErrMessageTooLarge = errors.New("message size exceeds the limit")

// JSONStreamCommandDecoder is a StreamCommandDecoder which reads commands
// separated by a `\n` delimiter.
type JSONStreamCommandDecoder struct {
	reader           *bufio.Reader
	limitedReader    *io.LimitedReader
	messageSizeLimit int64
}

// NewJSONStreamCommandDecoder creates a new JSONStreamCommandDecoder reading from
// reader. messageSizeLimit must be positive; a zero or negative value panics.
func NewJSONStreamCommandDecoder(reader io.Reader, messageSizeLimit int64) *JSONStreamCommandDecoder {
	if messageSizeLimit <= 0 {
		panic(errNonPositiveMessageSizeLimit)
	}
	limitedReader := &io.LimitedReader{R: reader, N: messageSizeLimit + 1}
	return &JSONStreamCommandDecoder{
		reader:           bufio.NewReader(limitedReader),
		limitedReader:    limitedReader,
		messageSizeLimit: messageSizeLimit,
	}
}

// Decode returns the next Command from the stream, see the StreamCommandDecoder
// interface.
func (d *JSONStreamCommandDecoder) Decode() (*Command, int, error) {
	if d.messageSizeLimit > 0 {
		d.limitedReader.N = int64(d.messageSizeLimit) + 1
	}
	cmdBytes, err := d.reader.ReadBytes('\n')
	if err != nil {
		if d.messageSizeLimit > 0 && int64(len(cmdBytes)) > d.messageSizeLimit {
			return nil, 0, ErrMessageTooLarge
		}
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

// Reset makes the decoder read from the given reader, applying the given message
// size limit.
func (d *JSONStreamCommandDecoder) Reset(reader io.Reader, messageSizeLimit int64) {
	d.messageSizeLimit = messageSizeLimit
	if messageSizeLimit > 0 {
		limitedReader := &io.LimitedReader{R: reader, N: messageSizeLimit + 1}
		bufioReader := bufio.NewReader(limitedReader)
		d.limitedReader = limitedReader
		d.reader.Reset(bufioReader)
	} else {
		d.limitedReader = nil
		d.reader.Reset(reader)
	}
}

// ProtobufStreamCommandDecoder is a StreamCommandDecoder which reads commands
// prefixed with their length encoded as a varint.
type ProtobufStreamCommandDecoder struct {
	reader           *bufio.Reader
	messageSizeLimit int64
}

// NewProtobufStreamCommandDecoder creates a new ProtobufStreamCommandDecoder
// reading from reader. messageSizeLimit must be positive; a zero or negative
// value panics, since the varint length prefix is attacker-controlled and used as
// an allocation size, so an unbounded decoder over untrusted input is a
// memory-exhaustion hazard.
func NewProtobufStreamCommandDecoder(reader io.Reader, messageSizeLimit int64) *ProtobufStreamCommandDecoder {
	if messageSizeLimit <= 0 {
		panic(errNonPositiveMessageSizeLimit)
	}
	return &ProtobufStreamCommandDecoder{reader: bufio.NewReader(reader), messageSizeLimit: messageSizeLimit}
}

// Decode returns the next Command from the stream, see the StreamCommandDecoder
// interface. The size limit is checked against the length prefix before the
// command is read, so an oversized command is rejected without buffering it.
func (d *ProtobufStreamCommandDecoder) Decode() (*Command, int, error) {
	msgLength, err := binary.ReadUvarint(d.reader)
	if err != nil {
		return nil, 0, err
	}

	if d.messageSizeLimit > 0 && msgLength > uint64(d.messageSizeLimit) {
		return nil, 0, ErrMessageTooLarge
	}
	// The length is declared by the other side and is used as an allocation size
	// below, so it must be bounded even when no explicit limit is configured.
	if msgLength > maxMessageLength {
		return nil, 0, ErrMessageTooLarge
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
	err = c.UnmarshalVT(bb.B[:int(msgLength)]) // Note, UnmarshalVTUnsafe here will result into issues.
	if err != nil {
		return nil, 0, err
	}
	return &c, int(msgLength) + 8, nil
}

// Reset makes the decoder read from the given reader, applying the given message
// size limit.
func (d *ProtobufStreamCommandDecoder) Reset(reader io.Reader, messageSizeLimit int64) {
	d.messageSizeLimit = messageSizeLimit
	d.reader.Reset(reader)
}
