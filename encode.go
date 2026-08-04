package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"

	fastJSON "github.com/segmentio/encoding/json"
)

var errInvalidJSON = errors.New("invalid JSON data")

// checks that JSON is valid.
func isValidJSON(b []byte) error {
	if b == nil {
		return nil
	}
	if !fastJSON.Valid(b) {
		return errInvalidJSON
	}
	return nil
}

// PushEncoder encodes Push and its parts to bytes.
//
// Parts of a Push are encoded separately from the Push envelope, so that a
// payload encoded once may be reused for all subscribers of a channel.
//
// Methods which accept a variadic reuse argument may write the result into the
// provided buffer if it's large enough, avoiding an allocation. The returned
// slice may or may not alias the given buffer, so always use the returned value
// and don't touch the buffer until the result is not needed anymore.
type PushEncoder interface {
	Encode(*Push) ([]byte, error)
	EncodeMessage(*Message, ...[]byte) ([]byte, error)
	EncodePublication(*Publication, ...[]byte) ([]byte, error)
	EncodeJoin(*Join, ...[]byte) ([]byte, error)
	EncodeLeave(*Leave, ...[]byte) ([]byte, error)
	EncodeUnsubscribe(*Unsubscribe, ...[]byte) ([]byte, error)
	EncodeSubscribe(*Subscribe, ...[]byte) ([]byte, error)
	EncodeConnect(*Connect, ...[]byte) ([]byte, error)
	EncodeDisconnect(*Disconnect, ...[]byte) ([]byte, error)
	EncodeRefresh(*Refresh, ...[]byte) ([]byte, error)
}

var _ PushEncoder = (*JSONPushEncoder)(nil)
var _ PushEncoder = (*ProtobufPushEncoder)(nil)

// JSONPushEncoder is a PushEncoder which encodes to JSON.
type JSONPushEncoder struct {
}

// NewJSONPushEncoder creates a new JSONPushEncoder. It's safe to use the
// returned encoder concurrently, see also DefaultJsonPushEncoder.
func NewJSONPushEncoder() *JSONPushEncoder {
	return &JSONPushEncoder{}
}

// Encode Push to bytes.
func (e *JSONPushEncoder) Encode(message *Push) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	res, err := jw.BuildBytes()
	if err != nil {
		return nil, err
	}
	if err := isValidJSON(res); err != nil {
		return nil, err
	}
	return res, nil
}

// EncodePublication to bytes.
func (e *JSONPushEncoder) EncodePublication(message *Publication, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeMessage to bytes.
func (e *JSONPushEncoder) EncodeMessage(message *Message, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeJoin to bytes.
func (e *JSONPushEncoder) EncodeJoin(message *Join, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeLeave to bytes.
func (e *JSONPushEncoder) EncodeLeave(message *Leave, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeUnsubscribe to bytes.
func (e *JSONPushEncoder) EncodeUnsubscribe(message *Unsubscribe, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeSubscribe to bytes.
func (e *JSONPushEncoder) EncodeSubscribe(message *Subscribe, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeConnect to bytes.
func (e *JSONPushEncoder) EncodeConnect(message *Connect, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeDisconnect to bytes.
func (e *JSONPushEncoder) EncodeDisconnect(message *Disconnect, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeRefresh to bytes.
func (e *JSONPushEncoder) EncodeRefresh(message *Refresh, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// ProtobufPushEncoder is a PushEncoder which encodes to Protobuf.
type ProtobufPushEncoder struct {
}

// NewProtobufPushEncoder creates a new ProtobufPushEncoder. It's safe to use the
// returned encoder concurrently, see also DefaultProtobufPushEncoder.
func NewProtobufPushEncoder() *ProtobufPushEncoder {
	return &ProtobufPushEncoder{}
}

// Encode Push to bytes.
func (e *ProtobufPushEncoder) Encode(message *Push) ([]byte, error) {
	return message.MarshalVT()
}

// EncodePublication to bytes.
func (e *ProtobufPushEncoder) EncodePublication(message *Publication, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeMessage to bytes.
func (e *ProtobufPushEncoder) EncodeMessage(message *Message, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeJoin to bytes.
func (e *ProtobufPushEncoder) EncodeJoin(message *Join, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeLeave to bytes.
func (e *ProtobufPushEncoder) EncodeLeave(message *Leave, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeUnsubscribe to bytes.
func (e *ProtobufPushEncoder) EncodeUnsubscribe(message *Unsubscribe, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeSubscribe to bytes.
func (e *ProtobufPushEncoder) EncodeSubscribe(message *Subscribe, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeConnect to bytes.
func (e *ProtobufPushEncoder) EncodeConnect(message *Connect, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeDisconnect to bytes.
func (e *ProtobufPushEncoder) EncodeDisconnect(message *Disconnect, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// EncodeRefresh to bytes.
func (e *ProtobufPushEncoder) EncodeRefresh(message *Refresh, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		size := message.SizeVT()
		if cap(reuse[0]) >= size {
			n, err := message.MarshalToSizedBufferVT(reuse[0][:size])
			if err != nil {
				return nil, err
			}
			return reuse[0][:n], nil
		}
	}
	return message.MarshalVT()
}

// ReplyEncoder encodes Reply to bytes. Use GetReplyEncoder to get an
// implementation for a concrete protocol Type.
type ReplyEncoder interface {
	Encode(*Reply) ([]byte, error)
}

// JSONReplyEncoder is a ReplyEncoder which encodes to JSON.
type JSONReplyEncoder struct{}

// NewJSONReplyEncoder creates a new JSONReplyEncoder. It's safe to use the
// returned encoder concurrently, see also DefaultJsonReplyEncoder.
func NewJSONReplyEncoder() *JSONReplyEncoder {
	return &JSONReplyEncoder{}
}

// Encode Reply to bytes.
func (e *JSONReplyEncoder) Encode(r *Reply) ([]byte, error) {
	jw := newWriter()
	r.MarshalEasyJSON(jw)
	result, err := jw.BuildBytes()
	if err != nil {
		return nil, err
	}
	if err := isValidJSON(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ProtobufReplyEncoder is a ReplyEncoder which encodes to Protobuf.
type ProtobufReplyEncoder struct{}

// NewProtobufReplyEncoder creates a new ProtobufReplyEncoder. It's safe to use
// the returned encoder concurrently, see also DefaultProtobufReplyEncoder.
func NewProtobufReplyEncoder() *ProtobufReplyEncoder {
	return &ProtobufReplyEncoder{}
}

// Encode Reply to bytes.
func (e *ProtobufReplyEncoder) Encode(r *Reply) ([]byte, error) {
	return r.MarshalVT()
}

// DataEncoder concatenates already encoded messages into a single transport
// frame, applying the framing of a concrete protocol Type: messages are
// separated by a `\n` delimiter in JSON and prefixed with their length in
// Protobuf.
//
// A DataEncoder is not safe for concurrent use. Use GetDataEncoder and
// PutDataEncoder to take one from a pool and return it back when done.
type DataEncoder interface {
	// Reset prepares the encoder to build a new frame, dropping everything
	// encoded so far.
	Reset()
	// Encode appends an already encoded message to the frame.
	Encode([]byte) error
	// Finish returns a copy of the frame built so far.
	Finish() []byte
	// FinishNoCopy returns the frame built so far without copying it. The
	// returned slice is only valid until the next Reset or Encode call, so use
	// it only if the data is written to a connection before that happens.
	FinishNoCopy() []byte
}

// JSONDataEncoder is a DataEncoder which separates messages by a `\n` delimiter.
type JSONDataEncoder struct {
	count  int
	buffer bytes.Buffer
}

// NewJSONDataEncoder creates a new JSONDataEncoder.
func NewJSONDataEncoder() *JSONDataEncoder {
	return &JSONDataEncoder{}
}

// Reset prepares the encoder to build a new frame.
func (e *JSONDataEncoder) Reset() {
	e.count = 0
	e.buffer.Reset()
}

// Encode appends an already encoded message to the frame, separating it from
// the previous one with a `\n` delimiter.
func (e *JSONDataEncoder) Encode(data []byte) error {
	if e.count > 0 {
		e.buffer.WriteString("\n")
	}
	e.buffer.Write(data)
	e.count++
	return nil
}

// Finish returns a copy of the frame built so far.
func (e *JSONDataEncoder) Finish() []byte {
	data := e.buffer.Bytes()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy
}

// FinishNoCopy returns the frame built so far without copying it, see the
// DataEncoder interface for the lifetime of the returned slice.
func (e *JSONDataEncoder) FinishNoCopy() []byte {
	return e.buffer.Bytes()
}

// ProtobufDataEncoder is a DataEncoder which prefixes each message with its
// length encoded as a varint.
type ProtobufDataEncoder struct {
	buffer bytes.Buffer
}

// NewProtobufDataEncoder creates a new ProtobufDataEncoder.
func NewProtobufDataEncoder() *ProtobufDataEncoder {
	return &ProtobufDataEncoder{}
}

// Encode appends an already encoded message to the frame, prefixing it with its
// length encoded as a varint.
func (e *ProtobufDataEncoder) Encode(data []byte) error {
	bs := make([]byte, 8)
	n := binary.PutUvarint(bs, uint64(len(data)))
	e.buffer.Write(bs[:n])
	e.buffer.Write(data)
	return nil
}

// Reset prepares the encoder to build a new frame.
func (e *ProtobufDataEncoder) Reset() {
	e.buffer.Reset()
}

// Finish returns a copy of the frame built so far.
func (e *ProtobufDataEncoder) Finish() []byte {
	data := e.buffer.Bytes()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy
}

// FinishNoCopy returns the frame built so far without copying it, see the
// DataEncoder interface for the lifetime of the returned slice.
func (e *ProtobufDataEncoder) FinishNoCopy() []byte {
	return e.buffer.Bytes()
}

// ResultEncoder encodes command results to bytes. Results are encoded separately
// from the Reply envelope, so that a result may be encoded once and then reused
// for many connections. Use GetResultEncoder to get an implementation for a
// concrete protocol Type.
type ResultEncoder interface {
	EncodeConnectResult(*ConnectResult) ([]byte, error)
	EncodeRefreshResult(*RefreshResult) ([]byte, error)
	EncodeSubscribeResult(*SubscribeResult) ([]byte, error)
	EncodeSubRefreshResult(*SubRefreshResult) ([]byte, error)
	EncodeUnsubscribeResult(*UnsubscribeResult) ([]byte, error)
	EncodePublishResult(*PublishResult) ([]byte, error)
	EncodePresenceResult(*PresenceResult) ([]byte, error)
	EncodePresenceStatsResult(*PresenceStatsResult) ([]byte, error)
	EncodeHistoryResult(*HistoryResult) ([]byte, error)
	EncodePingResult(*PingResult) ([]byte, error)
	EncodeRPCResult(*RPCResult) ([]byte, error)
}

// JSONResultEncoder is a ResultEncoder which encodes to JSON.
type JSONResultEncoder struct{}

// NewJSONResultEncoder creates a new JSONResultEncoder. It's safe to use the
// returned encoder concurrently.
func NewJSONResultEncoder() *JSONResultEncoder {
	return &JSONResultEncoder{}
}

// EncodeConnectResult encodes ConnectResult to bytes.
func (e *JSONResultEncoder) EncodeConnectResult(res *ConnectResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeRefreshResult encodes RefreshResult to bytes.
func (e *JSONResultEncoder) EncodeRefreshResult(res *RefreshResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeSubscribeResult encodes SubscribeResult to bytes.
func (e *JSONResultEncoder) EncodeSubscribeResult(res *SubscribeResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeSubRefreshResult encodes SubRefreshResult to bytes.
func (e *JSONResultEncoder) EncodeSubRefreshResult(res *SubRefreshResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeUnsubscribeResult encodes UnsubscribeResult to bytes.
func (e *JSONResultEncoder) EncodeUnsubscribeResult(res *UnsubscribeResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePublishResult encodes PublishResult to bytes.
func (e *JSONResultEncoder) EncodePublishResult(res *PublishResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePresenceResult encodes PresenceResult to bytes.
func (e *JSONResultEncoder) EncodePresenceResult(res *PresenceResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePresenceStatsResult encodes PresenceStatsResult to bytes.
func (e *JSONResultEncoder) EncodePresenceStatsResult(res *PresenceStatsResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeHistoryResult encodes HistoryResult to bytes.
func (e *JSONResultEncoder) EncodeHistoryResult(res *HistoryResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePingResult encodes PingResult to bytes.
func (e *JSONResultEncoder) EncodePingResult(res *PingResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeRPCResult encodes RPCResult to bytes.
func (e *JSONResultEncoder) EncodeRPCResult(res *RPCResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// ProtobufResultEncoder is a ResultEncoder which encodes to Protobuf.
type ProtobufResultEncoder struct{}

// NewProtobufResultEncoder creates a new ProtobufResultEncoder. It's safe to use
// the returned encoder concurrently.
func NewProtobufResultEncoder() *ProtobufResultEncoder {
	return &ProtobufResultEncoder{}
}

// EncodeConnectResult encodes ConnectResult to bytes.
func (e *ProtobufResultEncoder) EncodeConnectResult(res *ConnectResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodeRefreshResult encodes RefreshResult to bytes.
func (e *ProtobufResultEncoder) EncodeRefreshResult(res *RefreshResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodeSubscribeResult encodes SubscribeResult to bytes.
func (e *ProtobufResultEncoder) EncodeSubscribeResult(res *SubscribeResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodeSubRefreshResult encodes SubRefreshResult to bytes.
func (e *ProtobufResultEncoder) EncodeSubRefreshResult(res *SubRefreshResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodeUnsubscribeResult encodes UnsubscribeResult to bytes.
func (e *ProtobufResultEncoder) EncodeUnsubscribeResult(res *UnsubscribeResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodePublishResult encodes PublishResult to bytes.
func (e *ProtobufResultEncoder) EncodePublishResult(res *PublishResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodePresenceResult encodes PresenceResult to bytes.
func (e *ProtobufResultEncoder) EncodePresenceResult(res *PresenceResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodePresenceStatsResult encodes PresenceStatsResult to bytes.
func (e *ProtobufResultEncoder) EncodePresenceStatsResult(res *PresenceStatsResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodeHistoryResult encodes HistoryResult to bytes.
func (e *ProtobufResultEncoder) EncodeHistoryResult(res *HistoryResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodePingResult encodes PingResult to bytes.
func (e *ProtobufResultEncoder) EncodePingResult(res *PingResult) ([]byte, error) {
	return res.MarshalVT()
}

// EncodeRPCResult encodes RPCResult to bytes.
func (e *ProtobufResultEncoder) EncodeRPCResult(res *RPCResult) ([]byte, error) {
	return res.MarshalVT()
}

// CommandEncoder encodes Command to bytes. It's the client-side counterpart of
// CommandDecoder.
type CommandEncoder interface {
	Encode(cmd *Command) ([]byte, error)
}

// JSONCommandEncoder is a CommandEncoder which encodes to JSON.
type JSONCommandEncoder struct {
}

// NewJSONCommandEncoder creates a new JSONCommandEncoder. It's safe to use the
// returned encoder concurrently.
func NewJSONCommandEncoder() *JSONCommandEncoder {
	return &JSONCommandEncoder{}
}

// Encode Command to bytes. The result contains no framing: to send several
// commands in one frame join them with a `\n` delimiter, see JSONDataEncoder.
func (e *JSONCommandEncoder) Encode(cmd *Command) ([]byte, error) {
	jw := newWriter()
	cmd.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// ProtobufCommandEncoder is a CommandEncoder which encodes to Protobuf.
type ProtobufCommandEncoder struct {
}

// NewProtobufCommandEncoder creates a new ProtobufCommandEncoder. It's safe to
// use the returned encoder concurrently.
func NewProtobufCommandEncoder() *ProtobufCommandEncoder {
	return &ProtobufCommandEncoder{}
}

// Encode Command to bytes prefixed with the command length encoded as a varint,
// so that encoded commands may be sent one after another in a single frame.
func (e *ProtobufCommandEncoder) Encode(cmd *Command) ([]byte, error) {
	commandBytes, err := cmd.MarshalVT()
	if err != nil {
		return nil, err
	}
	bs := make([]byte, 8)
	n := binary.PutUvarint(bs, uint64(len(commandBytes)))
	var buf bytes.Buffer
	buf.Write(bs[:n])
	buf.Write(commandBytes)
	return buf.Bytes(), nil
}
