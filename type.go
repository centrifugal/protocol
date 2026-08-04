package protocol

import "sync"

// Type determines connection protocol type.
type Type string

const (
	// TypeJSON means JSON protocol.
	TypeJSON Type = "json"
	// TypeProtobuf means Protobuf protocol.
	TypeProtobuf Type = "protobuf"
)

// FrameType describes the type of a protocol frame. It's not a part of the wire
// format – it's used for observability, so that a server may count and measure
// frames it sends and receives per type.
type FrameType uint8

// Frame types of the protocol: a ping/pong pair, asynchronous server-to-client
// pushes and client-to-server commands.
const (
	FrameTypeServerPing FrameType = iota + 1
	FrameTypeClientPong

	FrameTypePushConnect
	FrameTypePushSubscribe
	FrameTypePushPublication
	FrameTypePushJoin
	FrameTypePushLeave
	FrameTypePushUnsubscribe
	FrameTypePushMessage
	FrameTypePushRefresh
	FrameTypePushDisconnect

	FrameTypeConnect
	FrameTypeSubscribe
	FrameTypePublish
	FrameTypeUnsubscribe
	FrameTypeRPC
	FrameTypePresence
	FrameTypePresenceStats
	FrameTypeHistory
	FrameTypeRefresh
	FrameTypeSubRefresh
	FrameTypeSend
)

// String returns a snake_case name of the frame type, suitable for using as a
// metric label. It returns "unknown" for frame types not defined in this package.
func (f FrameType) String() string {
	switch f {
	case FrameTypeServerPing:
		return "server_ping"
	case FrameTypeClientPong:
		return "client_pong"

	case FrameTypePushConnect:
		return "push_connect"
	case FrameTypePushSubscribe:
		return "push_subscribe"
	case FrameTypePushPublication:
		return "push_publication"
	case FrameTypePushJoin:
		return "push_join"
	case FrameTypePushLeave:
		return "push_leave"
	case FrameTypePushUnsubscribe:
		return "push_unsubscribe"
	case FrameTypePushMessage:
		return "push_message"
	case FrameTypePushRefresh:
		return "push_refresh"
	case FrameTypePushDisconnect:
		return "push_disconnect"

	case FrameTypeConnect:
		return "connect"
	case FrameTypeSubscribe:
		return "subscribe"
	case FrameTypePublish:
		return "publish"
	case FrameTypeUnsubscribe:
		return "unsubscribe"
	case FrameTypeRPC:
		return "rpc"
	case FrameTypePresence:
		return "presence"
	case FrameTypePresenceStats:
		return "presence_stats"
	case FrameTypeHistory:
		return "history"
	case FrameTypeSubRefresh:
		return "sub_refresh"
	case FrameTypeRefresh:
		return "refresh"
	case FrameTypeSend:
		return "send"

	default:
		return "unknown"
	}
}

// Default push encoders returned by GetPushEncoder. They are stateless, so a
// single instance per protocol type is shared by all connections.
var (
	DefaultJsonPushEncoder     = NewJSONPushEncoder()
	DefaultProtobufPushEncoder = NewProtobufPushEncoder()
)

// GetPushEncoder returns a PushEncoder for the given protocol type. Any type
// other than TypeJSON is treated as TypeProtobuf.
func GetPushEncoder(protoType Type) PushEncoder {
	if protoType == TypeJSON {
		return DefaultJsonPushEncoder
	}
	return DefaultProtobufPushEncoder
}

// Default reply encoders returned by GetReplyEncoder. They are stateless, so a
// single instance per protocol type is shared by all connections.
var (
	DefaultJsonReplyEncoder     = NewJSONReplyEncoder()
	DefaultProtobufReplyEncoder = NewProtobufReplyEncoder()
)

// GetReplyEncoder returns a ReplyEncoder for the given protocol type. Any type
// other than TypeJSON is treated as TypeProtobuf.
func GetReplyEncoder(protoType Type) ReplyEncoder {
	if protoType == TypeJSON {
		return DefaultJsonReplyEncoder
	}
	return DefaultProtobufReplyEncoder
}

var (
	jsonDataEncoderPool        sync.Pool
	protobufDataEncoderPool    sync.Pool
	jsonCommandDecoderPool     sync.Pool
	protobufCommandDecoderPool sync.Pool
)

// GetDataEncoder returns a DataEncoder for the given protocol type, taking it
// from a pool and resetting it. Return it with PutDataEncoder once the frame is
// built. Any type other than TypeJSON is treated as TypeProtobuf.
func GetDataEncoder(protoType Type) DataEncoder {
	if protoType == TypeJSON {
		e := jsonDataEncoderPool.Get()
		if e == nil {
			return NewJSONDataEncoder()
		}
		protoEncoder := e.(DataEncoder)
		protoEncoder.Reset()
		return protoEncoder
	}
	e := protobufDataEncoderPool.Get()
	if e == nil {
		return NewProtobufDataEncoder()
	}
	protoEncoder := e.(DataEncoder)
	protoEncoder.Reset()
	return protoEncoder
}

// PutDataEncoder returns a DataEncoder obtained with GetDataEncoder to the pool.
// The encoder must not be used after that, and neither must the slice returned
// by its FinishNoCopy method.
func PutDataEncoder(protoType Type, e DataEncoder) {
	if protoType == TypeJSON {
		jsonDataEncoderPool.Put(e)
		return
	}
	protobufDataEncoderPool.Put(e)
}

// GetCommandDecoder returns a CommandDecoder for the given protocol type, taking
// it from a pool and resetting it to the given frame. Return it with
// PutCommandDecoder once the frame is fully processed. Any type other than
// TypeJSON is treated as TypeProtobuf.
func GetCommandDecoder(protoType Type, data []byte) CommandDecoder {
	if protoType == TypeJSON {
		e := jsonCommandDecoderPool.Get()
		if e == nil {
			return NewJSONCommandDecoder(data)
		}
		commandDecoder := e.(*JSONCommandDecoder)
		_ = commandDecoder.Reset(data)
		return commandDecoder
	}
	e := protobufCommandDecoderPool.Get()
	if e == nil {
		return NewProtobufCommandDecoder(data)
	}
	commandDecoder := e.(*ProtobufCommandDecoder)
	_ = commandDecoder.Reset(data)
	return commandDecoder
}

// PutCommandDecoder returns a CommandDecoder obtained with GetCommandDecoder to
// the pool. The decoder must not be used after that.
func PutCommandDecoder(protoType Type, e CommandDecoder) {
	if protoType == TypeJSON {
		jsonCommandDecoderPool.Put(e)
		return
	}
	protobufCommandDecoderPool.Put(e)
}

// GetResultEncoder returns a ResultEncoder for the given protocol type. Any type
// other than TypeJSON is treated as TypeProtobuf.
func GetResultEncoder(protoType Type) ResultEncoder {
	if protoType == TypeJSON {
		return NewJSONResultEncoder()
	}
	return NewProtobufResultEncoder()
}

// PutResultEncoder is a no-op kept for symmetry with GetResultEncoder: result
// encoders are stateless, so there is nothing to return to a pool.
func PutResultEncoder(_ Type, _ ReplyEncoder) {}
