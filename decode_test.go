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

func TestProtobufReplyDecoder_Decode_Many(t *testing.T) {
	encoder := NewProtobufDataEncoder()
	replyEncoder := NewProtobufReplyEncoder()
	for _, id := range []uint32{1, 2} {
		replyData, err := replyEncoder.Encode(&Reply{Id: id})
		require.NoError(t, err)
		require.NoError(t, encoder.Encode(replyData))
	}

	decoder := NewProtobufReplyDecoder(encoder.Finish())
	var replies []*Reply
	for {
		reply, err := decoder.Decode()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}
		replies = append(replies, reply)
	}
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
