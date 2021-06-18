package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
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

// PushEncoder ...
type PushEncoder interface {
	Encode(*Push) ([]byte, error)
	EncodeMessage(*Message) ([]byte, error)
	EncodePublication(*Publication) ([]byte, error)
	EncodeJoin(*Join) ([]byte, error)
	EncodeLeave(*Leave) ([]byte, error)
	EncodeUnsubscribe(*Unsubscribe) ([]byte, error)
	EncodeSubscribe(*Subscribe) ([]byte, error)
	EncodeConnect(*Connect) ([]byte, error)
	EncodeDisconnect(*Disconnect) ([]byte, error)
}

var _ PushEncoder = (*JSONPushEncoder)(nil)
var _ PushEncoder = (*ProtobufPushEncoder)(nil)

// JSONPushEncoder ...
type JSONPushEncoder struct {
}

// NewJSONPushEncoder ...
func NewJSONPushEncoder() *JSONPushEncoder {
	return &JSONPushEncoder{}
}

// Encode ...
func (e *JSONPushEncoder) Encode(message *Push) ([]byte, error) {
	// Check data is valid JSON.
	if err := isValidJSON(message.Data); err != nil {
		return nil, err
	}
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePublication ...
func (e *JSONPushEncoder) EncodePublication(message *Publication) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeMessage ...
func (e *JSONPushEncoder) EncodeMessage(message *Message) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeJoin ...
func (e *JSONPushEncoder) EncodeJoin(message *Join) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeLeave ...
func (e *JSONPushEncoder) EncodeLeave(message *Leave) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeUnsub ...
func (e *JSONPushEncoder) EncodeUnsubscribe(message *Unsubscribe) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeSub ...
func (e *JSONPushEncoder) EncodeSubscribe(message *Subscribe) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeConn ...
func (e *JSONPushEncoder) EncodeConnect(message *Connect) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeDisconnect ...
func (e *JSONPushEncoder) EncodeDisconnect(message *Disconnect) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// ProtobufPushEncoder ...
type ProtobufPushEncoder struct {
}

// NewProtobufPushEncoder ...
func NewProtobufPushEncoder() *ProtobufPushEncoder {
	return &ProtobufPushEncoder{}
}

// Encode ...
func (e *ProtobufPushEncoder) Encode(message *Push) ([]byte, error) {
	return message.Marshal()
}

// EncodePublication ...
func (e *ProtobufPushEncoder) EncodePublication(message *Publication) ([]byte, error) {
	return message.Marshal()
}

// EncodeMessage ...
func (e *ProtobufPushEncoder) EncodeMessage(message *Message) ([]byte, error) {
	return message.Marshal()
}

// EncodeJoin ...
func (e *ProtobufPushEncoder) EncodeJoin(message *Join) ([]byte, error) {
	return message.Marshal()
}

// EncodeLeave ...
func (e *ProtobufPushEncoder) EncodeLeave(message *Leave) ([]byte, error) {
	return message.Marshal()
}

// EncodeUnsub ...
func (e *ProtobufPushEncoder) EncodeUnsubscribe(message *Unsubscribe) ([]byte, error) {
	return message.Marshal()
}

// EncodeSub ...
func (e *ProtobufPushEncoder) EncodeSubscribe(message *Subscribe) ([]byte, error) {
	return message.Marshal()
}

// EncodeConn ...
func (e *ProtobufPushEncoder) EncodeConnect(message *Connect) ([]byte, error) {
	return message.Marshal()
}

// EncodeDisconnect ...
func (e *ProtobufPushEncoder) EncodeDisconnect(message *Disconnect) ([]byte, error) {
	return message.Marshal()
}

// ReplyEncoder ...
type ReplyEncoder interface {
	Encode(*Reply) ([]byte, error)
}

// JSONReplyEncoder ...
type JSONReplyEncoder struct{}

// NewJSONReplyEncoder ...
func NewJSONReplyEncoder() *JSONReplyEncoder {
	return &JSONReplyEncoder{}
}

// Encode ...
func (e *JSONReplyEncoder) Encode(r *Reply) ([]byte, error) {
	if r.Id != 0 {
		// Only check command result reply. Push reply JSON validation is done in PushEncoder.
		if err := isValidJSON(r.Result); err != nil {
			return nil, err
		}
	}
	jw := newWriter()
	r.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// ProtobufReplyEncoder ...
type ProtobufReplyEncoder struct{}

// NewProtobufReplyEncoder ...
func NewProtobufReplyEncoder() *ProtobufReplyEncoder {
	return &ProtobufReplyEncoder{}
}

// Encode ...
func (e *ProtobufReplyEncoder) Encode(r *Reply) ([]byte, error) {
	return r.Marshal()
}

// DataEncoder ...
type DataEncoder interface {
	Reset()
	Encode([]byte) error
	Finish() []byte
}

// JSONDataEncoder ...
type JSONDataEncoder struct {
	count  int
	buffer bytes.Buffer
}

// NewJSONDataEncoder ...
func NewJSONDataEncoder() *JSONDataEncoder {
	return &JSONDataEncoder{}
}

// Reset ...
func (e *JSONDataEncoder) Reset() {
	e.count = 0
	e.buffer.Reset()
}

// Encode ...
func (e *JSONDataEncoder) Encode(data []byte) error {
	if e.count > 0 {
		e.buffer.WriteString("\n")
	}
	e.buffer.Write(data)
	e.count++
	return nil
}

// Finish ...
func (e *JSONDataEncoder) Finish() []byte {
	data := e.buffer.Bytes()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy
}

// ProtobufDataEncoder ...
type ProtobufDataEncoder struct {
	buffer bytes.Buffer
}

// NewProtobufDataEncoder ...
func NewProtobufDataEncoder() *ProtobufDataEncoder {
	return &ProtobufDataEncoder{}
}

// Encode ...
func (e *ProtobufDataEncoder) Encode(data []byte) error {
	bs := make([]byte, 8)
	n := binary.PutUvarint(bs, uint64(len(data)))
	e.buffer.Write(bs[:n])
	e.buffer.Write(data)
	return nil
}

// Reset ...
func (e *ProtobufDataEncoder) Reset() {
	e.buffer.Reset()
}

// Finish ...
func (e *ProtobufDataEncoder) Finish() []byte {
	data := e.buffer.Bytes()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy
}

// ResultEncoder ...
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

// JSONResultEncoder ...
type JSONResultEncoder struct{}

// NewJSONResultEncoder ...
func NewJSONResultEncoder() *JSONResultEncoder {
	return &JSONResultEncoder{}
}

// EncodeConnectResult ...
func (e *JSONResultEncoder) EncodeConnectResult(res *ConnectResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeRefreshResult ...
func (e *JSONResultEncoder) EncodeRefreshResult(res *RefreshResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeSubscribeResult ...
func (e *JSONResultEncoder) EncodeSubscribeResult(res *SubscribeResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeSubRefreshResult ...
func (e *JSONResultEncoder) EncodeSubRefreshResult(res *SubRefreshResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeUnsubscribeResult ...
func (e *JSONResultEncoder) EncodeUnsubscribeResult(res *UnsubscribeResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePublishResult ...
func (e *JSONResultEncoder) EncodePublishResult(res *PublishResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePresenceResult ...
func (e *JSONResultEncoder) EncodePresenceResult(res *PresenceResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePresenceStatsResult ...
func (e *JSONResultEncoder) EncodePresenceStatsResult(res *PresenceStatsResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeHistoryResult ...
func (e *JSONResultEncoder) EncodeHistoryResult(res *HistoryResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodePingResult ...
func (e *JSONResultEncoder) EncodePingResult(res *PingResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// EncodeRPCResult ...
func (e *JSONResultEncoder) EncodeRPCResult(res *RPCResult) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// ProtobufResultEncoder ...
type ProtobufResultEncoder struct{}

// NewProtobufResultEncoder ...
func NewProtobufResultEncoder() *ProtobufResultEncoder {
	return &ProtobufResultEncoder{}
}

// EncodeConnectResult ...
func (e *ProtobufResultEncoder) EncodeConnectResult(res *ConnectResult) ([]byte, error) {
	return res.Marshal()
}

// EncodeRefreshResult ...
func (e *ProtobufResultEncoder) EncodeRefreshResult(res *RefreshResult) ([]byte, error) {
	return res.Marshal()
}

// EncodeSubscribeResult ...
func (e *ProtobufResultEncoder) EncodeSubscribeResult(res *SubscribeResult) ([]byte, error) {
	return res.Marshal()
}

// EncodeSubRefreshResult ...
func (e *ProtobufResultEncoder) EncodeSubRefreshResult(res *SubRefreshResult) ([]byte, error) {
	return res.Marshal()
}

// EncodeUnsubscribeResult ...
func (e *ProtobufResultEncoder) EncodeUnsubscribeResult(res *UnsubscribeResult) ([]byte, error) {
	return res.Marshal()
}

// EncodePublishResult ...
func (e *ProtobufResultEncoder) EncodePublishResult(res *PublishResult) ([]byte, error) {
	return res.Marshal()
}

// EncodePresenceResult ...
func (e *ProtobufResultEncoder) EncodePresenceResult(res *PresenceResult) ([]byte, error) {
	return res.Marshal()
}

// EncodePresenceStatsResult ...
func (e *ProtobufResultEncoder) EncodePresenceStatsResult(res *PresenceStatsResult) ([]byte, error) {
	return res.Marshal()
}

// EncodeHistoryResult ...
func (e *ProtobufResultEncoder) EncodeHistoryResult(res *HistoryResult) ([]byte, error) {
	return res.Marshal()
}

// EncodePingResult ...
func (e *ProtobufResultEncoder) EncodePingResult(res *PingResult) ([]byte, error) {
	return res.Marshal()
}

// EncodeRPCResult ...
func (e *ProtobufResultEncoder) EncodeRPCResult(res *RPCResult) ([]byte, error) {
	return res.Marshal()
}

// CommandEncoder ...
type CommandEncoder interface {
	Encode(cmd *Command) ([]byte, error)
}

// JSONCommandEncoder ...
type JSONCommandEncoder struct {
}

// NewJSONCommandEncoder ...
func NewJSONCommandEncoder() *JSONCommandEncoder {
	return &JSONCommandEncoder{}
}

// Encode ...
func (e *JSONCommandEncoder) Encode(cmd *Command) ([]byte, error) {
	jw := newWriter()
	cmd.MarshalEasyJSON(jw)
	return jw.BuildBytes()
}

// ProtobufCommandEncoder ...
type ProtobufCommandEncoder struct {
}

// NewProtobufCommandEncoder ...
func NewProtobufCommandEncoder() *ProtobufCommandEncoder {
	return &ProtobufCommandEncoder{}
}

// Encode ...
func (e *ProtobufCommandEncoder) Encode(cmd *Command) ([]byte, error) {
	commandBytes, err := cmd.Marshal()
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

// ParamsEncoder ...
type ParamsEncoder interface {
	Encode(request interface{}) ([]byte, error)
}

var _ ParamsEncoder = NewJSONParamsEncoder()

// JSONParamsEncoder ...
type JSONParamsEncoder struct{}

// NewJSONParamsEncoder ...
func NewJSONParamsEncoder() *JSONParamsEncoder {
	return &JSONParamsEncoder{}
}

// Encode ...
func (d *JSONParamsEncoder) Encode(r interface{}) ([]byte, error) {
	return fastJSON.Marshal(r)
}

var _ ParamsEncoder = NewProtobufParamsEncoder()

// ProtobufParamsEncoder ...
type ProtobufParamsEncoder struct{}

// NewProtobufParamsEncoder ...
func NewProtobufParamsEncoder() *ProtobufParamsEncoder {
	return &ProtobufParamsEncoder{}
}

// Encode ...
func (d *ProtobufParamsEncoder) Encode(r interface{}) ([]byte, error) {
	m, ok := r.(proto.Marshaler)
	if !ok {
		return nil, fmt.Errorf("can not marshal type %T to Protobuf", r)
	}
	return m.Marshal()
}
