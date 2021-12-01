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
	data := []byte(`{"method":1,"params":{"channel":"chat:1","recover":true,"epoch":"WHBN"},"id":222}
{"method":1,"params":{"channel":"chat:2","recover":true,"epoch":"yenC"},"id":223}
{"method":1,"params":{"channel":"chat:index"},"id":224}
`)
	decoder := GetCommandDecoder(TypeJSON, data)
	commands := readCommands(t, decoder)
	require.Len(t, commands, 3)
	require.Contains(t, string(commands[0].Params), "chat:1")
	require.Contains(t, string(commands[1].Params), "chat:2")
	require.Contains(t, string(commands[2].Params), "chat:index")
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
