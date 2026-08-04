package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/segmentio/encoding/json"
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
	data            []byte
	messageCount    int
	prevNewLine     int
	numMessagesRead int
}

// NewJSONCommandDecoder creates a new JSONCommandDecoder for the given frame.
func NewJSONCommandDecoder(data []byte) *JSONCommandDecoder {
	// Protocol message must be separated by exactly one `\n`.
	messageCount := bytes.Count(data, []byte("\n")) + 1
	if len(data) == 0 || data[len(data)-1] == '\n' {
		// Protocol message must have zero or one `\n` at the end.
		messageCount--
	}
	return &JSONCommandDecoder{
		data:            data,
		messageCount:    messageCount,
		prevNewLine:     0,
		numMessagesRead: 0,
	}
}

// Reset makes the decoder ready to decode commands from the given frame.
func (d *JSONCommandDecoder) Reset(data []byte) error {
	// We have a strict contract that protocol messages should be separated by at most one `\n`.
	messageCount := bytes.Count(data, []byte("\n")) + 1
	if len(data) == 0 || data[len(data)-1] == '\n' {
		// We have a strict contract that protocol message should use at most one `\n` at the end.
		messageCount--
	}
	d.data = data
	d.messageCount = messageCount
	d.prevNewLine = 0
	d.numMessagesRead = 0
	return nil
}

// Decode returns the next Command in the frame. The last Command is returned
// together with io.EOF, see the CommandDecoder interface.
func (d *JSONCommandDecoder) Decode() (*Command, error) {
	if d.messageCount == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	var c Command
	if d.messageCount == 1 {
		_, err := json.Parse(d.data, &c, json.ZeroCopy)
		if err != nil {
			return nil, err
		}
		return &c, io.EOF
	}
	var nextNewLine int
	if d.numMessagesRead == d.messageCount-1 {
		// Last message, no need to search for a new line.
		nextNewLine = len(d.data[d.prevNewLine:])
	} else if len(d.data) > d.prevNewLine {
		nextNewLine = bytes.Index(d.data[d.prevNewLine:], []byte("\n"))
		if nextNewLine < 0 {
			return nil, io.ErrShortBuffer
		}
	} else {
		return nil, io.ErrShortBuffer
	}
	if len(d.data) >= d.prevNewLine+nextNewLine {
		_, err := json.Parse(d.data[d.prevNewLine:d.prevNewLine+nextNewLine], &c, json.ZeroCopy)
		if err != nil {
			return nil, err
		}
		d.numMessagesRead++
		d.prevNewLine = d.prevNewLine + nextNewLine + 1
		if d.numMessagesRead == d.messageCount {
			return &c, io.EOF
		}
		return &c, nil
	} else {
		return nil, io.ErrShortBuffer
	}
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
	decoder *json.Decoder
}

// NewJSONReplyDecoder creates a new JSONReplyDecoder for the given frame.
func NewJSONReplyDecoder(data []byte) *JSONReplyDecoder {
	return &JSONReplyDecoder{
		decoder: json.NewDecoder(bytes.NewReader(data)),
	}
}

// Reset makes the decoder ready to decode replies from the given frame.
func (d *JSONReplyDecoder) Reset(data []byte) error {
	d.decoder = json.NewDecoder(bytes.NewReader(data))
	return nil
}

// Decode returns the next Reply in the frame, or io.EOF if there are no replies
// left.
func (d *JSONReplyDecoder) Decode() (*Reply, error) {
	var c Reply
	err := d.decoder.Decode(&c)
	if err != nil {
		return nil, err
	}
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
		from := d.offset + n
		to := d.offset + n + int(l)
		// The from <= to part also catches an int overflow of the addition above.
		if from > to || to > len(d.data) {
			return nil, io.ErrShortBuffer
		}
		replyBytes := d.data[from:to]
		err := c.UnmarshalVT(replyBytes)
		if err != nil {
			return nil, err
		}
		d.offset = to
		return &c, nil
	}
	return nil, io.EOF
}
