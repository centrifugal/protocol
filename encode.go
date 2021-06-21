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
	Encode(*Push, ...[]byte) ([]byte, error)
	EncodeMessage(*Message, ...[]byte) ([]byte, error)
	EncodePublication(*Publication, ...[]byte) ([]byte, error)
	EncodeJoin(*Join, ...[]byte) ([]byte, error)
	EncodeLeave(*Leave, ...[]byte) ([]byte, error)
	EncodeUnsubscribe(*Unsubscribe, ...[]byte) ([]byte, error)
	EncodeSubscribe(*Subscribe, ...[]byte) ([]byte, error)
	EncodeConnect(*Connect, ...[]byte) ([]byte, error)
	EncodeDisconnect(*Disconnect, ...[]byte) ([]byte, error)
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
func (e *JSONPushEncoder) Encode(message *Push, reuse ...[]byte) ([]byte, error) {
	// Check data is valid JSON.
	if err := isValidJSON(message.Data); err != nil {
		return nil, err
	}
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodePublication ...
func (e *JSONPushEncoder) EncodePublication(message *Publication, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeMessage ...
func (e *JSONPushEncoder) EncodeMessage(message *Message, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeJoin ...
func (e *JSONPushEncoder) EncodeJoin(message *Join, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeLeave ...
func (e *JSONPushEncoder) EncodeLeave(message *Leave, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeUnsubscribe ...
func (e *JSONPushEncoder) EncodeUnsubscribe(message *Unsubscribe, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeSubscribe ...
func (e *JSONPushEncoder) EncodeSubscribe(message *Subscribe, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeConnect ...
func (e *JSONPushEncoder) EncodeConnect(message *Connect, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeDisconnect ...
func (e *JSONPushEncoder) EncodeDisconnect(message *Disconnect, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	message.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// ProtobufPushEncoder ...
type ProtobufPushEncoder struct {
}

// NewProtobufPushEncoder ...
func NewProtobufPushEncoder() *ProtobufPushEncoder {
	return &ProtobufPushEncoder{}
}

// Encode ...
func (e *ProtobufPushEncoder) Encode(message *Push, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodePublication ...
func (e *ProtobufPushEncoder) EncodePublication(message *Publication, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeMessage ...
func (e *ProtobufPushEncoder) EncodeMessage(message *Message, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeJoin ...
func (e *ProtobufPushEncoder) EncodeJoin(message *Join, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeLeave ...
func (e *ProtobufPushEncoder) EncodeLeave(message *Leave, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeUnsubscribe ...
func (e *ProtobufPushEncoder) EncodeUnsubscribe(message *Unsubscribe, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeSubscribe ...
func (e *ProtobufPushEncoder) EncodeSubscribe(message *Subscribe, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeConnect ...
func (e *ProtobufPushEncoder) EncodeConnect(message *Connect, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// EncodeDisconnect ...
func (e *ProtobufPushEncoder) EncodeDisconnect(message *Disconnect, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := message.MarshalTo(ret)
		return ret[:n], err
	}
	return message.Marshal()
}

// ReplyEncoder ...
type ReplyEncoder interface {
	Encode(*Reply, ...[]byte) ([]byte, error)
}

// JSONReplyEncoder ...
type JSONReplyEncoder struct{}

// NewJSONReplyEncoder ...
func NewJSONReplyEncoder() *JSONReplyEncoder {
	return &JSONReplyEncoder{}
}

// Encode ...
func (e *JSONReplyEncoder) Encode(r *Reply, reuse ...[]byte) ([]byte, error) {
	if r.Id != 0 {
		// Only check command result reply. Push reply JSON validation is done in PushEncoder.
		if err := isValidJSON(r.Result); err != nil {
			return nil, err
		}
	}
	jw := newWriter()
	r.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// ProtobufReplyEncoder ...
type ProtobufReplyEncoder struct{}

// NewProtobufReplyEncoder ...
func NewProtobufReplyEncoder() *ProtobufReplyEncoder {
	return &ProtobufReplyEncoder{}
}

// Encode ...
func (e *ProtobufReplyEncoder) Encode(r *Reply, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := r.MarshalTo(ret)
		return ret[:n], err
	}
	return r.Marshal()
}

// DataEncoder ...
type DataEncoder interface {
	Reset()
	Encode([]byte) error
	Size() int
	Finish(...[]byte) []byte
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

// Size ...
func (e *JSONDataEncoder) Size() int {
	return e.buffer.Len()
}

// Finish ...
func (e *JSONDataEncoder) Finish(reuse ...[]byte) []byte {
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

// Size ...
func (e *ProtobufDataEncoder) Size() int {
	return e.buffer.Len()
}

// Finish ...
func (e *ProtobufDataEncoder) Finish(reuse ...[]byte) []byte {
	data := e.buffer.Bytes()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy
}

// ResultEncoder ...
type ResultEncoder interface {
	EncodeConnectResult(*ConnectResult, ...[]byte) ([]byte, error)
	EncodeRefreshResult(*RefreshResult, ...[]byte) ([]byte, error)
	EncodeSubscribeResult(*SubscribeResult, ...[]byte) ([]byte, error)
	EncodeSubRefreshResult(*SubRefreshResult, ...[]byte) ([]byte, error)
	EncodeUnsubscribeResult(*UnsubscribeResult, ...[]byte) ([]byte, error)
	EncodePublishResult(*PublishResult, ...[]byte) ([]byte, error)
	EncodePresenceResult(*PresenceResult, ...[]byte) ([]byte, error)
	EncodePresenceStatsResult(*PresenceStatsResult, ...[]byte) ([]byte, error)
	EncodeHistoryResult(*HistoryResult, ...[]byte) ([]byte, error)
	EncodePingResult(*PingResult, ...[]byte) ([]byte, error)
	EncodeRPCResult(*RPCResult, ...[]byte) ([]byte, error)
}

// JSONResultEncoder ...
type JSONResultEncoder struct{}

// NewJSONResultEncoder ...
func NewJSONResultEncoder() *JSONResultEncoder {
	return &JSONResultEncoder{}
}

// EncodeConnectResult ...
func (e *JSONResultEncoder) EncodeConnectResult(res *ConnectResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeRefreshResult ...
func (e *JSONResultEncoder) EncodeRefreshResult(res *RefreshResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeSubscribeResult ...
func (e *JSONResultEncoder) EncodeSubscribeResult(res *SubscribeResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeSubRefreshResult ...
func (e *JSONResultEncoder) EncodeSubRefreshResult(res *SubRefreshResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeUnsubscribeResult ...
func (e *JSONResultEncoder) EncodeUnsubscribeResult(res *UnsubscribeResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodePublishResult ...
func (e *JSONResultEncoder) EncodePublishResult(res *PublishResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodePresenceResult ...
func (e *JSONResultEncoder) EncodePresenceResult(res *PresenceResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodePresenceStatsResult ...
func (e *JSONResultEncoder) EncodePresenceStatsResult(res *PresenceStatsResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeHistoryResult ...
func (e *JSONResultEncoder) EncodeHistoryResult(res *HistoryResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodePingResult ...
func (e *JSONResultEncoder) EncodePingResult(res *PingResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// EncodeRPCResult ...
func (e *JSONResultEncoder) EncodeRPCResult(res *RPCResult, reuse ...[]byte) ([]byte, error) {
	jw := newWriter()
	res.MarshalEasyJSON(jw)
	return jw.BuildBytes(reuse...)
}

// ProtobufResultEncoder ...
type ProtobufResultEncoder struct{}

// NewProtobufResultEncoder ...
func NewProtobufResultEncoder() *ProtobufResultEncoder {
	return &ProtobufResultEncoder{}
}

// EncodeConnectResult ...
func (e *ProtobufResultEncoder) EncodeConnectResult(res *ConnectResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodeRefreshResult ...
func (e *ProtobufResultEncoder) EncodeRefreshResult(res *RefreshResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodeSubscribeResult ...
func (e *ProtobufResultEncoder) EncodeSubscribeResult(res *SubscribeResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodeSubRefreshResult ...
func (e *ProtobufResultEncoder) EncodeSubRefreshResult(res *SubRefreshResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodeUnsubscribeResult ...
func (e *ProtobufResultEncoder) EncodeUnsubscribeResult(res *UnsubscribeResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodePublishResult ...
func (e *ProtobufResultEncoder) EncodePublishResult(res *PublishResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodePresenceResult ...
func (e *ProtobufResultEncoder) EncodePresenceResult(res *PresenceResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodePresenceStatsResult ...
func (e *ProtobufResultEncoder) EncodePresenceStatsResult(res *PresenceStatsResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodeHistoryResult ...
func (e *ProtobufResultEncoder) EncodeHistoryResult(res *HistoryResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodePingResult ...
func (e *ProtobufResultEncoder) EncodePingResult(res *PingResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
	return res.Marshal()
}

// EncodeRPCResult ...
func (e *ProtobufResultEncoder) EncodeRPCResult(res *RPCResult, reuse ...[]byte) ([]byte, error) {
	if len(reuse) == 1 {
		ret := reuse[0][:0]
		n, err := res.MarshalTo(ret)
		return ret[:n], err
	}
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
