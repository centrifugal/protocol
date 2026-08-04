package protocol_test

import (
	"errors"
	"fmt"
	"io"

	"github.com/centrifugal/protocol"
)

// A single transport frame may contain several commands: in JSON they are
// separated by a `\n` delimiter, in Protobuf each command is prefixed with its
// length. Decoders take care of the framing, so the loop is the same for both.
func ExampleGetCommandDecoder() {
	frame := []byte(`{"id":1,"connect":{"name":"go"}}
{"id":2,"subscribe":{"channel":"news"}}`)

	decoder := protocol.GetCommandDecoder(protocol.TypeJSON, frame)
	defer protocol.PutCommandDecoder(protocol.TypeJSON, decoder)

	for {
		cmd, err := decoder.Decode()
		if cmd != nil {
			// Server takes the first non-null request out of a Command.
			switch {
			case cmd.Connect != nil:
				fmt.Println(cmd.Id, protocol.FrameTypeConnect, cmd.Connect.Name)
			case cmd.Subscribe != nil:
				fmt.Println(cmd.Id, protocol.FrameTypeSubscribe, cmd.Subscribe.Channel)
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Println("decode error:", err)
			}
			break
		}
	}
	// Output:
	// 1 connect go
	// 2 subscribe news
}

// Replies are encoded per protocol type. The encoded result may be sent to a
// connection as is, or concatenated with other messages using a DataEncoder.
func ExampleGetReplyEncoder() {
	reply := &protocol.Reply{
		Id:      1,
		Connect: &protocol.ConnectResult{Client: "6d67dbfd", Version: "6.0.0"},
	}

	data, err := protocol.GetReplyEncoder(protocol.TypeJSON).Encode(reply)
	if err != nil {
		fmt.Println("encode error:", err)
		return
	}
	fmt.Println(string(data))
	// Output:
	// {"id":1,"connect":{"client":"6d67dbfd","version":"6.0.0"}}
}
