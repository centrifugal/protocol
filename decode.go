package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/segmentio/encoding/json"
)

// PushDecoder ...
type PushDecoder interface {
	Decode([]byte) (*Push, error)
	DecodePublication([]byte) (*Publication, error)
	DecodeJoin([]byte) (*Join, error)
	DecodeLeave([]byte) (*Leave, error)
	DecodeMessage([]byte) (*Message, error)
	DecodeUnsubscribe([]byte) (*Unsubscribe, error)
	DecodeSubscribe([]byte) (*Subscribe, error)
	DecodeConnect([]byte) (*Connect, error)
	DecodeDisconnect([]byte) (*Disconnect, error)
}

var _ PushDecoder = NewJSONPushDecoder()
var _ PushDecoder = NewProtobufPushDecoder()

// JSONPushDecoder ...
type JSONPushDecoder struct{}

// NewJSONPushDecoder ...
func NewJSONPushDecoder() *JSONPushDecoder {
	return &JSONPushDecoder{}
}

// Decode ...
func (e *JSONPushDecoder) Decode(data []byte) (*Push, error) {
	var m Push
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodePublication ...
func (e *JSONPushDecoder) DecodePublication(data []byte) (*Publication, error) {
	var m Publication
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeJoin ...
func (e *JSONPushDecoder) DecodeJoin(data []byte) (*Join, error) {
	var m Join
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeLeave  ...
func (e *JSONPushDecoder) DecodeLeave(data []byte) (*Leave, error) {
	var m Leave
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeMessage ...
func (e *JSONPushDecoder) DecodeMessage(data []byte) (*Message, error) {
	var m Message
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeUnsubscribe ...
func (e *JSONPushDecoder) DecodeUnsubscribe(data []byte) (*Unsubscribe, error) {
	var m Unsubscribe
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeSubscribe ...
func (e *JSONPushDecoder) DecodeSubscribe(data []byte) (*Subscribe, error) {
	var m Subscribe
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeConnect ...
func (e *JSONPushDecoder) DecodeConnect(data []byte) (*Connect, error) {
	var m Connect
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeDisconnect ...
func (e *JSONPushDecoder) DecodeDisconnect(data []byte) (*Disconnect, error) {
	var m Disconnect
	_, err := json.Parse(data, &m, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ProtobufPushDecoder ...
type ProtobufPushDecoder struct {
}

// NewProtobufPushDecoder ...
func NewProtobufPushDecoder() *ProtobufPushDecoder {
	return &ProtobufPushDecoder{}
}

// Decode ...
func (e *ProtobufPushDecoder) Decode(data []byte) (*Push, error) {
	var m Push
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodePublication ...
func (e *ProtobufPushDecoder) DecodePublication(data []byte) (*Publication, error) {
	var m Publication
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeJoin ...
func (e *ProtobufPushDecoder) DecodeJoin(data []byte) (*Join, error) {
	var m Join
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeLeave  ...
func (e *ProtobufPushDecoder) DecodeLeave(data []byte) (*Leave, error) {
	var m Leave
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeMessage ...
func (e *ProtobufPushDecoder) DecodeMessage(data []byte) (*Message, error) {
	var m Message
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeUnsubscribe ...
func (e *ProtobufPushDecoder) DecodeUnsubscribe(data []byte) (*Unsubscribe, error) {
	var m Unsubscribe
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeSubscribe ...
func (e *ProtobufPushDecoder) DecodeSubscribe(data []byte) (*Subscribe, error) {
	var m Subscribe
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeConnect ...
func (e *ProtobufPushDecoder) DecodeConnect(data []byte) (*Connect, error) {
	var m Connect
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DecodeDisconnect ...
func (e *ProtobufPushDecoder) DecodeDisconnect(data []byte) (*Disconnect, error) {
	var m Disconnect
	err := m.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CommandDecoder ...
type CommandDecoder interface {
	Reset([]byte) error
	Decode() (*Command, error)
}

// JSONCommandDecoder ...
type JSONCommandDecoder struct {
	data            []byte
	messageCount    int
	prevNewLine     int
	numMessagesRead int
}

// NewJSONCommandDecoder ...
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

// Reset ...
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

// Decode ...
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

// ProtobufCommandDecoder ...
type ProtobufCommandDecoder struct {
	data   []byte
	offset int
}

// NewProtobufCommandDecoder ...
func NewProtobufCommandDecoder(data []byte) *ProtobufCommandDecoder {
	return &ProtobufCommandDecoder{
		data: data,
	}
}

// Reset ...
func (d *ProtobufCommandDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode ...
func (d *ProtobufCommandDecoder) Decode() (*Command, error) {
	if d.offset < len(d.data) {
		var c Command
		l, n := binary.Uvarint(d.data[d.offset:])
		if l == 0 || n <= 0 {
			return nil, io.EOF
		}
		from := d.offset + n
		to := d.offset + n + int(l)
		if to > 0 && to <= len(d.data) {
			cmdBytes := d.data[from:to]
			err := c.UnmarshalVT(cmdBytes)
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

// ParamsDecoder ...
type ParamsDecoder interface {
	DecodeConnect([]byte) (*ConnectRequest, error)
	DecodeRefresh([]byte) (*RefreshRequest, error)
	DecodeSubscribe([]byte) (*SubscribeRequest, error)
	DecodeSubRefresh([]byte) (*SubRefreshRequest, error)
	DecodeUnsubscribe([]byte) (*UnsubscribeRequest, error)
	DecodePublish([]byte) (*PublishRequest, error)
	DecodePresence([]byte) (*PresenceRequest, error)
	DecodePresenceStats([]byte) (*PresenceStatsRequest, error)
	DecodeHistory([]byte) (*HistoryRequest, error)
	DecodePing([]byte) (*PingRequest, error)
	DecodeRPC([]byte) (*RPCRequest, error)
	DecodeSend([]byte) (*SendRequest, error)
}

// JSONParamsDecoder ...
type JSONParamsDecoder struct{}

// NewJSONParamsDecoder ...
func NewJSONParamsDecoder() *JSONParamsDecoder {
	return &JSONParamsDecoder{}
}

// DecodeConnect ...
func (d *JSONParamsDecoder) DecodeConnect(data []byte) (*ConnectRequest, error) {
	var p ConnectRequest
	if data != nil {
		_, err := json.Parse(data, &p, json.ZeroCopy)
		if err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// DecodeRefresh ...
func (d *JSONParamsDecoder) DecodeRefresh(data []byte) (*RefreshRequest, error) {
	var p RefreshRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeSubscribe ...
func (d *JSONParamsDecoder) DecodeSubscribe(data []byte) (*SubscribeRequest, error) {
	var p SubscribeRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeSubRefresh ...
func (d *JSONParamsDecoder) DecodeSubRefresh(data []byte) (*SubRefreshRequest, error) {
	var p SubRefreshRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeUnsubscribe ...
func (d *JSONParamsDecoder) DecodeUnsubscribe(data []byte) (*UnsubscribeRequest, error) {
	var p UnsubscribeRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePublish ...
func (d *JSONParamsDecoder) DecodePublish(data []byte) (*PublishRequest, error) {
	var p PublishRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePresence ...
func (d *JSONParamsDecoder) DecodePresence(data []byte) (*PresenceRequest, error) {
	var p PresenceRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePresenceStats ...
func (d *JSONParamsDecoder) DecodePresenceStats(data []byte) (*PresenceStatsRequest, error) {
	var p PresenceStatsRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeHistory ...
func (d *JSONParamsDecoder) DecodeHistory(data []byte) (*HistoryRequest, error) {
	var p HistoryRequest
	_, err := json.Parse(data, &p, json.ZeroCopy)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePing ...
func (d *JSONParamsDecoder) DecodePing(data []byte) (*PingRequest, error) {
	var p PingRequest
	if data != nil {
		_, err := json.Parse(data, &p, json.ZeroCopy)
		if err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// DecodeRPC ...
func (d *JSONParamsDecoder) DecodeRPC(data []byte) (*RPCRequest, error) {
	var p RPCRequest
	if data != nil {
		_, err := json.Parse(data, &p, json.ZeroCopy)
		if err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// DecodeSend ...
func (d *JSONParamsDecoder) DecodeSend(data []byte) (*SendRequest, error) {
	var p SendRequest
	if data != nil {
		_, err := json.Parse(data, &p, json.ZeroCopy)
		if err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// ProtobufParamsDecoder ...
type ProtobufParamsDecoder struct{}

// NewProtobufParamsDecoder ...
func NewProtobufParamsDecoder() *ProtobufParamsDecoder {
	return &ProtobufParamsDecoder{}
}

// DecodeConnect ...
func (d *ProtobufParamsDecoder) DecodeConnect(data []byte) (*ConnectRequest, error) {
	var p ConnectRequest
	if data != nil {
		err := p.UnmarshalVT(data)
		if err != nil {
			return nil, err
		}
	}
	return &p, nil
}

// DecodeRefresh ...
func (d *ProtobufParamsDecoder) DecodeRefresh(data []byte) (*RefreshRequest, error) {
	var p RefreshRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeSubscribe ...
func (d *ProtobufParamsDecoder) DecodeSubscribe(data []byte) (*SubscribeRequest, error) {
	var p SubscribeRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeSubRefresh ...
func (d *ProtobufParamsDecoder) DecodeSubRefresh(data []byte) (*SubRefreshRequest, error) {
	var p SubRefreshRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeUnsubscribe ...
func (d *ProtobufParamsDecoder) DecodeUnsubscribe(data []byte) (*UnsubscribeRequest, error) {
	var p UnsubscribeRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePublish ...
func (d *ProtobufParamsDecoder) DecodePublish(data []byte) (*PublishRequest, error) {
	var p PublishRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePresence ...
func (d *ProtobufParamsDecoder) DecodePresence(data []byte) (*PresenceRequest, error) {
	var p PresenceRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePresenceStats ...
func (d *ProtobufParamsDecoder) DecodePresenceStats(data []byte) (*PresenceStatsRequest, error) {
	var p PresenceStatsRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeHistory ...
func (d *ProtobufParamsDecoder) DecodeHistory(data []byte) (*HistoryRequest, error) {
	var p HistoryRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodePing ...
func (d *ProtobufParamsDecoder) DecodePing(data []byte) (*PingRequest, error) {
	var p PingRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeRPC ...
func (d *ProtobufParamsDecoder) DecodeRPC(data []byte) (*RPCRequest, error) {
	var p RPCRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DecodeSend ...
func (d *ProtobufParamsDecoder) DecodeSend(data []byte) (*SendRequest, error) {
	var p SendRequest
	err := p.UnmarshalVT(data)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ReplyDecoder ...
type ReplyDecoder interface {
	Reset([]byte) error
	Decode() (*Reply, error)
}

var _ ReplyDecoder = NewJSONReplyDecoder(nil)

// JSONReplyDecoder ...
type JSONReplyDecoder struct {
	decoder *json.Decoder
}

// NewJSONReplyDecoder ...
func NewJSONReplyDecoder(data []byte) *JSONReplyDecoder {
	return &JSONReplyDecoder{
		decoder: json.NewDecoder(bytes.NewReader(data)),
	}
}

// Reset ...
func (d *JSONReplyDecoder) Reset(data []byte) error {
	d.decoder = json.NewDecoder(bytes.NewReader(data))
	return nil
}

// Decode ...
func (d *JSONReplyDecoder) Decode() (*Reply, error) {
	var c Reply
	err := d.decoder.Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

var _ ReplyDecoder = NewProtobufReplyDecoder(nil)

// ProtobufReplyDecoder ...
type ProtobufReplyDecoder struct {
	data   []byte
	offset int
}

// NewProtobufReplyDecoder ...
func NewProtobufReplyDecoder(data []byte) *ProtobufReplyDecoder {
	return &ProtobufReplyDecoder{
		data: data,
	}
}

// Reset ...
func (d *ProtobufReplyDecoder) Reset(data []byte) error {
	d.data = data
	d.offset = 0
	return nil
}

// Decode ...
func (d *ProtobufReplyDecoder) Decode() (*Reply, error) {
	if d.offset < len(d.data) {
		var c Reply
		l, n := binary.Uvarint(d.data[d.offset:])
		replyBytes := d.data[d.offset+n : d.offset+n+int(l)]
		err := c.UnmarshalVT(replyBytes)
		if err != nil {
			return nil, err
		}
		d.offset = d.offset + n + int(l)
		return &c, nil
	}
	return nil, io.EOF
}

// ResultDecoder ...
type ResultDecoder interface {
	Decode([]byte, interface{}) error
}

var _ ResultDecoder = NewJSONResultDecoder()

// JSONResultDecoder ...
type JSONResultDecoder struct{}

// NewJSONResultDecoder ...
func NewJSONResultDecoder() *JSONResultDecoder {
	return &JSONResultDecoder{}
}

// Decode ...
func (e *JSONResultDecoder) Decode(data []byte, dst interface{}) error {
	_, err := json.Parse(data, dst, json.ZeroCopy)
	return err
}

var _ ResultDecoder = NewProtobufResultDecoder()

// ProtobufResultDecoder ...
type ProtobufResultDecoder struct{}

// NewProtobufResultDecoder ...
func NewProtobufResultDecoder() *ProtobufResultDecoder {
	return &ProtobufResultDecoder{}
}

type vtUnmarshaler interface {
	UnmarshalVT([]byte) error
}

// Decode ...
func (e *ProtobufResultDecoder) Decode(data []byte, dst interface{}) error {
	m, ok := dst.(vtUnmarshaler)
	if !ok {
		return fmt.Errorf("can not unmarshal type from Protobuf")
	}
	return m.UnmarshalVT(data)
}
