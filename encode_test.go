package protocol

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/centrifugal/protocol/cfjson"
)

// Tests of the encoders: commands (the client side), replies, pushes and
// results (the server side), and the data encoders which put encoded
// messages into a frame.

func TestCommandEncoder(t *testing.T) {
	cmd := &Command{Id: 1, Subscribe: &SubscribeRequest{Channel: "news", Recover: true, Epoch: "xyz", Offset: 5}}
	forEachType(t, func(t *testing.T, protoType Type) {
		got := readCommands(t, GetCommandDecoder(protoType, encodeCommand(t, protoType, cmd)))
		require.Len(t, got, 1)
		require.Equal(t, cmd.Subscribe.Channel, got[0].Subscribe.Channel)
		require.Equal(t, cmd.Subscribe.Offset, got[0].Subscribe.Offset)
	})
	data, err := NewJSONCommandEncoder().Encode(cmd)
	require.NoError(t, err)
	require.Equal(t, `{"id":1,"subscribe":{"channel":"news","recover":true,"epoch":"xyz","offset":5}}`, string(data))
}

func TestReplyEncoder(t *testing.T) {
	reply := &Reply{Id: 1, Connect: &ConnectResult{Client: "client", Version: "6.0.0", Ping: 25, Pong: true}}
	forEachType(t, func(t *testing.T, protoType Type) {
		data, err := GetReplyEncoder(protoType).Encode(reply)
		require.NoError(t, err)
		var got Reply
		decodeInto(t, protoType, data, &got)
		require.Equal(t, uint32(1), got.Id)
		require.Equal(t, "client", got.Connect.Client)
		require.Equal(t, uint32(25), got.Connect.Ping)
	})
	data, err := NewJSONReplyEncoder().Encode(reply)
	require.NoError(t, err)
	require.Equal(t, `{"id":1,"connect":{"client":"client","version":"6.0.0","ping":25,"pong":true}}`, string(data))
}

func TestPushEncoder(t *testing.T) {
	push := &Push{Channel: "test", Pub: &Publication{
		Data: Raw(`{"input":"hello"}`), Offset: 42, Time: 1700000000000, Delta: true,
		Info: &ClientInfo{User: "user", Client: "client"},
	}}
	forEachType(t, func(t *testing.T, protoType Type) {
		data, err := GetPushEncoder(protoType).Encode(push)
		require.NoError(t, err)
		var got Push
		decodeInto(t, protoType, data, &got)
		require.Equal(t, "test", got.Channel)
		require.JSONEq(t, `{"input":"hello"}`, string(got.Pub.Data))
		require.Equal(t, uint64(42), got.Pub.Offset)
		require.Equal(t, int64(1700000000000), got.Pub.Time)
		require.True(t, got.Pub.Delta)
		require.Equal(t, "user", got.Pub.Info.User)
	})
}

// pushParts are the parts of a Push a PushEncoder encodes on their own, so
// that a part encoded once serves all subscribers of a channel.
var pushParts = []struct {
	name   string
	encode func(e PushEncoder, reuse ...[]byte) ([]byte, error)
	check  func(t *testing.T, protoType Type, data []byte)
}{
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

func TestPushEncoder_Parts(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		for _, part := range pushParts {
			t.Run(part.name, func(t *testing.T) {
				encoder := GetPushEncoder(protoType)
				want, err := part.encode(encoder)
				require.NoError(t, err)
				part.check(t, protoType, want)

				// A reuse buffer too small for the result is not used.
				small := make([]byte, 0, 1)
				got, err := part.encode(encoder, small)
				require.NoError(t, err)
				require.Equal(t, want, got)
				require.Equal(t, 1, cap(small))

				// One large enough is written into instead of allocating.
				buf := make([]byte, 512)
				got, err = part.encode(encoder, buf)
				require.NoError(t, err)
				require.Equal(t, want, got)
				require.Same(t, &buf[0], &got[0])
			})
		}
	})
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
					Recoverable: true, Epoch: "xyz", Offset: 5,
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
			name:   "unsubscribe",
			encode: func(e ResultEncoder) ([]byte, error) { return e.EncodeUnsubscribeResult(&UnsubscribeResult{}) },
			check:  func(t *testing.T, protoType Type, data []byte) { decodeInto(t, protoType, data, &UnsubscribeResult{}) },
		},
		{
			name:   "publish",
			encode: func(e ResultEncoder) ([]byte, error) { return e.EncodePublishResult(&PublishResult{}) },
			check:  func(t *testing.T, protoType Type, data []byte) { decodeInto(t, protoType, data, &PublishResult{}) },
		},
		{
			name: "presence",
			encode: func(e ResultEncoder) ([]byte, error) {
				return e.EncodePresenceResult(&PresenceResult{Presence: map[string]*ClientInfo{"client": {User: "user", Client: "client"}}})
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
				return e.EncodeHistoryResult(&HistoryResult{Publications: []*Publication{{Data: Raw(`{"a":1}`), Offset: 1}}, Epoch: "xyz", Offset: 1})
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
			name:   "ping",
			encode: func(e ResultEncoder) ([]byte, error) { return e.EncodePingResult(&PingResult{}) },
			check:  func(t *testing.T, protoType Type, data []byte) { decodeInto(t, protoType, data, &PingResult{}) },
		},
		{
			name:   "rpc",
			encode: func(e ResultEncoder) ([]byte, error) { return e.EncodeRPCResult(&RPCResult{Data: Raw(`{"a":1}`)}) },
			check: func(t *testing.T, protoType Type, data []byte) {
				var got RPCResult
				decodeInto(t, protoType, data, &got)
				require.JSONEq(t, `{"a":1}`, string(got.Data))
			},
		},
	}
	forEachType(t, func(t *testing.T, protoType Type) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				data, err := tc.encode(GetResultEncoder(protoType))
				require.NoError(t, err)
				tc.check(t, protoType, data)
			})
		}
	})
}

func TestDataEncoder(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		t.Run("frame", func(t *testing.T) {
			frame := replyFrame(t, protoType, &Reply{Id: 1}, &Reply{Id: 2})
			replies := readReplies(t, newReplyDecoder(protoType, frame))
			require.Len(t, replies, 2)
			require.Equal(t, uint32(2), replies[1].Id)
		})
		t.Run("finish", func(t *testing.T) {
			encoder := GetDataEncoder(protoType)
			defer PutDataEncoder(protoType, encoder)
			require.NoError(t, encoder.Encode([]byte(`{"id":1}`)))
			require.NoError(t, encoder.Encode([]byte(`{"id":2}`)))
			// FinishNoCopy returns what Finish does, without the copy.
			frame := encoder.Finish()
			require.Equal(t, frame, encoder.FinishNoCopy())
			frame[0] ^= 0xff
			require.NotEqual(t, frame, encoder.FinishNoCopy(), "Finish must return a copy")
			// Reset starts a new frame.
			encoder.Reset()
			require.NoError(t, encoder.Encode([]byte(`{"id":3}`)))
			require.Less(t, len(encoder.FinishNoCopy()), len(frame))
		})
	})
	encoder := NewJSONDataEncoder()
	require.NoError(t, encoder.Encode([]byte(`{"id":1}`)))
	require.NoError(t, encoder.Encode([]byte(`{"id":2}`)))
	require.Equal(t, "{\"id\":1}\n{\"id\":2}", string(encoder.Finish()))
}

// payloadPlaces are messages with a payload in each place a reply or a push
// may have one. The JSON encoders must look at every one of them.
var payloadPlaces = []struct {
	name  string
	build func(raw Raw) (*Reply, *Push)
}{
	{"publication data", func(raw Raw) (*Reply, *Push) {
		pub := &Publication{Data: raw, Offset: 5, Info: &ClientInfo{User: "u", Client: "c"}}
		return &Reply{Push: &Push{Channel: "c", Pub: pub}}, &Push{Channel: "c", Pub: pub}
	}},
	{"publication conn_info", func(raw Raw) (*Reply, *Push) {
		pub := &Publication{Data: Raw(`{}`), Info: &ClientInfo{User: "u", Client: "c", ConnInfo: raw}}
		return &Reply{Push: &Push{Channel: "c", Pub: pub}}, &Push{Channel: "c", Pub: pub}
	}},
	{"join chan_info", func(raw Raw) (*Reply, *Push) {
		join := &Join{Info: &ClientInfo{User: "u", Client: "c", ChanInfo: raw}}
		return &Reply{Push: &Push{Channel: "c", Join: join}}, &Push{Channel: "c", Join: join}
	}},
	{"history", func(raw Raw) (*Reply, *Push) {
		return &Reply{Id: 1, History: &HistoryResult{Publications: []*Publication{{Data: Raw(`1`)}, {Data: raw, Tags: map[string]string{"k": "v"}}}}}, nil
	}},
	{"presence", func(raw Raw) (*Reply, *Push) {
		return &Reply{Id: 1, Presence: &PresenceResult{Presence: map[string]*ClientInfo{"b": {User: "v", ConnInfo: raw}}}}, nil
	}},
	{"connect data", func(raw Raw) (*Reply, *Push) {
		return &Reply{Id: 1, Connect: &ConnectResult{Client: "c", Data: raw}}, &Push{Connect: &Connect{Client: "c", Data: raw}}
	}},
	{"recovered publications", func(raw Raw) (*Reply, *Push) {
		res := &SubscribeResult{Publications: []*Publication{{Data: raw}}}
		return &Reply{Id: 1, Connect: &ConnectResult{Subs: map[string]*SubscribeResult{"ch": res}}}, nil
	}},
	{"rpc result", func(raw Raw) (*Reply, *Push) {
		return &Reply{Id: 1, Rpc: &RPCResult{Data: raw}}, &Push{Message: &Message{Data: raw}}
	}},
}

// A payload is bytes of an application put into a message as they are. The
// JSON encoders of replies and pushes refuse one which is not a JSON value,
// and what counts is the payload itself: bytes which leave the message valid
// JSON while adding to it, or taking fields away, are refused too. See also
// FuzzJSONPayloads.
func TestJSONEncoder_Payloads(t *testing.T) {
	good := []string{`{}`, `{"a":[1,2,{"b":null}]}`, `"text"`, `1.5e3`, `true`, `null`, ` {"a":1} `, "{\n\"a\": 1\n}", "", "\n"}
	bad := []string{
		`nope`, `{`, `{"a":1`, `{"a":1}}`, `1 2`, `{"a":"\x01"}`, "\xff", " ", "\r\n", "\"a\nb\"",
		// Valid as a part of a message, not as a value.
		`1,"offset":999`,
		`{}},"x":{"a":1`,
		`{},"info":{"user":"admin"}},"x":{"a":1`,
		`1}},"id":7,"push":{"pub":{"data":2`,
	}
	encode := func(t *testing.T, build func(Raw) (*Reply, *Push), raw string) bool {
		reply, push := build(Raw(raw))
		data, err := NewJSONReplyEncoder().Encode(reply)
		ok := err == nil
		if ok {
			require.True(t, cfjson.Valid(data), "%q: %s", raw, data)
			require.NotContains(t, string(data), "\n")
		} else {
			require.ErrorIs(t, err, errInvalidJSON)
		}
		if push != nil {
			data, err = NewJSONPushEncoder().Encode(push)
			require.Equal(t, ok, err == nil, "%q: the encoders disagree", raw)
			if ok {
				require.True(t, cfjson.Valid(data), "%q: %s", raw, data)
			}
		}
		return ok
	}
	for _, place := range payloadPlaces {
		t.Run(place.name, func(t *testing.T) {
			for _, raw := range good {
				require.True(t, encode(t, place.build, raw), "%q refused", raw)
			}
			for _, raw := range bad {
				require.False(t, encode(t, place.build, raw), "%q accepted", raw)
			}
		})
	}

	// A payload of nothing but newlines is null, not a hole in the message.
	data, err := NewJSONReplyEncoder().Encode(&Reply{Push: &Push{Channel: "c", Pub: &Publication{Data: Raw("\n\n")}}})
	require.NoError(t, err)
	require.Equal(t, `{"push":{"channel":"c","pub":{"data":null}}}`, string(data))

	// The encoders of parts are for payloads checked already: they write
	// them as they are, newlines dropped.
	data, err = NewJSONPushEncoder().EncodePublication(&Publication{Data: Raw("{\n  \"num\": \"1\\n\"\n\n}\n")})
	require.NoError(t, err)
	require.Equal(t, `{"data":{  "num": "1\n"}}`, string(data))
}

// encodeJSON writes into the buffer given for reuse when it is large enough:
// the reuse argument of the push encoders relies on it to save an allocation
// per subscriber.
func TestEncodeJSON(t *testing.T) {
	t.Run("reuse", func(t *testing.T) {
		reuse := make([]byte, 0, 64)
		out := encodeJSON(&Command{Id: 1}, reuse)
		require.Equal(t, `{"id":1}`, string(out))
		require.Same(t, &reuse[:1][0], &out[0])

		small := make([]byte, 0, 2)
		out = encodeJSON(&Command{Id: 1}, small)
		require.Equal(t, `{"id":1}`, string(out))
		require.Equal(t, 2, cap(small))
	})
	t.Run("result is not shared", func(t *testing.T) {
		// The result must not point into the pooled scratch buffer, which the
		// next message is encoded into.
		first := encodeJSON(&Command{Id: 1})
		second := encodeJSON(&Command{Id: 2})
		require.Equal(t, `{"id":1}`, string(first))
		require.Equal(t, `{"id":2}`, string(second))
		require.Equal(t, len(first), cap(first))
	})
	t.Run("larger than the pool keeps", func(t *testing.T) {
		channel := strings.Repeat("c", maxBufferLength+1)
		require.Equal(t, `{"channel":"`+channel+`"}`, string(encodeJSON(&Push{Channel: channel})))
	})
}

// payloadPath is how a payload is reached from a message: field indexes,
// with -1 for an element of a slice or a value of a map.
type payloadPath []int

// payloadPaths returns the paths to every payload in a value of type t,
// through pointers, slices and maps. A type is not entered again while it is
// being walked, so recursive types end.
func payloadPaths(t reflect.Type, walking map[reflect.Type]bool) []payloadPath {
	if t == reflect.TypeFor[Raw]() {
		return []payloadPath{nil}
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map:
		var paths []payloadPath
		for _, p := range payloadPaths(t.Elem(), walking) {
			if t.Kind() == reflect.Pointer {
				paths = append(paths, p)
			} else {
				paths = append(paths, append(payloadPath{-1}, p...))
			}
		}
		return paths
	case reflect.Struct:
		if walking[t] {
			return nil
		}
		walking[t] = true
		defer delete(walking, t)
		var paths []payloadPath
		for i := 0; i < t.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			for _, p := range payloadPaths(t.Field(i).Type, walking) {
				paths = append(paths, append(payloadPath{i}, p...))
			}
		}
		return paths
	}
	return nil
}

// setPayload sets the payload at the end of path in v, making what is on the
// way there, and returns how the payload is reached, for messages.
func setPayload(v reflect.Value, path payloadPath, raw Raw) string {
	if v.Type() == reflect.TypeFor[Raw]() {
		v.SetBytes(raw)
		return ""
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setPayload(v.Elem(), path, raw)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		return "[0]" + setPayload(v.Index(0), path[1:], raw)
	case reflect.Map:
		// A value of a map cannot be set in place: it is filled first.
		elem := reflect.New(v.Type().Elem()).Elem()
		where := setPayload(elem, path[1:], raw)
		v.Set(reflect.MakeMap(v.Type()))
		v.SetMapIndex(reflect.ValueOf("k").Convert(v.Type().Key()), elem)
		return "[k]" + where
	case reflect.Struct:
		return "." + v.Type().Field(path[0]).Name + setPayload(v.Field(path[0]), path[1:], raw)
	}
	panic("no payload at the end of the path in " + v.Type().String())
}

// Every payload a reply or a push can have is checked by the JSON encoders,
// wherever it is. The places are found by reflection, so a payload field
// added to the schema is covered without a change here. See also
// TestJSONEncoder_Payloads for which payloads are refused.
func TestJSONEncoder_EveryPayloadIsChecked(t *testing.T) {
	encode := map[reflect.Type]func(v any) error{
		reflect.TypeFor[*Reply](): func(v any) error { _, err := NewJSONReplyEncoder().Encode(v.(*Reply)); return err },
		reflect.TypeFor[*Push]():  func(v any) error { _, err := NewJSONPushEncoder().Encode(v.(*Push)); return err },
	}
	total := 0
	for typ, enc := range encode {
		paths := payloadPaths(typ, map[reflect.Type]bool{})
		require.NotEmpty(t, paths, typ.String())
		for _, path := range paths {
			total++
			bad := reflect.New(typ.Elem())
			where := setPayload(bad, path, Raw(`1,"x":2`))
			require.ErrorIs(t, enc(bad.Interface()), errInvalidJSON, "%s%s", typ.Elem().Name(), where)

			good := reflect.New(typ.Elem())
			setPayload(good, path, Raw(`{"a":1}`))
			require.NoError(t, enc(good.Interface()), "%s%s", typ.Elem().Name(), where)
		}
	}
	// 51 places in a Reply and 20 in a Push today. Fewer would mean the walk
	// above lost some.
	require.GreaterOrEqual(t, total, 71)
}

// The encoders of replies and pushes allocate the result and nothing else:
// that is what keeps a broadcast to many subscribers cheap. Checked so that a
// change which adds an allocation shows up here, not in production.
func TestEncoder_Allocations(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector allocates")
	}
	reply := &Reply{Id: 1, Connect: &ConnectResult{Client: "client", Version: "6.0.0", Ping: 25, Pong: true, Data: Raw(`{"a":1}`)}}
	push := &Push{Channel: "chat", Pub: &Publication{Data: Raw(`{"text":"hello"}`), Offset: 5,
		Info: &ClientInfo{User: "u", Client: "c", ConnInfo: Raw(`{"name":"n"}`)}, Tags: map[string]string{"k": "v"}}}
	forEachType(t, func(t *testing.T, protoType Type) {
		replyEncoder, pushEncoder := GetReplyEncoder(protoType), GetPushEncoder(protoType)
		allocs := testing.AllocsPerRun(100, func() {
			if _, err := replyEncoder.Encode(reply); err != nil {
				t.Fatal(err)
			}
		})
		require.Equal(t, 1.0, allocs, "reply")
		allocs = testing.AllocsPerRun(100, func() {
			if _, err := pushEncoder.Encode(push); err != nil {
				t.Fatal(err)
			}
		})
		require.Equal(t, 1.0, allocs, "push")
	})
}

// The encoders share pooled buffers. Used from many goroutines at once, each
// must still get its own result, never one written over by another.
func TestEncoder_Concurrent(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 500; i++ {
					id := uint32(g*1000 + i)
					reply := &Reply{Id: id, Push: &Push{Channel: "c", Pub: &Publication{Data: textPayload(10+i%300, int64(id)), Offset: uint64(id)}}}
					data, err := GetReplyEncoder(protoType).Encode(reply)
					if err != nil {
						t.Error(err)
						return
					}
					encoder := GetDataEncoder(protoType)
					if err := encoder.Encode(data); err != nil {
						t.Error(err)
						return
					}
					frame := encoder.Finish()
					PutDataEncoder(protoType, encoder)
					replies := readReplies(t, newReplyDecoder(protoType, frame))
					if len(replies) != 1 || replies[0].Id != id || replies[0].Push.Pub.Offset != uint64(id) ||
						string(replies[0].Push.Pub.Data) != string(reply.Push.Pub.Data) {
						t.Errorf("reply %d came back as %v", id, replies)
						return
					}
				}
			}()
		}
		wg.Wait()
	})
}
