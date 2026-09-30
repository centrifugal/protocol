package cfprotobufcmp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func payload(size int) Raw {
	return Raw(`{"input":"` + strings.Repeat("i", size) + `"}`)
}

const benchToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiI0MiIsImV4cCI6MTc2MDAwMDAwMCwiaW5mbyI6eyJuYW1lIjoiQWxleCJ9fQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

func benchInfo(n int) *ClientInfo {
	return &ClientInfo{
		User:     fmt.Sprintf("user-%d", n),
		Client:   fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", n),
		ConnInfo: Raw(`{"name":"Alex","avatar":"https://example.com/a.png"}`),
	}
}

func benchPublications(n, size int) []*Publication {
	pubs := make([]*Publication, n)
	for i := range pubs {
		pubs[i] = &Publication{Data: payload(size), Offset: uint64(1000 + i), Info: benchInfo(i)}
	}
	return pubs
}

func benchPresence(n int) map[string]*ClientInfo {
	m := make(map[string]*ClientInfo, n)
	for i := 0; i < n; i++ {
		info := benchInfo(i)
		m[info.Client] = info
	}
	return m
}

// benchMessages are the messages benchmarks run on, the same ones the JSON
// benchmarks in ../cfjsoncmp use: the commands a server decodes and the
// replies and pushes it encodes, from the smallest to large ones.
var benchMessages = []struct {
	name string
	new  func() message
	msg  message
}{
	{"cmd_ping", func() message { return new(Command) }, &Command{}},
	{"cmd_connect", func() message { return new(Command) }, &Command{Id: 1, Connect: &ConnectRequest{
		Token: benchToken, Name: "js", Version: "5.4.0",
		Subs: map[string]*SubscribeRequest{"news": {Recover: true, Epoch: "xyzw", Offset: 42}},
	}}},
	{"cmd_subscribe", func() message { return new(Command) }, &Command{Id: 2, Subscribe: &SubscribeRequest{
		Channel: "chat:room-1234", Token: benchToken, Recover: true, Epoch: "xyzw", Offset: 123456, Positioned: true,
	}}},
	{"cmd_publish_256", func() message { return new(Command) }, &Command{Id: 3, Publish: &PublishRequest{
		Channel: "chat:room-1234", Data: payload(256),
	}}},
	{"cmd_publish_4k", func() message { return new(Command) }, &Command{Id: 3, Publish: &PublishRequest{
		Channel: "chat:room-1234", Data: payload(4096),
	}}},
	{"cmd_rpc", func() message { return new(Command) }, &Command{Id: 4, Rpc: &RPCRequest{
		Method: "getCurrentPrice", Data: Raw(`{"symbol":"BTCUSD","depth":10}`),
	}}},
	{"reply_pub_256", func() message { return new(Reply) }, &Reply{Push: &Push{
		Channel: "chat:room-1234", Pub: &Publication{Data: payload(256), Offset: 123456},
	}}},
	{"reply_pub_info_256", func() message { return new(Reply) }, &Reply{Push: &Push{
		Channel: "chat:room-1234", Pub: &Publication{
			Data: payload(256), Offset: 123456, Info: benchInfo(1), Tags: map[string]string{"lang": "en"},
		},
	}}},
	{"reply_pub_4k", func() message { return new(Reply) }, &Reply{Push: &Push{
		Channel: "chat:room-1234", Pub: &Publication{Data: payload(4096), Offset: 123456},
	}}},
	{"reply_join", func() message { return new(Reply) }, &Reply{Push: &Push{
		Channel: "chat:room-1234", Join: &Join{Info: benchInfo(1)},
	}}},
	{"reply_connect", func() message { return new(Reply) }, &Reply{Id: 1, Connect: &ConnectResult{
		Client: "550e8400-e29b-41d4-a716-446655440000", Version: "0.0.0 OSS", Expires: true, Ttl: 3600, Ping: 25, Pong: true,
		Session: "b1946ac9-2492-4d4b-a8a7-0e1f5a3c8c2d", Node: "8d8f2c1e-6b7a-4f0e-9d3c-5a4b3c2d1e0f",
		Subs: map[string]*SubscribeResult{"news": {Recoverable: true, Epoch: "xyzw", Offset: 42, Positioned: true}},
	}}},
	{"reply_subscribe_recovered_20", func() message { return new(Reply) }, &Reply{Id: 2, Subscribe: &SubscribeResult{
		Recoverable: true, Epoch: "xyzw", Offset: 1020, Positioned: true, Recovered: true, Publications: benchPublications(20, 128),
	}}},
	{"reply_history_100", func() message { return new(Reply) }, &Reply{Id: 5, History: &HistoryResult{
		Epoch: "xyzw", Offset: 1100, Publications: benchPublications(100, 128),
	}}},
	{"reply_presence_100", func() message { return new(Reply) }, &Reply{Id: 6, Presence: &PresenceResult{
		Presence: benchPresence(100),
	}}},
	{"reply_error", func() message { return new(Reply) }, &Reply{Id: 7, Error: &Error{Code: 103, Message: "permission denied"}}},
}

var sink []byte

// BenchmarkMarshal measures encoding into a new slice, which is what the
// Protobuf encoders of protocol do.
func BenchmarkMarshal(b *testing.B) {
	for _, bm := range benchMessages {
		size := int64(bm.msg.SizeVT())
		b.Run("msg="+bm.name+"/impl=vtprotobuf", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(size)
			for i := 0; i < b.N; i++ {
				sink, _ = bm.msg.MarshalVT()
			}
		})
		b.Run("msg="+bm.name+"/impl=cfprotobuf", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(size)
			for i := 0; i < b.N; i++ {
				sink, _ = bm.msg.MarshalCF()
			}
		})
	}
}

// BenchmarkMarshalTo measures the generated code alone: sizing a message and
// writing it into a buffer which is reused.
func BenchmarkMarshalTo(b *testing.B) {
	for _, bm := range benchMessages {
		size := bm.msg.SizeVT()
		buf := make([]byte, size)
		b.Run("msg="+bm.name+"/impl=vtprotobuf", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				_, _ = bm.msg.MarshalToSizedBufferVT(buf[:bm.msg.SizeVT()])
			}
		})
		b.Run("msg="+bm.name+"/impl=cfprotobuf", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				_, _ = bm.msg.MarshalToSizedBufferCF(buf[:bm.msg.SizeCF()])
			}
		})
	}
}

var msgSink message

// BenchmarkUnmarshal measures decoding into a new message, the way protocol
// does it.
func BenchmarkUnmarshal(b *testing.B) {
	for _, bm := range benchMessages {
		data, _ := bm.msg.MarshalVT()
		b.Run("msg="+bm.name+"/impl=vtprotobuf", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				m := bm.new()
				if err := m.UnmarshalVT(data); err != nil {
					b.Fatal(err)
				}
				msgSink = m
			}
		})
		b.Run("msg="+bm.name+"/impl=cfprotobuf", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				m := bm.new()
				if err := m.UnmarshalCF(data); err != nil {
					b.Fatal(err)
				}
				msgSink = m
			}
		})
	}
}

// The benchmarks are only worth something if both implementations do the same
// work.
func TestBenchMessages(t *testing.T) {
	for _, bm := range benchMessages {
		want, _ := bm.msg.MarshalVT()
		got, _ := bm.msg.MarshalCF()
		if reply, ok := bm.msg.(*Reply); !ok || reply.Presence == nil {
			if !bytes.Equal(got, want) {
				t.Fatalf("%s: encodings differ", bm.name)
			}
		}
		compareUnmarshal(t, want, bm.new())
	}
}
