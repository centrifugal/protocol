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

// maxRetainedLineBuffer bounds the read buffer a pooled JSONStreamCommandDecoder
// keeps between uses. Commands larger than this are still decoded, their buffer
// is just dropped instead of being retained by the pool.
const maxRetainedLineBuffer = 65536

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
	// buf accumulates a command which does not fit into the bufio.Reader
	// buffer, reused across Decode calls. It's held behind a pointer so that
	// the struct stays comparable, as it was before the field was added.
	buf *ByteBuffer
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
	cmdBytes, err := d.readLine()
	// The limit is checked on both paths out of readLine. Checking it only
	// when reading failed is not enough: a command whose delimiter was already
	// buffered under an earlier Decode's budget comes back with a nil error,
	// and would otherwise skip the check entirely.
	if d.messageSizeLimit > 0 && int64(commandLen(cmdBytes)) > d.messageSizeLimit {
		return nil, 0, ErrMessageTooLarge
	}
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

// commandLen returns the length of a command read by readLine, without the `\n`
// delimiter. The delimiter separates commands rather than belonging to one, and
// the last command in a frame carries none, so counting it would make the size
// limit depend on where in the frame a command sits.
func commandLen(cmdBytes []byte) int {
	if n := len(cmdBytes); n > 0 && cmdBytes[n-1] == '\n' {
		return n - 1
	}
	return len(cmdBytes)
}

// readLine returns the next `\n` terminated command, including the delimiter.
//
// The returned slice is only valid until the next Decode call - it may point
// into the bufio.Reader buffer or into a buffer reused across calls. Callers
// must copy anything they keep, which json.Parse does since it's used here
// without the ZeroCopy flag.
func (d *JSONStreamCommandDecoder) readLine() ([]byte, error) {
	chunk, err := d.reader.ReadSlice('\n')
	if err != bufio.ErrBufferFull {
		// Fast path: the whole command was in the bufio.Reader buffer, so
		// there is nothing to accumulate and nothing to allocate.
		return chunk, err
	}
	if d.buf == nil {
		d.buf = &ByteBuffer{}
	}
	d.buf.B = append(d.buf.B[:0], chunk...)
	for {
		chunk, err = d.reader.ReadSlice('\n')
		d.buf.B = append(d.buf.B, chunk...)
		if err != bufio.ErrBufferFull {
			return d.buf.B, err
		}
	}
}

// Reset makes the decoder read from the given reader, applying the given message
// size limit.
func (d *JSONStreamCommandDecoder) Reset(reader io.Reader, messageSizeLimit int64) {
	d.messageSizeLimit = messageSizeLimit
	if messageSizeLimit > 0 {
		if d.limitedReader == nil {
			d.limitedReader = &io.LimitedReader{}
		}
		d.limitedReader.R = reader
		d.limitedReader.N = messageSizeLimit + 1
		d.reader.Reset(d.limitedReader)
	} else {
		if d.limitedReader != nil {
			// Drop the reference so a pooled decoder does not pin it.
			d.limitedReader.R = nil
			d.limitedReader.N = 0
		}
		d.reader.Reset(reader)
	}
	if d.buf != nil {
		if cap(d.buf.B) > maxRetainedLineBuffer {
			// A single large command must not make a pooled decoder hold on to
			// a large buffer for the rest of the process lifetime.
			d.buf = nil
		} else {
			d.buf.B = d.buf.B[:0]
		}
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

	// Fast path: the whole message is already buffered, so it can be unmarshaled
	// straight out of the bufio.Reader without copying it into a scratch buffer
	// first. UnmarshalVT copies what it keeps, so the peeked slice may be
	// invalidated by the Discard below.
	if msgBytes, peekErr := d.reader.Peek(int(msgLength)); peekErr == nil {
		var c Command
		err = c.UnmarshalVT(msgBytes) // Note, UnmarshalVTUnsafe here will result into issues.
		// The message is consumed even when it failed to unmarshal, matching
		// the scratch buffer path below, which reads it off the stream before
		// unmarshaling it. A caller which keeps decoding after an error must
		// see the next message rather than this body again.
		if _, discardErr := d.reader.Discard(int(msgLength)); discardErr != nil && err == nil {
			err = discardErr
		}
		if err != nil {
			return nil, 0, err
		}
		return &c, int(msgLength) + 8, nil
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
