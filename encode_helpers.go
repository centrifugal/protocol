package protocol

// Some helpers to effectively construct Push messages by reusing byte buffers – reduces byte slice copies.

// newMessagePush returns initialized async push message.
func newMessagePush(data Raw) *Push {
	return &Push{
		Type: Push_MESSAGE,
		Data: data,
	}
}

// newPublicationPush returns initialized async publication message.
func newPublicationPush(ch string, data Raw) *Push {
	return &Push{
		Type:    Push_PUBLICATION,
		Channel: ch,
		Data:    data,
	}
}

// newJoinPush returns initialized async join message.
func newJoinPush(ch string, data Raw) *Push {
	return &Push{
		Type:    Push_JOIN,
		Channel: ch,
		Data:    data,
	}
}

// newLeavePush returns initialized async leave message.
func newLeavePush(ch string, data Raw) *Push {
	return &Push{
		Type:    Push_LEAVE,
		Channel: ch,
		Data:    data,
	}
}

// newUnsubscribePush returns initialized async unsubscribe message.
func newUnsubscribePush(ch string, data Raw) *Push {
	return &Push{
		Type:    Push_UNSUBSCRIBE,
		Channel: ch,
		Data:    data,
	}
}

// newSubscribePush returns initialized async subscribe message.
func newSubscribePush(ch string, data Raw) *Push {
	return &Push{
		Type:    Push_SUBSCRIBE,
		Channel: ch,
		Data:    data,
	}
}

// newConnectPush returns initialized async connect message.
func newConnectPush(data Raw) *Push {
	return &Push{
		Type: Push_CONNECT,
		Data: data,
	}
}

// newDisconnectPush returns initialized async disconnect message.
func newDisconnectPush(data Raw) *Push {
	return &Push{
		Type: Push_DISCONNECT,
		Data: data,
	}
}

// newRefreshPush returns initialized async refresh message.
func newRefreshPush(data Raw) *Push {
	return &Push{
		Type: Push_REFRESH,
		Data: data,
	}
}

// At the moment this is hardcoded to a value which should be enough for most our messages
// sent. If we will have a message with field names total size greater than this value then
// byte buffer won't be reused in JSON case (so need to take care of this to not loose
// performance at some point). Would be nice to add additional size for messages like
// Connect push which can have variable length Connect.Subs field.
const MaxJSONPushFieldsSize = 64

func EncodePublicationPush(protoType Type, channel string, message *Publication) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodePublication(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newPublicationPush(channel, data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodePublication(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newPublicationPush(channel, data)
	return pushEncoder.Encode(push)
}

func EncodeJoinPush(protoType Type, channel string, message *Join) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := GetPushEncoder(protoType)
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeJoin(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newJoinPush(channel, data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeJoin(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newJoinPush(channel, data)
	return pushEncoder.Encode(push)
}

func EncodeLeavePush(protoType Type, channel string, message *Leave) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeLeave(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newLeavePush(channel, data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeLeave(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newLeavePush(channel, data)
	return pushEncoder.Encode(push)
}

func EncodeMessagePush(protoType Type, message *Message) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeMessage(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newMessagePush(data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeMessage(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newMessagePush(data)
	return pushEncoder.Encode(push)
}

func EncodeUnsubscribePush(protoType Type, channel string, message *Unsubscribe) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeUnsubscribe(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newUnsubscribePush(channel, data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeUnsubscribe(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newUnsubscribePush(channel, data)
	return pushEncoder.Encode(push)
}

func EncodeSubscribePush(protoType Type, channel string, message *Subscribe) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeSubscribe(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newSubscribePush(channel, data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeSubscribe(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newSubscribePush(channel, data)
	return pushEncoder.Encode(push)
}

func EncodeDisconnectPush(protoType Type, message *Disconnect) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeDisconnect(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newDisconnectPush(data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeDisconnect(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newDisconnectPush(data)
	return pushEncoder.Encode(push)
}

func EncodeConnectPush(protoType Type, message *Connect) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeConnect(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newConnectPush(data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeConnect(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newConnectPush(data)
	return pushEncoder.Encode(push)
}

func EncodeRefreshPush(protoType Type, message *Refresh) ([]byte, error) {
	if protoType == TypeJSON {
		// Use branching here instead of GetPushEncoder(protoType) since otherwise
		// Go allocates more on heap (due to interface involved).
		pushEncoder := jsonPushEncoder
		size := message.SizeVT() + MaxJSONPushFieldsSize
		reuse := getByteBuffer(size)
		defer putByteBuffer(reuse)
		data, err := pushEncoder.EncodeRefresh(message, reuse.B)
		if err != nil {
			return nil, err
		}
		push := newRefreshPush(data)
		return pushEncoder.Encode(push)
	}
	pushEncoder := protobufPushEncoder
	size := message.SizeVT()
	reuse := getByteBuffer(size)
	defer putByteBuffer(reuse)
	data, err := pushEncoder.EncodeRefresh(message, reuse.B)
	if err != nil {
		return nil, err
	}
	push := newRefreshPush(data)
	return pushEncoder.Encode(push)
}
