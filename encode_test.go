package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	fastJSON "github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
)

func TestEncodeEasyJson(t *testing.T) {
	data := []byte(`{
  "num": "1\n"

}
`)
	pushEncoder := NewJSONPushEncoder()
	pub := &Publication{
		Data: data,
	}
	res, err := pushEncoder.EncodePublication(pub)
	require.NoError(t, err)
	require.Len(t, strings.Split(string(res), "\n"), 1)
}

func TestEncodeStd(t *testing.T) {
	data := []byte(`{
  "num": "1\n"

}
`)
	pub := &Publication{
		Data: data,
	}

	res, err := json.Marshal(pub)
	require.NoError(t, err)
	require.Len(t, strings.Split(string(res), "\n"), 1)
}

func TestEncodeFast(t *testing.T) {
	data := []byte(`{
  "num": "1\n"

}
`)
	pub := &Publication{
		Data: data,
	}

	res, err := fastJSON.Marshal(pub)
	require.NoError(t, err)
	require.Len(t, strings.Split(string(res), "\n"), 1)
}

// protoMessage is the part of a generated message this file needs to decode a
// payload an encoder produced, whichever protocol Type it was encoded with.
type protoMessage interface {
	UnmarshalVT([]byte) error
}

// decodeInto parses data produced by a PushEncoder or a ResultEncoder back into
// msg, using the codec matching protoType.
func decodeInto(t *testing.T, protoType Type, data []byte, msg protoMessage) {
	t.Helper()
	if protoType == TypeJSON {
		require.NoError(t, json.Unmarshal(data, msg))
		return
	}
	require.NoError(t, msg.UnmarshalVT(data))
}

func TestPushEncoder_Encode(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		t.Run(string(protoType), func(t *testing.T) {
			push := &Push{
				Channel: "test",
				Pub: &Publication{
					Data:   Raw(`{"input":"hello"}`),
					Offset: 42,
					Time:   1700000000000,
					Delta:  true,
					Info:   &ClientInfo{User: "user", Client: "client"},
				},
			}
			data, err := GetPushEncoder(protoType).Encode(push)
			require.NoError(t, err)

			var got Push
			decodeInto(t, protoType, data, &got)
			require.Equal(t, "test", got.Channel)
			require.NotNil(t, got.Pub)
			require.JSONEq(t, `{"input":"hello"}`, string(got.Pub.Data))
			require.Equal(t, uint64(42), got.Pub.Offset)
			require.Equal(t, int64(1700000000000), got.Pub.Time)
			require.True(t, got.Pub.Delta)
			require.NotNil(t, got.Pub.Info)
			require.Equal(t, "user", got.Pub.Info.User)
		})
	}
}

// TestJSONPushEncoder_Encode_InvalidPayload makes sure a Push carrying a
// payload which is not valid JSON is rejected instead of being sent as a frame
// no client can parse. Raw payloads are passed through as is, so this is the
// only place the mistake can be caught.
func TestJSONPushEncoder_Encode_InvalidPayload(t *testing.T) {
	push := &Push{Channel: "test", Pub: &Publication{Data: Raw(`{"broken":`)}}
	_, err := NewJSONPushEncoder().Encode(push)
	require.ErrorIs(t, err, errInvalidJSON)
}

// pushPart is one of the Push parts a PushEncoder encodes separately from the
// Push envelope, so that a payload encoded once can be reused for all
// subscribers of a channel.
type pushPart struct {
	name string
	// encode encodes the part, passing reuse through to the encoder.
	encode func(e PushEncoder, reuse ...[]byte) ([]byte, error)
	// check decodes an encoded part and asserts it round-tripped.
	check func(t *testing.T, protoType Type, data []byte)
}

func pushParts() []pushPart {
	return []pushPart{
		{
			name: "publication",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodePublication(&Publication{Data: Raw(`{"a":1}`), Offset: 7, Channel: "ch"}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Publication
				decodeInto(t, protoType, data, &got)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
				require.Equal(t, uint64(7), got.Offset)
				require.Equal(t, "ch", got.Channel)
			},
		},
		{
			name: "message",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeMessage(&Message{Data: Raw(`{"a":1}`)}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Message
				decodeInto(t, protoType, data, &got)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
			},
		},
		{
			name: "join",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeJoin(&Join{Info: &ClientInfo{User: "user", Client: "client"}}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Join
				decodeInto(t, protoType, data, &got)
				require.NotNil(t, got.Info)
				require.Equal(t, "user", got.Info.User)
				require.Equal(t, "client", got.Info.Client)
			},
		},
		{
			name: "leave",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeLeave(&Leave{Info: &ClientInfo{User: "user"}}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Leave
				decodeInto(t, protoType, data, &got)
				require.NotNil(t, got.Info)
				require.Equal(t, "user", got.Info.User)
			},
		},
		{
			name: "unsubscribe",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeUnsubscribe(&Unsubscribe{Code: 2500, Reason: "bye"}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Unsubscribe
				decodeInto(t, protoType, data, &got)
				require.Equal(t, uint32(2500), got.Code)
				require.Equal(t, "bye", got.Reason)
			},
		},
		{
			name: "subscribe",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeSubscribe(&Subscribe{Recoverable: true, Epoch: "xyz", Offset: 11, Data: Raw(`{"a":1}`)}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Subscribe
				decodeInto(t, protoType, data, &got)
				require.True(t, got.Recoverable)
				require.Equal(t, "xyz", got.Epoch)
				require.Equal(t, uint64(11), got.Offset)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
			},
		},
		{
			name: "connect",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeConnect(&Connect{Client: "client", Version: "1.0.0", Ttl: 60, Data: Raw(`{"a":1}`)}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Connect
				decodeInto(t, protoType, data, &got)
				require.Equal(t, "client", got.Client)
				require.Equal(t, "1.0.0", got.Version)
				require.Equal(t, uint32(60), got.Ttl)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
			},
		},
		{
			name: "disconnect",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeDisconnect(&Disconnect{Code: 3000, Reason: "shutdown", Reconnect: true}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Disconnect
				decodeInto(t, protoType, data, &got)
				require.Equal(t, uint32(3000), got.Code)
				require.Equal(t, "shutdown", got.Reason)
				require.True(t, got.Reconnect)
			},
		},
		{
			name: "refresh",
			encode: func(e PushEncoder, reuse ...[]byte) ([]byte, error) {
				return e.EncodeRefresh(&Refresh{Expires: true, Ttl: 120}, reuse...)
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got Refresh
				decodeInto(t, protoType, data, &got)
				require.True(t, got.Expires)
				require.Equal(t, uint32(120), got.Ttl)
			},
		},
	}
}

func TestPushEncoder_Parts(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		for _, part := range pushParts() {
			t.Run(string(protoType)+"/"+part.name, func(t *testing.T) {
				encoder := GetPushEncoder(protoType)
				data, err := part.encode(encoder)
				require.NoError(t, err)
				part.check(t, protoType, data)

				// A reuse buffer too small to hold the result must not be used,
				// and must not change what is encoded.
				small, err := part.encode(encoder, make([]byte, 0, 1))
				require.NoError(t, err)
				require.Equal(t, data, small)
			})
		}
	}
}

// TestPushEncoder_PartsReuseBuffer checks the documented reuse contract: given a
// buffer large enough for the result, an encoder writes into it instead of
// allocating, and produces exactly what it would have produced without one.
func TestPushEncoder_PartsReuseBuffer(t *testing.T) {
	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		for _, part := range pushParts() {
			t.Run(string(protoType)+"/"+part.name, func(t *testing.T) {
				encoder := GetPushEncoder(protoType)
				want, err := part.encode(encoder)
				require.NoError(t, err)
				require.NotEmpty(t, want)

				buf := make([]byte, 512)
				got, err := part.encode(encoder, buf)
				require.NoError(t, err)
				require.Equal(t, want, got)
				// The caller's buffer is the one which was written into.
				require.Equal(t, want, buf[:len(got)])
				part.check(t, protoType, got)
			})
		}
	}
}

func TestResultEncoder(t *testing.T) {
	cases := []struct {
		name   string
		encode func(e ResultEncoder) ([]byte, error)
		check  func(t *testing.T, protoType Type, data []byte)
	}{
		{
			name: "connect",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeConnectResult(&ConnectResult{Client: "client", Version: "1.0.0", Expires: true, Ttl: 60, Data: Raw(`{"a":1}`)})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got ConnectResult
				decodeInto(t, protoType, data, &got)
				require.Equal(t, "client", got.Client)
				require.Equal(t, "1.0.0", got.Version)
				require.True(t, got.Expires)
				require.Equal(t, uint32(60), got.Ttl)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
			},
		},
		{
			name: "refresh",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeRefreshResult(&RefreshResult{Client: "client", Expires: true, Ttl: 30})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got RefreshResult
				decodeInto(t, protoType, data, &got)
				require.Equal(t, "client", got.Client)
				require.True(t, got.Expires)
				require.Equal(t, uint32(30), got.Ttl)
			},
		},
		{
			name: "subscribe",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeSubscribeResult(&SubscribeResult{
					Recoverable:  true,
					Epoch:        "xyz",
					Offset:       5,
					Publications: []*Publication{{Data: Raw(`{"a":1}`), Offset: 5}},
				})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got SubscribeResult
				decodeInto(t, protoType, data, &got)
				require.True(t, got.Recoverable)
				require.Equal(t, "xyz", got.Epoch)
				require.Equal(t, uint64(5), got.Offset)
				require.Len(t, got.Publications, 1)
				require.JSONEq(t, `{"a":1}`, string(got.Publications[0].Data))
			},
		},
		{
			name: "sub_refresh",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeSubRefreshResult(&SubRefreshResult{Expires: true, Ttl: 15})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got SubRefreshResult
				decodeInto(t, protoType, data, &got)
				require.True(t, got.Expires)
				require.Equal(t, uint32(15), got.Ttl)
			},
		},
		{
			name: "unsubscribe",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeUnsubscribeResult(&UnsubscribeResult{})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got UnsubscribeResult
				decodeInto(t, protoType, data, &got)
			},
		},
		{
			name: "publish",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodePublishResult(&PublishResult{})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got PublishResult
				decodeInto(t, protoType, data, &got)
			},
		},
		{
			name: "presence",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodePresenceResult(&PresenceResult{
					Presence: map[string]*ClientInfo{"client": {User: "user", Client: "client"}},
				})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got PresenceResult
				decodeInto(t, protoType, data, &got)
				require.Len(t, got.Presence, 1)
				require.Equal(t, "user", got.Presence["client"].User)
			},
		},
		{
			name: "presence_stats",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodePresenceStatsResult(&PresenceStatsResult{NumClients: 3, NumUsers: 2})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got PresenceStatsResult
				decodeInto(t, protoType, data, &got)
				require.Equal(t, uint32(3), got.NumClients)
				require.Equal(t, uint32(2), got.NumUsers)
			},
		},
		{
			name: "history",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeHistoryResult(&HistoryResult{
					Publications: []*Publication{{Data: Raw(`{"a":1}`), Offset: 1}},
					Epoch:        "xyz",
					Offset:       1,
				})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got HistoryResult
				decodeInto(t, protoType, data, &got)
				require.Len(t, got.Publications, 1)
				require.JSONEq(t, `{"a":1}`, string(got.Publications[0].Data))
				require.Equal(t, "xyz", got.Epoch)
				require.Equal(t, uint64(1), got.Offset)
			},
		},
		{
			name: "ping",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodePingResult(&PingResult{})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got PingResult
				decodeInto(t, protoType, data, &got)
			},
		},
		{
			name: "rpc",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodeRPCResult(&RPCResult{Data: Raw(`{"a":1}`)})
			},
			check: func(t *testing.T, protoType Type, data []byte) {
				var got RPCResult
				decodeInto(t, protoType, data, &got)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
			},
		},
	}

	for _, protoType := range []Type{TypeJSON, TypeProtobuf} {
		for _, tc := range cases {
			t.Run(string(protoType)+"/"+tc.name, func(t *testing.T) {
				data, err := tc.encode(GetResultEncoder(protoType))
				require.NoError(t, err)
				tc.check(t, protoType, data)
			})
		}
	}
}

// TestJSONDataEncoder_FinishNoCopy checks FinishNoCopy returns the same frame
// Finish does, just without copying it out of the encoder buffer.
func TestJSONDataEncoder_FinishNoCopy(t *testing.T) {
	e := NewJSONDataEncoder()
	require.NoError(t, e.Encode([]byte(`{"id":1}`)))
	require.NoError(t, e.Encode([]byte(`{"id":2}`)))
	require.Equal(t, []byte("{\"id\":1}\n{\"id\":2}"), e.FinishNoCopy())
	require.Equal(t, e.Finish(), e.FinishNoCopy())
}
