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
	jsonPushEncoder     = NewJSONPushEncoder()
	protobufPushEncoder = NewProtobufPushEncoder()
)

// GetPushEncoder ...
func GetPushEncoder(protoType Type) PushEncoder {
	if protoType == TypeJSON {
		return jsonPushEncoder
	}
	return protobufPushEncoder
}

var (
	jsonReplyEncoder     = NewJSONReplyEncoder()
	protobufReplyEncoder = NewProtobufReplyEncoder()
)

// GetReplyEncoder ...
func GetReplyEncoder(protoType Type) ReplyEncoder {
	if protoType == TypeJSON {
		return jsonReplyEncoder
	}
	return protobufReplyEncoder
}

var (
	jsonDataEncoderPool     sync.Pool
	protobufDataEncoderPool sync.Pool
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
		return NewJSONCommandDecoder(data)
	}
	return NewProtobufCommandDecoder(data)
}

// PutCommandDecoder ...
func PutCommandDecoder(protoType Type, e CommandDecoder) {
	return
}

// GetResultEncoder ...
func GetResultEncoder(protoType Type) ResultEncoder {
	if protoType == TypeJSON {
		return NewJSONResultEncoder()
	}
	return NewProtobufResultEncoder()
}

// PutResultEncoder ...
func PutResultEncoder(protoType Type, e ReplyEncoder) {
	return
}

// GetParamsDecoder ...
func GetParamsDecoder(protoType Type) ParamsDecoder {
	if protoType == TypeJSON {
		return NewJSONParamsDecoder()
	}
	return NewProtobufParamsDecoder()
}

// PutParamsDecoder ...
func PutParamsDecoder(protoType Type, e ParamsDecoder) {
	return
}
