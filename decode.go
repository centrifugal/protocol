package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/centrifugal/protocol/cfjson"
)

// CommandDecoder decodes commands from a transport frame which may contain
// several of them, see DataEncoder for the framing used.
//
// Decode returns io.EOF together with the last successfully decoded Command, so
// a non-nil Command must be handled even when an error is returned:
//
//	for {
//		cmd, err := decoder.Decode()
//		if cmd != nil {
//			// Handle the command.
//		}
//		if err != nil {
//			// io.EOF means the frame is fully processed.
//			break
//		}
//	}
//
// A CommandDecoder is not safe for concurrent use. Use GetCommandDecoder and
// PutCommandDecoder to take one from a pool and return it back when done.
type CommandDecoder interface {
	// Reset makes the decoder ready to decode commands from the given frame.
	Reset([]byte) error
	// Decode returns the next Command in the frame.
	Decode() (*Command, error)
}

// JSONCommandDecoder is a CommandDecoder for commands separated by a `\n`
// delimiter.
//
// Decoding is zero-copy: string fields of the returned Command point into the
// frame passed to NewJSONCommandDecoder or Reset rather than into copies of it.
// The frame must therefore stay unmodified for as long as the decoded commands
// are used – do not hand decoded commands to another goroutine while reusing the
// read buffer they came from. Raw payload fields are copied and are not affected.
// The Protobuf decoders copy everything, so this applies to JSON only.
type JSONCommandDecoder struct {
	data   []byte
	offset int
}

// NewJSONCommandDecoder creates a new JSONCommandDecoder for the given frame.
func NewJSONCommandDecoder(data []byte) *JSONCommandDecoder {
	return &JSONCommandDecoder{data: data}
}

// Reset makes the decoder ready to decode commands from the given frame.
func (d *JSONCommandDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode returns the next Command in the frame. The last Command is returned
// together with io.EOF, see the CommandDecoder interface.
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
	if err := cfjson.Unmarshal(msg, &c, 0); err != nil {
		return nil, err
	}
	if d.offset >= len(d.data) {
		return &c, io.EOF
	}
	return &c, nil
}

// ProtobufCommandDecoder is a CommandDecoder for commands prefixed with their
// length encoded as a varint.
type ProtobufCommandDecoder struct {
	data   []byte
	offset int
}

// NewProtobufCommandDecoder creates a new ProtobufCommandDecoder for the given
// frame.
func NewProtobufCommandDecoder(data []byte) *ProtobufCommandDecoder {
	return &ProtobufCommandDecoder{
		data: data,
	}
}

// Reset makes the decoder ready to decode commands from the given frame.
func (d *ProtobufCommandDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode returns the next Command in the frame. The last Command is returned
// together with io.EOF, see the CommandDecoder interface.
func (d *ProtobufCommandDecoder) Decode() (*Command, error) {
	if d.offset < len(d.data) {
		var c Command
		l, n := binary.Uvarint(d.data[d.offset:])
		if n <= 0 {
			return nil, io.EOF
		}
		// The length is declared by the other side: it must fit what is left
		// of the frame, which also makes the conversion below safe.
		from := d.offset + n
		if l > uint64(len(d.data)-from) { //nolint:gosec // G115: from <= len(d.data).
			return nil, io.ErrShortBuffer
		}
		to := from + int(l) //nolint:gosec // G115: l fits what is left, checked above.
		cmdBytes := d.data[from:to]
		err := c.UnmarshalCF(cmdBytes)
		if err != nil {
			return nil, err
		}
		d.offset = to
		if d.offset == len(d.data) {
			err = io.EOF
		}
		return &c, err
	}
	return nil, io.EOF
}

// ReplyDecoder decodes replies from a transport frame which may contain several
// of them. It's the client-side counterpart of ReplyEncoder.
//
// Unlike CommandDecoder, Decode here returns io.EOF on its own once the frame is
// fully processed – with a nil Reply.
//
// A ReplyDecoder is not safe for concurrent use.
type ReplyDecoder interface {
	// Reset makes the decoder ready to decode replies from the given frame.
	Reset([]byte) error
	// Decode returns the next Reply in the frame, or io.EOF if there are no
	// replies left.
	Decode() (*Reply, error)
}

var _ ReplyDecoder = NewJSONReplyDecoder(nil)

// JSONReplyDecoder is a ReplyDecoder which reads a stream of JSON replies, such
// as the `\n` separated frame produced by JSONDataEncoder.
type JSONReplyDecoder struct {
	data   []byte
	offset int
	err    error
}

// NewJSONReplyDecoder creates a new JSONReplyDecoder for the given frame.
func NewJSONReplyDecoder(data []byte) *JSONReplyDecoder {
	return &JSONReplyDecoder{data: data}
}

// Reset makes the decoder ready to decode replies from the given frame.
func (d *JSONReplyDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	d.err = nil
	return nil
}

// Decode returns the next Reply in the frame, or io.EOF if there are no replies
// left. Once Decode fails, it keeps returning the same error until Reset.
func (d *JSONReplyDecoder) Decode() (*Reply, error) {
	if d.err != nil {
		return nil, d.err
	}
	i := cfjson.SkipSpace(d.data, d.offset)
	if i >= len(d.data) {
		d.offset = len(d.data)
		d.err = io.EOF
		return nil, d.err
	}
	var c Reply
	n := c.DecodeJSON(d.data, i, 0)
	if n < 0 {
		d.err = cfjson.Error(d.data, n)
		return nil, d.err
	}
	d.offset = n
	return &c, nil
}

var _ ReplyDecoder = NewProtobufReplyDecoder(nil)

// ProtobufReplyDecoder is a ReplyDecoder for replies prefixed with their length
// encoded as a varint.
type ProtobufReplyDecoder struct {
	data   []byte
	offset int
}

// NewProtobufReplyDecoder creates a new ProtobufReplyDecoder for the given frame.
func NewProtobufReplyDecoder(data []byte) *ProtobufReplyDecoder {
	return &ProtobufReplyDecoder{
		data: data,
	}
}

// Reset makes the decoder ready to decode replies from the given frame.
func (d *ProtobufReplyDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode returns the next Reply in the frame, or io.EOF if there are no replies
// left. It returns io.ErrShortBuffer if a length prefix does not match the data
// which follows it.
func (d *ProtobufReplyDecoder) Decode() (*Reply, error) {
	if d.offset < len(d.data) {
		var c Reply
		l, n := binary.Uvarint(d.data[d.offset:])
		if n <= 0 {
			// Length prefix is truncated or overflows uint64, treat the frame
			// as fully processed.
			return nil, io.EOF
		}
		// The length is declared by the other side: it must fit what is left
		// of the frame, which also makes the conversion below safe.
		from := d.offset + n
		if l > uint64(len(d.data)-from) { //nolint:gosec // G115: from <= len(d.data).
			return nil, io.ErrShortBuffer
		}
		to := from + int(l) //nolint:gosec // G115: l fits what is left, checked above.
		replyBytes := d.data[from:to]
		err := c.UnmarshalCF(replyBytes)
		if err != nil {
			return nil, err
		}
		d.offset = to
		return &c, nil
	}
	return nil, io.EOF
}
