package cfjsoncmp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/centrifugal/protocol/cfjson"
	"github.com/mailru/easyjson/jlexer"
	segmentio "github.com/segmentio/encoding/json"
)

func payload(size int) Raw {
	return Raw(`{"input":"` + strings.Repeat("i", size) + `"}`)
}

// benchDocument is a payload with structure, as opposed to the single long
// string payload() returns: about 1 KiB of nested objects, arrays, numbers
// and short strings.
var benchDocument = Raw(`{"type":"order_update","order":{"id":"ord_8f3a2b1c","status":"filled","symbol":"BTC-USD","side":"buy",` +
	`"price":64250.5,"quantity":0.125,"filled":0.125,"fee":{"amount":1.0039,"currency":"USD"},"created_at":1760000000123,` +
	`"updated_at":1760000004567,"tags":["margin","api","ioc"],"client":{"id":"c_42","name":"Alex","verified":true,"tier":null}},` +
	`"book":{"bids":[[64250.0,1.2],[64249.5,0.8],[64249.0,2.5],[64248.5,0.3],[64248.0,4.1]],` +
	`"asks":[[64251.0,0.9],[64251.5,1.7],[64252.0,0.4],[64252.5,3.2],[64253.0,1.1]],"sequence":99887766},` +
	`"trades":[{"id":1,"p":64250.5,"q":0.05,"t":1760000004001,"m":false},{"id":2,"p":64250.5,"q":0.075,"t":1760000004333,"m":true}],` +
	`"message":"Your order has been filled. Thank you for trading with us!","meta":{"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736",` +
	`"span_id":"00f067aa0ba902b7","region":"eu-central-1","retries":0,"flags":{"sampled":true,"debug":false}}}`)

// benchUTF8Text is a payload which is mostly text outside of ASCII: about
// 1 KiB of Cyrillic words with an emoji now and then. cfjson checks that it is
// valid UTF-8, segmentio does not.
var benchUTF8Text = Raw(`{"text":"` + strings.Repeat("\xd0\xbf\xd1\x80\xd0\xb8\xd0\xb2\xd0\xb5\xd1\x82 \xd0\xbc\xd0\xb8\xd1\x80 \xf0\x9f\x98\x80 ", 40) + `"}`)

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

// benchMessages are the messages benchmarks run on: the commands a server
// decodes and the replies and pushes it encodes, from the smallest to large
// ones.
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
	{"cmd_publish_doc_1k", func() message { return new(Command) }, &Command{Id: 3, Publish: &PublishRequest{
		Channel: "chat:room-1234", Data: benchDocument,
	}}},
	{"cmd_publish_utf8_1k", func() message { return new(Command) }, &Command{Id: 3, Publish: &PublishRequest{
		Channel: "chat:room-1234", Data: benchUTF8Text,
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
	{"reply_pub_doc_1k", func() message { return new(Reply) }, &Reply{Push: &Push{
		Channel: "chat:room-1234", Pub: &Publication{Data: benchDocument, Offset: 123456},
	}}},
	{"reply_pub_utf8_1k", func() message { return new(Reply) }, &Reply{Push: &Push{
		Channel: "chat:room-1234", Pub: &Publication{Data: benchUTF8Text, Offset: 123456},
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

// BenchmarkEncode measures encoding the way protocol does it: into a pooled
// buffer, with the result copied into a slice of its own.
func BenchmarkEncode(b *testing.B) {
	for _, bm := range benchMessages {
		b.Run("msg="+bm.name+"/impl=easyjson", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(oldEncode(bm.msg))))
			for i := 0; i < b.N; i++ {
				sink = oldEncode(bm.msg)
			}
		})
		b.Run("msg="+bm.name+"/impl=cfjson", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(newEncode(bm.msg))))
			for i := 0; i < b.N; i++ {
				sink = newEncode(bm.msg)
			}
		})
	}
}

// BenchmarkAppend measures the generated encoders alone, writing into a
// buffer which is reused.
func BenchmarkAppend(b *testing.B) {
	for _, bm := range benchMessages {
		b.Run("msg="+bm.name+"/impl=easyjson", func(b *testing.B) {
			w := newWriter()
			b.ReportAllocs()
			b.SetBytes(int64(len(oldEncode(bm.msg))))
			for i := 0; i < b.N; i++ {
				w.Buffer.Reset()
				bm.msg.MarshalEasyJSON(w)
			}
			sink = w.Buffer.Bytes()
		})
		b.Run("msg="+bm.name+"/impl=cfjson", func(b *testing.B) {
			var buf []byte
			b.ReportAllocs()
			b.SetBytes(int64(len(oldEncode(bm.msg))))
			for i := 0; i < b.N; i++ {
				buf = bm.msg.AppendJSON(buf[:0])
			}
			sink = buf
		})
	}
}

var msgSink message

// BenchmarkDecode measures decoding into a new message, the way protocol does
// it: segmentio is what it used before, easyjson is the decoder easyjson
// generates (never used by protocol, here for reference).
func BenchmarkDecode(b *testing.B) {
	for _, bm := range benchMessages {
		data := oldEncode(bm.msg)
		// mode=copy keeps the names of results recorded when there was a
		// zero-copy mode as well.
		prefix := "msg=" + bm.name + "/mode=copy"
		b.Run(prefix+"/impl=segmentio", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				m := bm.new()
				if err := oldDecode(data, m); err != nil {
					b.Fatal(err)
				}
				msgSink = m
			}
		})
		b.Run(prefix+"/impl=easyjson", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				m := bm.new()
				l := jlexer.Lexer{Data: data}
				m.(interface{ UnmarshalEasyJSON(*jlexer.Lexer) }).UnmarshalEasyJSON(&l)
				if err := l.Error(); err != nil {
					b.Fatal(err)
				}
				msgSink = m
			}
		})
		b.Run(prefix+"/impl=cfjson", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				m := bm.new()
				if err := newDecode(data, m); err != nil {
					b.Fatal(err)
				}
				msgSink = m
			}
		})
	}
}

// BenchmarkDecodeReuse measures the decoders with less allocation around
// them: the message is decoded into over and over, so that after the first
// iteration only strings are allocated. protocol never decodes this way, a
// Command must be a new one every time.
func BenchmarkDecodeReuse(b *testing.B) {
	for _, bm := range benchMessages {
		data := oldEncode(bm.msg)
		b.Run("msg="+bm.name+"/impl=segmentio", func(b *testing.B) {
			m := bm.new()
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if err := oldDecode(data, m); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("msg="+bm.name+"/impl=cfjson", func(b *testing.B) {
			m := bm.new()
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if err := newDecode(data, m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// capitalizeKeys rewrites every key of an encoded message to start with a
// capital letter: {"Id":1,"Publish":{"Channel":"c"}}. Only a decoder which
// matches keys without regard to letter case makes anything of it.
func capitalizeKeys(data []byte) []byte {
	return keyPattern.ReplaceAllFunc(data, func(key []byte) []byte {
		key = bytes.Clone(key)
		if len(key) > 1 && key[1] >= 'a' && key[1] <= 'z' {
			key[1] -= 'a' - 'A'
		}
		return key
	})
}

// BenchmarkDecodeFold measures what matching keys without regard to letter
// case costs, which the -fold-keys option of the generator turns on and
// segmentio does by default.
//
// With keys=exact the input is what encoders produce: it shows that the
// option costs nothing when keys are written the way fields are named. With
// keys=capitalized every key starts with a capital letter, the worst case:
// every single key takes the fallback.
func BenchmarkDecodeFold(b *testing.B) {
	for _, bm := range benchMessages {
		exact := oldEncode(bm.msg)
		for _, keys := range []string{"exact", "capitalized"} {
			data := exact
			if keys == "capitalized" {
				data = capitalizeKeys(exact)
			}
			prefix := "msg=" + bm.name + "/keys=" + keys
			b.Run(prefix+"/impl=segmentio", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(data)))
				for i := 0; i < b.N; i++ {
					m := bm.new()
					if err := segmentioDecode(data, m); err != nil {
						b.Fatal(err)
					}
					msgSink = m
				}
			})
			if keys == "exact" {
				b.Run(prefix+"/impl=cfjson", func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(data)))
					for i := 0; i < b.N; i++ {
						m := bm.new()
						if err := newDecode(data, m); err != nil {
							b.Fatal(err)
						}
						msgSink = m
					}
				})
			}
			b.Run(prefix+"/impl=cfjson-fold", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(data)))
				for i := 0; i < b.N; i++ {
					m := bm.new()
					if err := foldDecode(data, m); err != nil {
						b.Fatal(err)
					}
					msgSink = m
				}
			})
		}
	}
}

var boolSink bool

// BenchmarkValid measures validation of an encoded message, which protocol
// does for every Reply and Push it encodes.
func BenchmarkValid(b *testing.B) {
	for _, bm := range benchMessages {
		data := oldEncode(bm.msg)
		if !segmentio.Valid(data) || !cfjson.Valid(data) {
			b.Fatalf("invalid message %s", bm.name)
		}
		b.Run("msg="+bm.name+"/impl=segmentio", func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				boolSink = segmentio.Valid(data)
			}
		})
		b.Run("msg="+bm.name+"/impl=cfjson", func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				boolSink = cfjson.Valid(data)
			}
		})
	}
}

// The benchmarks are only worth something if both implementations do the same
// work.
func TestBenchMessages(t *testing.T) {
	for _, bm := range benchMessages {
		want := oldEncode(bm.msg)
		if _, ok := bm.msg.(*Reply); !ok || bm.msg.(*Reply).Presence == nil {
			if got := newEncode(bm.msg); !bytes.Equal(got, want) {
				t.Fatalf("%s:\n got %s\nwant %s", bm.name, got, want)
			}
		}
		compareDecode(t, want, bm.new())
	}
}
