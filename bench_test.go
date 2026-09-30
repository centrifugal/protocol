package protocol_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/centrifugal/protocol"
)

// Benchmarks of the package on what a server and a client do with it, on
// messages shaped like real ones. Names have the form
// Benchmark<Operation>/type=<json|protobuf>/msg=<message>, for benchstat:
//
//	benchstat -col /type -row /msg bench.txt
//
// The file uses nothing but the exported API of the package, and none of the
// methods of messages, so it runs unchanged against earlier versions: see
// `make bench-compare`, which compares the working tree with any git ref.

var protocolTypes = []protocol.Type{protocol.TypeJSON, protocol.TypeProtobuf}

const (
	jwt    = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiI0MiIsImV4cCI6MTc5MDAwMDAwMCwiaWF0IjoxNzgwMDAwMDAwLCJpbmZvIjp7Im5hbWUiOiJBbGV4In19.q3Yk0n8Zr3bTtB7m5oYV8m2JX0e1VgG5q0m8y9Qw3Hc"
	client = "c8d9a0f2-0d5f-4d0b-9d5e-6d2f6b8f2a11"
)

// payload is a JSON object of about size bytes, like an application
// publishes.
func payload(size int) protocol.Raw {
	text := strings.Repeat("hello world ", max(size-60, 0)/12+1)[:max(size-60, 0)]
	return protocol.Raw(`{"id":184467,"user":"user-42","text":"` + text + `","ts":1790000000123}`)
}

func clientInfo(n int) *protocol.ClientInfo {
	return &protocol.ClientInfo{
		User: fmt.Sprintf("user-%d", n), Client: fmt.Sprintf("%s-%d", client, n),
		ConnInfo: protocol.Raw(`{"name":"Alex","avatar":"https://example.com/a.png"}`),
	}
}

func publication(size int, info bool) *protocol.Publication {
	pub := &protocol.Publication{Data: payload(size), Offset: 1043}
	if info {
		pub.Info = clientInfo(0)
		pub.Tags = map[string]string{"kind": "chat"}
	}
	return pub
}

// Commands, as a client sends them.
var commands = []struct {
	name string
	cmd  *protocol.Command
}{
	{"connect", &protocol.Command{Id: 1, Connect: &protocol.ConnectRequest{Token: jwt, Name: "js", Version: "5.3.4"}}},
	{"subscribe", &protocol.Command{Id: 2, Subscribe: &protocol.SubscribeRequest{Channel: "chat:index", Recover: true, Epoch: "xKdP", Offset: 1042}}},
	{"publish_256B", &protocol.Command{Id: 3, Publish: &protocol.PublishRequest{Channel: "chat:index", Data: payload(256)}}},
	{"rpc", &protocol.Command{Id: 4, Rpc: &protocol.RPCRequest{Method: "getMessages", Data: protocol.Raw(`{"room":"index","limit":20}`)}}},
}

// Frames of commands, as a server reads them.
var commandFrames = []struct {
	name string
	cmds []*protocol.Command
}{
	{"connect", []*protocol.Command{commands[0].cmd}},
	{"connect+3_subscribes", []*protocol.Command{
		commands[0].cmd,
		{Id: 2, Subscribe: &protocol.SubscribeRequest{Channel: "chat:index", Recover: true, Epoch: "xKdP", Offset: 1042}},
		{Id: 3, Subscribe: &protocol.SubscribeRequest{Channel: "notifications", Recover: true, Epoch: "pQ7z", Offset: 17}},
		{Id: 4, Subscribe: &protocol.SubscribeRequest{Channel: "#42"}},
	}},
	{"64_publishes_256B", func() []*protocol.Command {
		cmds := make([]*protocol.Command, 64)
		for i := range cmds {
			cmds[i] = &protocol.Command{Id: uint32(i + 1), Publish: &protocol.PublishRequest{Channel: "chat:index", Data: payload(256)}}
		}
		return cmds
	}()},
}

// Replies, as a server sends them in response to commands.
var replies = []struct {
	name  string
	reply *protocol.Reply
}{
	{"connect", &protocol.Reply{Id: 1, Connect: &protocol.ConnectResult{
		Client: client, Version: "6.5.2", Expires: true, Ttl: 3600, Ping: 25, Pong: true,
	}}},
	{"connect_data+2_subs", &protocol.Reply{Id: 1, Connect: &protocol.ConnectResult{
		Client: client, Version: "6.5.2", Expires: true, Ttl: 3600, Ping: 25, Pong: true,
		Data: protocol.Raw(`{"settings":{"theme":"dark","lang":"en"},"features":["a","b","c"]}`),
		Subs: map[string]*protocol.SubscribeResult{
			"#42":           {Recoverable: true, Epoch: "xKdP", Offset: 1042, Positioned: true},
			"notifications": {Recoverable: true, Epoch: "pQ7z", Offset: 17, Positioned: true},
		},
	}}},
	{"subscribe_recovered_20", &protocol.Reply{Id: 2, Subscribe: &protocol.SubscribeResult{
		Recoverable: true, Epoch: "xKdP", Offset: 1062, Recovered: true, Positioned: true,
		Publications: func() []*protocol.Publication {
			pubs := make([]*protocol.Publication, 20)
			for i := range pubs {
				pubs[i] = publication(256, i%2 == 0)
			}
			return pubs
		}(),
	}}},
	{"history_100", &protocol.Reply{Id: 3, History: &protocol.HistoryResult{
		Epoch: "xKdP", Offset: 1142,
		Publications: func() []*protocol.Publication {
			pubs := make([]*protocol.Publication, 100)
			for i := range pubs {
				pubs[i] = publication(128, false)
			}
			return pubs
		}(),
	}}},
	{"presence_100", &protocol.Reply{Id: 4, Presence: &protocol.PresenceResult{
		Presence: func() map[string]*protocol.ClientInfo {
			presence := make(map[string]*protocol.ClientInfo, 100)
			for i := 0; i < 100; i++ {
				info := clientInfo(i)
				presence[info.Client] = info
			}
			return presence
		}(),
	}}},
	{"error", &protocol.Reply{Id: 5, Error: &protocol.Error{Code: 103, Message: "permission denied"}}},
}

// Pushes of publications, as a server broadcasts them.
var pushes = []struct {
	name string
	push *protocol.Push
}{
	{"publication_64B", &protocol.Push{Channel: "chat:index", Pub: publication(64, false)}},
	{"publication_256B", &protocol.Push{Channel: "chat:index", Pub: publication(256, false)}},
	{"publication_256B+info", &protocol.Push{Channel: "chat:index", Pub: publication(256, true)}},
	{"publication_4KB", &protocol.Push{Channel: "chat:index", Pub: publication(4096, false)}},
	{"join", &protocol.Push{Channel: "chat:index", Join: &protocol.Join{Info: clientInfo(1)}}},
}

func encodeCommand(b *testing.B, protoType protocol.Type, cmd *protocol.Command) []byte {
	var data []byte
	var err error
	if protoType == protocol.TypeJSON {
		data, err = protocol.NewJSONCommandEncoder().Encode(cmd)
	} else {
		data, err = protocol.NewProtobufCommandEncoder().Encode(cmd)
	}
	if err != nil {
		b.Fatal(err)
	}
	return data
}

// commandFrame puts commands into a frame: JSON ones separated by newlines,
// Protobuf ones, which come with their length, one after another.
func commandFrame(b *testing.B, protoType protocol.Type, cmds []*protocol.Command) []byte {
	var frame []byte
	for i, cmd := range cmds {
		if protoType == protocol.TypeJSON && i > 0 {
			frame = append(frame, '\n')
		}
		frame = append(frame, encodeCommand(b, protoType, cmd)...)
	}
	return frame
}

func replyFrame(b *testing.B, protoType protocol.Type, reply *protocol.Reply, n int) []byte {
	data, err := protocol.GetReplyEncoder(protoType).Encode(reply)
	if err != nil {
		b.Fatal(err)
	}
	encoder := protocol.GetDataEncoder(protoType)
	defer protocol.PutDataEncoder(protoType, encoder)
	for i := 0; i < n; i++ {
		if err := encoder.Encode(data); err != nil {
			b.Fatal(err)
		}
	}
	return encoder.Finish()
}

var (
	sinkBytes   []byte
	sinkCommand *protocol.Command
	sinkReply   *protocol.Reply
)

func BenchmarkEncodeCommand(b *testing.B) {
	for _, protoType := range protocolTypes {
		for _, c := range commands {
			b.Run("type="+string(protoType)+"/msg="+c.name, func(b *testing.B) {
				b.ReportAllocs()
				var encoder protocol.CommandEncoder = protocol.NewJSONCommandEncoder()
				if protoType == protocol.TypeProtobuf {
					encoder = protocol.NewProtobufCommandEncoder()
				}
				for b.Loop() {
					data, err := encoder.Encode(c.cmd)
					if err != nil {
						b.Fatal(err)
					}
					sinkBytes = data
				}
			})
		}
	}
}

// decodeFrame is what a server does with a frame a client sent.
func decodeFrame(b *testing.B, protoType protocol.Type, frame []byte, want int) {
	decoder := protocol.GetCommandDecoder(protoType, frame)
	n := 0
	for {
		cmd, err := decoder.Decode()
		if cmd != nil {
			sinkCommand = cmd
			n++
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				b.Fatal(err)
			}
			break
		}
	}
	protocol.PutCommandDecoder(protoType, decoder)
	if n != want {
		b.Fatalf("%d commands decoded, want %d", n, want)
	}
}

func BenchmarkDecodeFrame(b *testing.B) {
	for _, protoType := range protocolTypes {
		for _, f := range commandFrames {
			frame := commandFrame(b, protoType, f.cmds)
			b.Run("type="+string(protoType)+"/msg="+f.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(frame)))
				for b.Loop() {
					decodeFrame(b, protoType, frame, len(f.cmds))
				}
			})
		}
	}
}

func BenchmarkDecodeFrameParallel(b *testing.B) {
	for _, protoType := range protocolTypes {
		f := commandFrames[1]
		frame := commandFrame(b, protoType, f.cmds)
		b.Run("type="+string(protoType)+"/msg="+f.name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					decodeFrame(b, protoType, frame, len(f.cmds))
				}
			})
		})
	}
}

// BenchmarkDecodeStream is the same with a decoder which reads from a stream,
// as the transports which have no frames do.
func BenchmarkDecodeStream(b *testing.B) {
	for _, protoType := range protocolTypes {
		for _, f := range commandFrames {
			frame := commandFrame(b, protoType, f.cmds)
			b.Run("type="+string(protoType)+"/msg="+f.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(frame)))
				reader := bytes.NewReader(frame)
				for b.Loop() {
					reader.Reset(frame)
					decoder := protocol.GetStreamCommandDecoderLimited(protoType, reader, 1<<20)
					n := 0
					for {
						cmd, _, err := decoder.Decode()
						if cmd != nil {
							sinkCommand = cmd
							n++
						}
						if err != nil {
							if !errors.Is(err, io.EOF) {
								b.Fatal(err)
							}
							break
						}
					}
					protocol.PutStreamCommandDecoder(protoType, decoder)
					if n != len(f.cmds) {
						b.Fatalf("%d commands decoded, want %d", n, len(f.cmds))
					}
				}
			})
		}
	}
}

func BenchmarkEncodeReply(b *testing.B) {
	for _, protoType := range protocolTypes {
		for _, r := range replies {
			b.Run("type="+string(protoType)+"/msg="+r.name, func(b *testing.B) {
				b.ReportAllocs()
				encoder := protocol.GetReplyEncoder(protoType)
				for b.Loop() {
					data, err := encoder.Encode(r.reply)
					if err != nil {
						b.Fatal(err)
					}
					sinkBytes = data
				}
			})
		}
	}
}

// BenchmarkEncodePush encodes a push the way a server does once per
// broadcast, for all subscribers of a channel.
func BenchmarkEncodePush(b *testing.B) {
	for _, protoType := range protocolTypes {
		for _, p := range pushes {
			b.Run("type="+string(protoType)+"/msg="+p.name, func(b *testing.B) {
				b.ReportAllocs()
				encoder := protocol.GetPushEncoder(protoType)
				for b.Loop() {
					data, err := encoder.Encode(p.push)
					if err != nil {
						b.Fatal(err)
					}
					sinkBytes = data
				}
			})
		}
	}
}

func BenchmarkEncodePushParallel(b *testing.B) {
	for _, protoType := range protocolTypes {
		p := pushes[2]
		b.Run("type="+string(protoType)+"/msg="+p.name, func(b *testing.B) {
			b.ReportAllocs()
			encoder := protocol.GetPushEncoder(protoType)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					data, err := encoder.Encode(p.push)
					if err != nil {
						b.Error(err)
						return
					}
					sinkBytes = data
				}
			})
		})
	}
}

// BenchmarkDecodeReplies is what a client does with a frame of a server.
func BenchmarkDecodeReplies(b *testing.B) {
	for _, protoType := range protocolTypes {
		for _, f := range []struct {
			name  string
			reply *protocol.Reply
			n     int
		}{
			{"connect", replies[0].reply, 1},
			{"8_publications_256B+info", &protocol.Reply{Push: pushes[2].push}, 8},
			{"history_100", replies[3].reply, 1},
		} {
			frame := replyFrame(b, protoType, f.reply, f.n)
			b.Run("type="+string(protoType)+"/msg="+f.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(frame)))
				for b.Loop() {
					var decoder protocol.ReplyDecoder
					if protoType == protocol.TypeJSON {
						decoder = protocol.NewJSONReplyDecoder(frame)
					} else {
						decoder = protocol.NewProtobufReplyDecoder(frame)
					}
					n := 0
					for {
						reply, err := decoder.Decode()
						if err != nil {
							break
						}
						sinkReply = reply
						n++
					}
					if n != f.n {
						b.Fatalf("%d replies decoded, want %d", n, f.n)
					}
				}
			})
		}
	}
}

// BenchmarkDataEncoder puts encoded messages into a frame, as a server does
// for what it writes to a connection at once.
func BenchmarkDataEncoder(b *testing.B) {
	for _, protoType := range protocolTypes {
		message, err := protocol.GetPushEncoder(protoType).Encode(pushes[1].push)
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range []int{8, 64} {
			b.Run(fmt.Sprintf("type=%s/msg=%d_publications_256B", protoType, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					encoder := protocol.GetDataEncoder(protoType)
					for i := 0; i < n; i++ {
						if err := encoder.Encode(message); err != nil {
							b.Fatal(err)
						}
					}
					sinkBytes = encoder.Finish()
					protocol.PutDataEncoder(protoType, encoder)
				}
			})
		}
	}
}
