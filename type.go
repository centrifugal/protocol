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

var (
	DefaultJsonPushEncoder     = NewJSONPushEncoder()
	DefaultProtobufPushEncoder = NewProtobufPushEncoder()
)

// GetPushEncoder ...
func GetPushEncoder(protoType Type) PushEncoder {
	if protoType == TypeJSON {
		return DefaultJsonPushEncoder
	}
	return DefaultProtobufPushEncoder
}

var (
	DefaultJsonReplyEncoder     = NewJSONReplyEncoder()
	DefaultProtobufReplyEncoder = NewProtobufReplyEncoder()
)

// GetReplyEncoder ...
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

// GetDataEncoder ...
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

// PutDataEncoder ...
func PutDataEncoder(protoType Type, e DataEncoder) {
	if protoType == TypeJSON {
		jsonDataEncoderPool.Put(e)
		return
	}
	protobufDataEncoderPool.Put(e)
}

// GetCommandDecoder ...
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

// PutCommandDecoder ...
func PutCommandDecoder(protoType Type, e CommandDecoder) {
	if protoType == TypeJSON {
		jsonCommandDecoderPool.Put(e)
		return
	}
	protobufCommandDecoderPool.Put(e)
}

// GetResultEncoder ...
func GetResultEncoder(protoType Type) ResultEncoder {
	if protoType == TypeJSON {
		return NewJSONResultEncoder()
	}
	return NewProtobufResultEncoder()
}

// PutResultEncoder ...
func PutResultEncoder(_ Type, _ ReplyEncoder) {}
