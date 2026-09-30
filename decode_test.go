package protocol

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tests of the decoders of whole frames: CommandDecoder, which a server uses
// for commands of clients, and ReplyDecoder, which a client uses for what a
// server sends. The decoders which read from a stream are in
// decode_stream_test.go.

// messageSizes are the sizes around which allocation and buffering change: the
// bufio.Reader buffer (4096), maxRetainedLineBuffer (65536), maxBufferLength,
// the ceiling of the byte pool (262144), and a few megabytes.
var messageSizes = []int{
	0, 1, 100,
	4095, 4096, 4097,
	65535, 65536, 65537,
	262143, 262144, 262145,
	1_000_000,
	5_000_000,
}

func TestCommandDecoder(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		t.Run("one command", func(t *testing.T) {
			cmds := []*Command{{Id: 1, Connect: &ConnectRequest{Token: "token", Name: "go"}}}
			got := readCommands(t, GetCommandDecoder(protoType, commandFrame(t, protoType, cmds...)))
			requireCommands(t, cmds, got)
			require.Equal(t, "token", got[0].Connect.Token)
		})
		t.Run("several commands", func(t *testing.T) {
			cmds := []*Command{publishCommand(1, "a", 10), publishCommand(2, "b", 100), {Id: 3}}
			requireCommands(t, cmds, readCommands(t, GetCommandDecoder(protoType, commandFrame(t, protoType, cmds...))))
		})
		t.Run("sizes", func(t *testing.T) {
			for _, size := range messageSizes {
				cmds := []*Command{publishCommand(42, "chan", size)}
				requireCommands(t, cmds, readCommands(t, GetCommandDecoder(protoType, commandFrame(t, protoType, cmds...))))
			}
		})
		t.Run("sizes mixed in a frame", func(t *testing.T) {
			var cmds []*Command
			for i, size := range []int{10, 5000, 70000, 300000, 1_000_000, 20, 100000} {
				cmds = append(cmds, publishCommand(uint32(i+1), "c", size))
			}
			requireCommands(t, cmds, readCommands(t, GetCommandDecoder(protoType, commandFrame(t, protoType, cmds...))))
		})
		t.Run("reset", func(t *testing.T) {
			first := []*Command{{Id: 1}}
			second := []*Command{{Id: 2}, {Id: 3}}
			decoder := GetCommandDecoder(protoType, commandFrame(t, protoType, first...))
			requireCommands(t, first, readCommands(t, decoder))
			// Reset rewinds as well as swaps the frame: the second frame must
			// not look fully consumed already.
			require.NoError(t, decoder.Reset(commandFrame(t, protoType, second...)))
			requireCommands(t, second, readCommands(t, decoder))
			require.NoError(t, decoder.Reset(commandFrame(t, protoType, first...)))
			requireCommands(t, first, readCommands(t, decoder))
		})
		t.Run("pooled", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				cmds := []*Command{{Id: uint32(i + 1)}}
				decoder := GetCommandDecoder(protoType, commandFrame(t, protoType, cmds...))
				requireCommands(t, cmds, readCommands(t, decoder))
				PutCommandDecoder(protoType, decoder)
			}
		})
	})
}

func TestJSONCommandDecoder(t *testing.T) {
	t.Run("newline after the last command", func(t *testing.T) {
		frame := []byte(`{"subscribe":{"channel":"chat:1","recover":true,"epoch":"WHBN"},"id":222}
{"subscribe":{"channel":"chat:2","recover":true,"epoch":"yenC"},"id":223}
{"subscribe":{"channel":"chat:index"},"id":224}
`)
		cmds := readCommands(t, NewJSONCommandDecoder(frame))
		require.Len(t, cmds, 3)
		require.Equal(t, "chat:1", cmds[0].Subscribe.Channel)
		require.Equal(t, "chat:2", cmds[1].Subscribe.Channel)
		require.Equal(t, "chat:index", cmds[2].Subscribe.Channel)
	})
	t.Run("empty frame", func(t *testing.T) {
		_, err := NewJSONCommandDecoder(nil).Decode()
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})
	t.Run("empty line", func(t *testing.T) {
		// Commands are one per line, so an empty line is an empty command.
		decoder := NewJSONCommandDecoder([]byte("{\"id\":1}\n\n\n{\"id\":2}\n"))
		cmd, err := decoder.Decode()
		require.NoError(t, err)
		require.Equal(t, uint32(1), cmd.Id)
		_, err = decoder.Decode()
		require.Error(t, err)
		require.NotErrorIs(t, err, io.EOF)
	})
	t.Run("zero-copy", func(t *testing.T) {
		// Strings point into the frame, payloads are copied.
		frame := []byte(`{"id":1,"publish":{"channel":"chan","data":{"a":1}}}`)
		cmd, err := NewJSONCommandDecoder(frame).Decode()
		require.ErrorIs(t, err, io.EOF)
		copy(frame, `{"id":1,"publish":{"channel":"CHAN","data":{"A":1}}}`)
		require.Equal(t, "CHAN", cmd.Publish.Channel)
		require.Equal(t, `{"a":1}`, string(cmd.Publish.Data))
	})
}

// malformedFrame is a Protobuf frame a decoder must refuse with err (with an
// error other than io.EOF if err is nil).
type malformedFrame struct {
	name string
	data []byte
	err  error
}

// malformedProtobufFrames are frames whose length prefix is wrong: the same
// for commands and replies.
var malformedProtobufFrames = []malformedFrame{
	{"length beyond the frame", []byte{0x10, 0x01, 0x02}, io.ErrShortBuffer},
	{"cut off length", []byte{0x80}, io.EOF},
	{"length overflowing uint64", []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x02}, io.EOF},
	{"length overflowing int", []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f, 0x01}, io.ErrShortBuffer},
}

// Commands come from an untrusted client: malformed input must be an error,
// never a panic or a loop. A body which fails to decode must not be taken for
// io.EOF, which a caller takes for a fully processed frame.
func TestProtobufCommandDecoder_Malformed(t *testing.T) {
	frames := append([]malformedFrame{
		{"empty frame", []byte{}, io.EOF},
		{"cut off command", commandFrame(t, TypeProtobuf, &Command{Id: 1})[:2], io.ErrShortBuffer},
		// Field 1 with wire type 7, which does not exist.
		{"invalid body", []byte{0x01, 0x0f}, nil},
	}, malformedProtobufFrames...)
	for _, tt := range frames {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewProtobufCommandDecoder(tt.data).Decode()
			require.Nil(t, cmd)
			require.Error(t, err)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NotErrorIs(t, err, io.EOF)
			}
		})
	}
}

func TestReplyDecoder(t *testing.T) {
	forEachType(t, func(t *testing.T, protoType Type) {
		t.Run("several replies", func(t *testing.T) {
			replies := readReplies(t, newReplyDecoder(protoType, replyFrame(t, protoType,
				&Reply{Id: 1, Connect: &ConnectResult{Client: "client"}},
				&Reply{Push: &Push{Channel: "c", Pub: &Publication{Data: Raw(`{"a":1}`), Offset: 5}}},
				&Reply{Id: 2},
			)))
			require.Len(t, replies, 3)
			require.Equal(t, "client", replies[0].Connect.Client)
			require.Equal(t, `{"a":1}`, string(replies[1].Push.Pub.Data))
			require.Equal(t, uint64(5), replies[1].Push.Pub.Offset)
			require.Equal(t, uint32(2), replies[2].Id)
		})
		t.Run("empty frame", func(t *testing.T) {
			require.Empty(t, readReplies(t, newReplyDecoder(protoType, nil)))
		})
		t.Run("sizes", func(t *testing.T) {
			for _, size := range messageSizes {
				payload := textPayload(size, int64(size))
				replies := readReplies(t, newReplyDecoder(protoType, replyFrame(t, protoType, &Reply{Push: &Push{Channel: "c", Pub: &Publication{Data: payload}}})))
				require.Len(t, replies, 1, "size %d", size)
				require.Equal(t, string(payload), string(replies[0].Push.Pub.Data), "size %d", size)
			}
		})
		t.Run("reset", func(t *testing.T) {
			decoder := newReplyDecoder(protoType, replyFrame(t, protoType, &Reply{Id: 1}))
			require.Len(t, readReplies(t, decoder), 1)
			require.NoError(t, decoder.Reset(replyFrame(t, protoType, &Reply{Id: 2})))
			replies := readReplies(t, decoder)
			require.Len(t, replies, 1)
			require.Equal(t, uint32(2), replies[0].Id)
		})
	})
}

// A payload may have a newline in it, which is what separates replies in a
// JSON frame. The encoders drop newlines of payloads, so that a client which
// splits a frame on them (as SDKs in other languages do) does not see a
// publication torn in two.
func TestJSONReplyDecoder_PayloadWithNewline(t *testing.T) {
	frame := replyFrame(t, TypeJSON,
		&Reply{Push: &Push{Channel: "chat:1", Pub: &Publication{Data: Raw("{\"num\":\n1}")}}},
		&Reply{Id: 7},
	)
	replies := readReplies(t, NewJSONReplyDecoder(frame))
	require.Len(t, replies, 2)
	require.Equal(t, Raw(`{"num":1}`), replies[0].Push.Pub.Data)
	require.Equal(t, uint32(7), replies[1].Id)
}

// Replies come from a server, which a client does not have to trust either.
func TestReplyDecoder_Malformed(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		decoder := NewJSONReplyDecoder([]byte(`{"id": 1}` + "\n" + `{"id": `))
		reply, err := decoder.Decode()
		require.NoError(t, err)
		require.Equal(t, uint32(1), reply.Id)
		for i := 0; i < 2; i++ {
			// The error stays until Reset.
			_, err = decoder.Decode()
			require.Error(t, err)
			require.NotErrorIs(t, err, io.EOF)
		}
	})
	for _, tt := range malformedProtobufFrames {
		t.Run(fmt.Sprintf("protobuf/%s", tt.name), func(t *testing.T) {
			reply, err := NewProtobufReplyDecoder(tt.data).Decode()
			require.Nil(t, reply)
			require.ErrorIs(t, err, tt.err)
		})
	}
}
