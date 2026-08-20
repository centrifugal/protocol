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

// StreamCommandDecoderTo is implemented by StreamCommandDecoder implementations
// which can decode into a Command owned by the caller, avoiding an allocation
// per command. Both decoders returned by GetStreamCommandDecoderLimited
// implement it, so a caller may type assert to it:
//
//	var cmd protocol.Command
//	if d, ok := decoder.(protocol.StreamCommandDecoderTo); ok {
//		size, err := d.DecodeTo(&cmd)
//		// ...
//	}
//
// It is deliberately a separate interface rather than a method on
// StreamCommandDecoder, so that existing implementations keep compiling.
type StreamCommandDecoderTo interface {
	StreamCommandDecoder
	// DecodeTo decodes the next Command from the stream into cmd, resetting it
	// first, and returns the number of bytes attributed to the command. Errors
	// are reported exactly as by Decode, including io.EOF together with the
	// last command in the stream.
	//
	// The returned size says whether cmd was written to: a non-zero size means
	// cmd holds a command which must be handled even when an error is returned
	// alongside it, while a zero size means cmd was left untouched and still
	// holds the previous command, so it must not be looked at. This mirrors the
	// nil Command which Decode returns in that case.
	//
	// cmd is only owned by the decoder for the duration of the call, so it may
	// be reused across calls. Reuse is only safe if nothing keeps the *Command
	// past the next DecodeTo call. Fields reachable from it are freshly
	// allocated on every call and may be retained: cmd is reset before
	// decoding, so nothing is unmarshaled into a struct kept from a previous
	// command.
	DecodeTo(cmd *Command) (int, error)
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
	// buffer, reused across Decode calls.
	buf []byte
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
	cmdBytes, err := d.nextCommandBytes()
	if len(cmdBytes) == 0 {
		return nil, 0, err
	}
	// Allocated only once there is a command to decode, so that reaching the
	// end of the stream costs nothing.
	var c Command
	if _, parseErr := json.Parse(cmdBytes, &c, 0); parseErr != nil {
		return nil, 0, parseErr
	}
	return &c, len(cmdBytes), err
}

// DecodeTo decodes the next Command from the stream into cmd, see the
// StreamCommandDecoderTo interface.
func (d *JSONStreamCommandDecoder) DecodeTo(cmd *Command) (int, error) {
	cmdBytes, err := d.nextCommandBytes()
	if len(cmdBytes) == 0 {
		return 0, err
	}
	cmd.Reset()
	if _, parseErr := json.Parse(cmdBytes, cmd, 0); parseErr != nil {
		return 0, parseErr
	}
	return len(cmdBytes), err
}

// nextCommandBytes returns the bytes of the next command in the stream. A nil
// result means no command was read, in which case the error explains why. A
// non-nil result may still come with io.EOF, which is how the last command in
// the stream is reported.
//
// The returned slice is only valid until the next call, see readLine.
func (d *JSONStreamCommandDecoder) nextCommandBytes() ([]byte, error) {
	if d.messageSizeLimit > 0 {
		d.limitedReader.N = int64(d.messageSizeLimit) + 1
	}
	cmdBytes, err := d.readLine()
	if err != nil {
		if d.messageSizeLimit > 0 && int64(len(cmdBytes)) > d.messageSizeLimit {
			return nil, ErrMessageTooLarge
		}
		if err == io.EOF && len(cmdBytes) > 0 {
			return cmdBytes, io.EOF
		}
		return nil, err
	}
	return cmdBytes, nil
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
	d.buf = append(d.buf[:0], chunk...)
	for {
		chunk, err = d.reader.ReadSlice('\n')
		d.buf = append(d.buf, chunk...)
		if err != bufio.ErrBufferFull {
			return d.buf, err
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
	if cap(d.buf) > maxRetainedLineBuffer {
		// A single large command must not make a pooled decoder hold on to a
		// large buffer for the rest of the process lifetime.
		d.buf = nil
	} else {
		d.buf = d.buf[:0]
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
	msgLength, err := d.nextMessageLength()
	if err != nil {
		return nil, 0, err
	}
	// Allocated only once there is a command to decode, so that reaching the
	// end of the stream costs nothing.
	var c Command
	if err = d.unmarshalMessage(&c, msgLength); err != nil {
		return nil, 0, err
	}
	return &c, int(msgLength) + 8, nil
}

// DecodeTo decodes the next Command from the stream into cmd, see the
// StreamCommandDecoderTo interface.
func (d *ProtobufStreamCommandDecoder) DecodeTo(cmd *Command) (int, error) {
	msgLength, err := d.nextMessageLength()
	if err != nil {
		return 0, err
	}
	cmd.Reset()
	if err = d.unmarshalMessage(cmd, msgLength); err != nil {
		return 0, err
	}
	return int(msgLength) + 8, nil
}

// nextMessageLength reads the length prefix of the next message and checks it
// against the configured limit, before any of the message body is read.
func (d *ProtobufStreamCommandDecoder) nextMessageLength() (uint64, error) {
	msgLength, err := binary.ReadUvarint(d.reader)
	if err != nil {
		return 0, err
	}
	if d.messageSizeLimit > 0 && msgLength > uint64(d.messageSizeLimit) {
		return 0, ErrMessageTooLarge
	}
	// The length is declared by the other side and is used as an allocation size
	// below, so it must be bounded even when no explicit limit is configured.
	if msgLength > maxMessageLength {
		return 0, ErrMessageTooLarge
	}
	return msgLength, nil
}

// unmarshalMessage reads msgLength bytes and unmarshals them into cmd.
func (d *ProtobufStreamCommandDecoder) unmarshalMessage(cmd *Command, msgLength uint64) error {
	// Fast path: the whole message is already buffered, so it can be unmarshaled
	// straight out of the bufio.Reader without copying it into a scratch buffer
	// first. UnmarshalVT copies what it keeps, so the peeked slice may be
	// invalidated by the Discard below.
	if msgBytes, peekErr := d.reader.Peek(int(msgLength)); peekErr == nil {
		if err := cmd.UnmarshalVT(msgBytes); err != nil { // Note, UnmarshalVTUnsafe here will result into issues.
			return err
		}
		_, err := d.reader.Discard(int(msgLength))
		return err
	}

	bb := getByteBuffer(int(msgLength))
	defer putByteBuffer(bb)

	n, err := io.ReadFull(d.reader, bb.B[:int(msgLength)])
	if err != nil {
		return err
	}
	if uint64(n) != msgLength {
		return io.ErrShortBuffer
	}
	return cmd.UnmarshalVT(bb.B[:int(msgLength)]) // Note, UnmarshalVTUnsafe here will result into issues.
}

// Reset makes the decoder read from the given reader, applying the given message
// size limit.
func (d *ProtobufStreamCommandDecoder) Reset(reader io.Reader, messageSizeLimit int64) {
	d.messageSizeLimit = messageSizeLimit
	d.reader.Reset(reader)
}

var (
	_ StreamCommandDecoderTo = (*JSONStreamCommandDecoder)(nil)
	_ StreamCommandDecoderTo = (*ProtobufStreamCommandDecoder)(nil)
)
