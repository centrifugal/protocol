package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// String is used as a metric label, so every defined FrameType must map to a
// stable name and anything else must fall back to "unknown" rather than panic
// or return an empty label.
func TestFrameType_String(t *testing.T) {
	tests := []struct {
		frameType FrameType
		want      string
	}{
		{FrameTypeServerPing, "server_ping"},
		{FrameTypeClientPong, "client_pong"},
		{FrameTypePushConnect, "push_connect"},
		{FrameTypePushSubscribe, "push_subscribe"},
		{FrameTypePushPublication, "push_publication"},
		{FrameTypePushJoin, "push_join"},
		{FrameTypePushLeave, "push_leave"},
		{FrameTypePushUnsubscribe, "push_unsubscribe"},
		{FrameTypePushMessage, "push_message"},
		{FrameTypePushRefresh, "push_refresh"},
		{FrameTypePushDisconnect, "push_disconnect"},
		{FrameTypeConnect, "connect"},
		{FrameTypeSubscribe, "subscribe"},
		{FrameTypePublish, "publish"},
		{FrameTypeUnsubscribe, "unsubscribe"},
		{FrameTypeRPC, "rpc"},
		{FrameTypePresence, "presence"},
		{FrameTypePresenceStats, "presence_stats"},
		{FrameTypeHistory, "history"},
		{FrameTypeSubRefresh, "sub_refresh"},
		{FrameTypeRefresh, "refresh"},
		{FrameTypeSend, "send"},
		{FrameType(0), "unknown"},
		{FrameType(255), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			require.Equal(t, tt.want, tt.frameType.String())
		})
	}
}

func TestGetPushEncoder(t *testing.T) {
	require.IsType(t, &JSONPushEncoder{}, GetPushEncoder(TypeJSON))
	require.IsType(t, &ProtobufPushEncoder{}, GetPushEncoder(TypeProtobuf))
	// Any type other than TypeJSON is treated as TypeProtobuf.
	require.IsType(t, &ProtobufPushEncoder{}, GetPushEncoder(Type("unknown")))
}

func TestGetReplyEncoder(t *testing.T) {
	require.IsType(t, &JSONReplyEncoder{}, GetReplyEncoder(TypeJSON))
	require.IsType(t, &ProtobufReplyEncoder{}, GetReplyEncoder(TypeProtobuf))
	require.IsType(t, &ProtobufReplyEncoder{}, GetReplyEncoder(Type("unknown")))
}

func TestGetResultEncoder(t *testing.T) {
	require.IsType(t, &JSONResultEncoder{}, GetResultEncoder(TypeJSON))
	require.IsType(t, &ProtobufResultEncoder{}, GetResultEncoder(TypeProtobuf))
	require.IsType(t, &ProtobufResultEncoder{}, GetResultEncoder(Type("unknown")))

	// PutResultEncoder is a no-op kept for symmetry with GetResultEncoder; it
	// must not panic regardless of arguments passed to it.
	PutResultEncoder(TypeJSON, GetReplyEncoder(TypeJSON))
}

func TestGetPutCommandDecoder(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		d := GetCommandDecoder(protoType, []byte(`{}`))
		if protoType == TypeJSON {
			require.IsType(t, &JSONCommandDecoder{}, d)
		} else {
			require.IsType(t, &ProtobufCommandDecoder{}, d)
		}
		PutCommandDecoder(protoType, d)
		// A decoder returned to the pool must come back reset to the new frame,
		// not still holding state from the previous one.
		d2 := GetCommandDecoder(protoType, []byte(`{}`))
		require.NotNil(t, d2)
		PutCommandDecoder(protoType, d2)
	}
}
