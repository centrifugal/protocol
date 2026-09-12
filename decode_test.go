package protocol

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func readCommands(t testing.TB, decoder CommandDecoder) []*Command {
	t.Helper()
	var commands []*Command
	for {
		cmd, err := decoder.Decode()
		if err != nil {
			if err == io.EOF {
				if cmd != nil {
					commands = append(commands, cmd)
				}
				break
			}
			t.Fatal(err)
		}
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}
	return commands
}

func TestJSONCommandDecoder_Decode_Single(t *testing.T) {
	data := []byte(`{"id": 1}`)
	decoder := GetCommandDecoder(TypeJSON, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 1)
}

func TestJSONCommandDecoder_Decode_Single_ExtraNewLine(t *testing.T) {
	data := []byte(`{"id": 1}
`)
	decoder := GetCommandDecoder(TypeJSON, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 1)
}

func TestJSONCommandDecoder_Decode_Large(t *testing.T) {
	var s string
	for i := 0; i < 200000; i++ {
		s += "1"
	}
	data := []byte(`{"id": 1, "x": "` + s + `"}`)
	decoder := GetCommandDecoder(TypeJSON, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 1)
}

func TestJSONCommandDecoder_Decode_Many(t *testing.T) {
	data := []byte(`{"id": 1}
{"id": 2}`)
	decoder := GetCommandDecoder(TypeJSON, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 2)
	require.Equal(t, uint32(1), commands[0].Id)
	require.Equal(t, uint32(2), commands[1].Id)
}

func TestJSONCommandDecoder_DifferentNumberOfMessages(t *testing.T) {
	data1 := []byte(`{"id": 1}`)
	data2 := []byte(`{"id": 2}
{"id": 3}`)

	decoder := GetCommandDecoder(TypeJSON, data1)

	commands := readCommands(t, decoder)
	require.Len(t, commands, 1)
	require.Equal(t, uint32(1), commands[0].Id)

	err := decoder.Reset(data2)
	require.NoError(t, err)
	commands = readCommands(t, decoder)
	require.Len(t, commands, 2)
	require.Equal(t, uint32(2), commands[0].Id)
	require.Equal(t, uint32(3), commands[1].Id)

	err = decoder.Reset(data1)
	require.NoError(t, err)
	commands = readCommands(t, decoder)
	require.Len(t, commands, 1)
}

func TestJSONCommandDecoder_Decode_Many_ExtraNewLine(t *testing.T) {
	data := []byte(`{"subscribe":{"channel":"chat:1","recover":true,"epoch":"WHBN"},"id":222}
{"subscribe":{"channel":"chat:2","recover":true,"epoch":"yenC"},"id":223}
{"subscribe":{"channel":"chat:index"},"id":224}
`)
	decoder := GetCommandDecoder(TypeJSON, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 3)
	require.Equal(t, "chat:1", commands[0].Subscribe.Channel)
	require.Equal(t, "chat:2", commands[1].Subscribe.Channel)
	require.Equal(t, "chat:index", commands[2].Subscribe.Channel)
}

func TestJSONCommandDecoder_Decode_Many_UnexpectedEOF(t *testing.T) {
	data := []byte(``)
	decoder := GetCommandDecoder(TypeJSON, data)
	_, err := decoder.Decode()
	require.Equal(t, io.ErrUnexpectedEOF, err)
}

func TestJSONCommandDecoder_Decode_Many_FormatError(t *testing.T) {
	data := []byte(`{"id": 1}


{"id": 2}
`)
	decoder := GetCommandDecoder(TypeJSON, data)
	_, err := decoder.Decode()
	require.NoError(t, err)
	_, err = decoder.Decode()
	require.Error(t, err)
}

func TestProtobufCommandDecoder_Decode_Many(t *testing.T) {
	encoder := NewProtobufCommandEncoder()
	data1, err := encoder.Encode(&Command{Id: 1})
	require.NoError(t, err)
	data2, err := encoder.Encode(&Command{Id: 2})
	require.NoError(t, err)
	data1 = append(data1, data2...)

	data := make([]byte, len(data1))
	copy(data, data1)

	decoder := GetCommandDecoder(TypeProtobuf, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 2)
	if len(commands) == 2 { // Make Goland happy.
		require.Equal(t, uint32(1), commands[0].Id)
		require.Equal(t, uint32(2), commands[1].Id)
	}
}

func TestProtobufCommandDecoder_Decode_ShortData(t *testing.T) {
	encoder := NewProtobufCommandEncoder()
	data1, err := encoder.Encode(&Command{Id: 1})
	require.NoError(t, err)
	data := make([]byte, len(data1)-2)
	copy(data, data1)

	decoder := GetCommandDecoder(TypeProtobuf, data)
	for {
		_, err = decoder.Decode()
		require.ErrorIs(t, err, io.ErrShortBuffer)
		break
	}
}

// Commands come from an untrusted client, so malformed input must produce an
// error without a Command - decoding must never panic and must always terminate.
func TestProtobufCommandDecoder_Decode_Malformed(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		err  error
	}{
		{
			name: "empty frame",
			data: []byte{},
			err:  io.EOF,
		},
		{
			name: "length prefix beyond buffer",
			data: []byte{0x10, 0x01, 0x02},
			err:  io.ErrShortBuffer,
		},
		{
			name: "truncated varint",
			data: []byte{0x80},
			err:  io.EOF,
		},
		{
			name: "varint overflowing uint64",
			data: []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x02},
			err:  io.EOF,
		},
		{
			name: "length overflowing int arithmetic",
			data: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f, 0x01},
			err:  io.ErrShortBuffer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewProtobufCommandDecoder(tt.data)
			cmd, err := decoder.Decode()
			require.Nil(t, cmd)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

// A body which fails to unmarshal must be reported as an error of its own - not
// as io.EOF, which callers treat as a fully processed frame.
func TestProtobufCommandDecoder_Decode_InvalidBody(t *testing.T) {
	badBody := []byte{0x0F} // Field 1 with wire type 7, which is not valid.
	data := append([]byte{byte(len(badBody))}, badBody...)

	decoder := NewProtobufCommandDecoder(data)
	cmd, err := decoder.Decode()
	require.Nil(t, cmd)
	require.Error(t, err)
	require.NotErrorIs(t, err, io.EOF)
}

func readReplies(t testing.TB, decoder ReplyDecoder) []*Reply {
	t.Helper()
	var replies []*Reply
	for {
		reply, err := decoder.Decode()
		if err != nil {
			// Unlike CommandDecoder, a ReplyDecoder signals the end of a frame
			// with io.EOF on its own, without a Reply attached to it.
			require.ErrorIs(t, err, io.EOF)
			require.Nil(t, reply)
			break
		}
		replies = append(replies, reply)
	}
	return replies
}

func TestProtobufReplyDecoder_Decode_Many(t *testing.T) {
	encoder := NewProtobufDataEncoder()
	replyEncoder := NewProtobufReplyEncoder()
	for _, id := range []uint32{1, 2} {
		replyData, err := replyEncoder.Encode(&Reply{Id: id})
		require.NoError(t, err)
		require.NoError(t, encoder.Encode(replyData))
	}

	replies := readReplies(t, NewProtobufReplyDecoder(encoder.Finish()))
	require.Len(t, replies, 2)
	if len(replies) == 2 { // Make Goland happy.
		require.Equal(t, uint32(1), replies[0].Id)
		require.Equal(t, uint32(2), replies[1].Id)
	}
}

// Malformed framing must result in an error - decoding must never panic and must
// always terminate, since replies may come from an untrusted server.
func TestProtobufReplyDecoder_Decode_Malformed(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		err  error
	}{
		{
			name: "length prefix beyond buffer",
			data: []byte{0x10, 0x01, 0x02},
			err:  io.ErrShortBuffer,
		},
		{
			name: "truncated varint",
			data: []byte{0x80},
			err:  io.EOF,
		},
		{
			name: "varint overflowing uint64",
			data: []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x02},
			err:  io.EOF,
		},
		{
			name: "length overflowing int arithmetic",
			data: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f, 0x01},
			err:  io.ErrShortBuffer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewProtobufReplyDecoder(tt.data)
			reply, err := decoder.Decode()
			require.Nil(t, reply)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestJSONReplyDecoder_Decode_Many(t *testing.T) {
	dataEncoder := NewJSONDataEncoder()
	replyEncoder := NewJSONReplyEncoder()
	for _, id := range []uint32{1, 2} {
		replyData, err := replyEncoder.Encode(&Reply{Id: id, Connect: &ConnectResult{Client: "client"}})
		require.NoError(t, err)
		require.NoError(t, dataEncoder.Encode(replyData))
	}

	replies := readReplies(t, NewJSONReplyDecoder(dataEncoder.Finish()))
	require.Len(t, replies, 2)
	if len(replies) == 2 { // Make Goland happy.
		require.Equal(t, uint32(1), replies[0].Id)
		require.Equal(t, uint32(2), replies[1].Id)
		require.Equal(t, "client", replies[0].Connect.Client)
	}
}

func TestJSONReplyDecoder_Decode_Empty(t *testing.T) {
	require.Empty(t, readReplies(t, NewJSONReplyDecoder(nil)))
}

// A Raw payload may contain a raw newline, which is exactly what delimits
// replies inside a JSON frame. Raw.MarshalJSON strips those on encode so that a
// payload cannot inject a delimiter into the frame - this asserts the whole
// round trip, since a client which splits a frame on `\n` (as SDKs in other
// languages do) would otherwise see the publication torn in two.
func TestJSONReplyDecoder_Decode_PayloadWithNewline(t *testing.T) {
	dataEncoder := NewJSONDataEncoder()
	replyEncoder := NewJSONReplyEncoder()

	pubReply, err := replyEncoder.Encode(&Reply{
		Push: &Push{Channel: "chat:1", Pub: &Publication{Data: Raw("{\"num\":\n1}")}},
	})
	require.NoError(t, err)
	require.NoError(t, dataEncoder.Encode(pubReply))
	nextReply, err := replyEncoder.Encode(&Reply{Id: 7})
	require.NoError(t, err)
	require.NoError(t, dataEncoder.Encode(nextReply))

	replies := readReplies(t, NewJSONReplyDecoder(dataEncoder.Finish()))
	require.Len(t, replies, 2)
	if len(replies) == 2 { // Make Goland happy.
		require.Equal(t, "chat:1", replies[0].Push.Channel)
		require.Equal(t, Raw(`{"num":1}`), replies[0].Push.Pub.Data)
		require.Equal(t, uint32(7), replies[1].Id)
	}
}

// Replies come from a server, so malformed input must produce an error instead
// of a panic.
func TestJSONReplyDecoder_Decode_Malformed(t *testing.T) {
	decoder := NewJSONReplyDecoder([]byte(`{"id": 1}` + "\n" + `{"id": `))
	reply, err := decoder.Decode()
	require.NoError(t, err)
	require.Equal(t, uint32(1), reply.Id)
	_, err = decoder.Decode()
	require.Error(t, err)
	require.NotErrorIs(t, err, io.EOF)
}

func TestJSONReplyDecoder_Reset(t *testing.T) {
	replyEncoder := NewJSONReplyEncoder()
	first, err := replyEncoder.Encode(&Reply{Id: 1})
	require.NoError(t, err)
	second, err := replyEncoder.Encode(&Reply{Id: 2})
	require.NoError(t, err)

	decoder := NewJSONReplyDecoder(first)
	replies := readReplies(t, decoder)
	require.Len(t, replies, 1)

	require.NoError(t, decoder.Reset(second))
	replies = readReplies(t, decoder)
	require.Len(t, replies, 1)
	require.Equal(t, uint32(2), replies[0].Id)
}

func TestProtobufReplyDecoder_Reset(t *testing.T) {
	replyEncoder := NewProtobufReplyEncoder()
	dataEncoder := NewProtobufDataEncoder()
	replyData, err := replyEncoder.Encode(&Reply{Id: 1})
	require.NoError(t, err)
	require.NoError(t, dataEncoder.Encode(replyData))
	first := dataEncoder.Finish()

	dataEncoder.Reset()
	replyData, err = replyEncoder.Encode(&Reply{Id: 2})
	require.NoError(t, err)
	require.NoError(t, dataEncoder.Encode(replyData))
	second := dataEncoder.Finish()

	decoder := NewProtobufReplyDecoder(first)
	replies := readReplies(t, decoder)
	require.Len(t, replies, 1)

	// Reset must rewind the offset, not just swap the frame - otherwise the
	// second frame would be reported as fully consumed already.
	require.NoError(t, decoder.Reset(second))
	replies = readReplies(t, decoder)
	require.Len(t, replies, 1)
	require.Equal(t, uint32(2), replies[0].Id)
}
