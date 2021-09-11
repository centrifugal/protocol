package protocol

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONCommandDecoder_Decode_Single(t *testing.T) {
	data := []byte(`{"id": 1}`)
	decoder := GetCommandDecoder(TypeJSON, data)
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
	require.Len(t, commands, 1)
}

func TestJSONCommandDecoder_Decode_Large(t *testing.T) {
	var s string
	for i := 0; i < 120000; i++ {
		s += "1"
	}
	data := []byte(`{"id": 1, "x": "` + s + `"}`)
	decoder := GetCommandDecoder(TypeJSON, data)
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
	require.Len(t, commands, 1)
}

func TestJSONCommandDecoder_Decode_Many(t *testing.T) {
	data := []byte(`{"id": 1}
{"id": 2}`)
	decoder := GetCommandDecoder(TypeJSON, data)
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
	require.Len(t, commands, 2)
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
