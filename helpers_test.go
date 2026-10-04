package protocol

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/centrifugal/protocol/cfjson"
)

// Helpers shared by the tests of the package. The benchmarks live in the
// protocol_test package and do not use them, see bench_test.go.

// protocolTypes are the protocol types every encoder and decoder exists for.
var protocolTypes = []Type{TypeJSON, TypeProtobuf}

// forEachType runs f as a subtest for each protocol type.
func forEachType(t *testing.T, f func(t *testing.T, protoType Type)) {
	t.Helper()
	for _, protoType := range protocolTypes {
		t.Run(string(protoType), func(t *testing.T) {
			f(t, protoType)
		})
	}
}

// textPayload returns a payload of n bytes which is valid for both protocol
// types: a JSON string of random letters. Raw is written into JSON as it is,
// so for JSON a payload has to be JSON already; Protobuf takes any bytes.
func textPayload(n int, seed int64) Raw {
	if n <= 2 {
		// The shortest JSON string.
		return Raw(`""`)
	}
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	b[0], b[n-1] = '"', '"'
	for i := 1; i < n-1; i++ {
		b[i] = byte('a' + r.Intn(26))
	}
	return b
}

// publishCommand is a command with a payload of the given size.
func publishCommand(id uint32, channel string, payloadSize int) *Command {
	return &Command{Id: id, Publish: &PublishRequest{Channel: channel, Data: textPayload(payloadSize, int64(id))}}
}

// encodeCommand encodes a command the way a client does, ready to be put
// into a frame (a Protobuf command comes with its length in front).
func encodeCommand(tb testing.TB, protoType Type, cmd *Command) []byte {
	tb.Helper()
	var data []byte
	var err error
	if protoType == TypeJSON {
		data, err = NewJSONCommandEncoder().Encode(cmd)
	} else {
		data, err = NewProtobufCommandEncoder().Encode(cmd)
	}
	require.NoError(tb, err)
	return data
}

// commandFrame puts commands into one frame, as a client sends them: JSON
// commands separated by newlines, Protobuf ones one after another.
func commandFrame(tb testing.TB, protoType Type, cmds ...*Command) []byte {
	tb.Helper()
	var frame []byte
	for i, cmd := range cmds {
		if protoType == TypeJSON && i > 0 {
			frame = append(frame, '\n')
		}
		frame = append(frame, encodeCommand(tb, protoType, cmd)...)
	}
	return frame
}

// replyFrame puts replies into one frame, as a server sends them.
func replyFrame(tb testing.TB, protoType Type, replies ...*Reply) []byte {
	tb.Helper()
	encoder := GetDataEncoder(protoType)
	defer PutDataEncoder(protoType, encoder)
	for _, reply := range replies {
		data, err := GetReplyEncoder(protoType).Encode(reply)
		require.NoError(tb, err)
		require.NoError(tb, encoder.Encode(data))
	}
	return encoder.Finish()
}

// newReplyDecoder returns the ReplyDecoder of a protocol type.
func newReplyDecoder(protoType Type, frame []byte) ReplyDecoder {
	if protoType == TypeJSON {
		return NewJSONReplyDecoder(frame)
	}
	return NewProtobufReplyDecoder(frame)
}

// readCommands decodes all commands of a frame. A CommandDecoder returns the
// last command together with io.EOF.
func readCommands(tb testing.TB, decoder CommandDecoder) []*Command {
	tb.Helper()
	var commands []*Command
	for {
		cmd, err := decoder.Decode()
		if cmd != nil {
			commands = append(commands, cmd)
		}
		if errors.Is(err, io.EOF) {
			return commands
		}
		require.NoError(tb, err)
	}
}

// readStream decodes all commands of a stream, checking the size reported for
// each. Like a CommandDecoder, a StreamCommandDecoder may return the last
// command together with io.EOF.
func readStream(tb testing.TB, decoder StreamCommandDecoder) []*Command {
	tb.Helper()
	var commands []*Command
	for {
		cmd, size, err := decoder.Decode()
		if cmd != nil {
			require.Positive(tb, size)
			commands = append(commands, cmd)
		}
		if errors.Is(err, io.EOF) {
			return commands
		}
		require.NoError(tb, err)
	}
}

// readReplies decodes all replies of a frame. Unlike a CommandDecoder, a
// ReplyDecoder reports the end of a frame with io.EOF on its own.
func readReplies(tb testing.TB, decoder ReplyDecoder) []*Reply {
	tb.Helper()
	var replies []*Reply
	for {
		reply, err := decoder.Decode()
		if err != nil {
			require.ErrorIs(tb, err, io.EOF)
			require.Nil(tb, reply)
			return replies
		}
		replies = append(replies, reply)
	}
}

// requireCommands checks decoded commands against the ones which were
// encoded, payloads included.
func requireCommands(tb testing.TB, want, got []*Command) {
	tb.Helper()
	require.Len(tb, got, len(want))
	for i := range want {
		require.Equal(tb, want[i].Id, got[i].Id, "command %d", i)
		if want[i].Publish != nil {
			require.Equal(tb, want[i].Publish.Channel, got[i].Publish.Channel, "command %d", i)
			require.Equal(tb, string(want[i].Publish.Data), string(got[i].Publish.Data), "command %d", i)
		}
	}
}

// decodeInto decodes data an encoder produced back into msg, with the codec
// of the protocol type. JSON goes through encoding/json on purpose: what the
// encoders write must be readable by a decoder other than our own.
func decodeInto(tb testing.TB, protoType Type, data []byte, msg message) {
	tb.Helper()
	if protoType == TypeJSON {
		require.NoError(tb, json.Unmarshal(data, msg))
		return
	}
	require.NoError(tb, msg.UnmarshalCF(data))
}

// nilElement looks for a nil message in the slices and maps of a message and
// returns the name of the field which has it.
func nilElement(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			return nilElement(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			if where := nilElement(v.Field(i)); where != "" {
				return f.Name + "." + where
			}
		}
	case reflect.Slice, reflect.Map:
		if v.Type().Elem().Kind() != reflect.Pointer {
			return ""
		}
		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				if v.Index(i).IsNil() {
					return "nil"
				}
				if where := nilElement(v.Index(i)); where != "" {
					return where
				}
			}
			return ""
		}
		for iter := v.MapRange(); iter.Next(); {
			if iter.Value().IsNil() {
				return "nil"
			}
			if where := nilElement(iter.Value()); where != "" {
				return where
			}
		}
	}
	return ""
}

// message is what every type of the protocol is: a message of
// google.golang.org/protobuf, with the methods cfprotobuf and cfjson generate.
type message interface {
	proto.Message
	MarshalCF() ([]byte, error)
	MarshalToCF([]byte) (int, error)
	MarshalToSizedBufferCF([]byte) (int, error)
	SizeCF() int
	UnmarshalCF([]byte) error
	cfjson.Appender
	cfjson.Decoder
}

// messageTypes has a constructor for every type of the protocol, see
// TestMessageTypesComplete.
var messageTypes = map[string]func() message{
	"Error":                func() message { return new(Error) },
	"EmulationRequest":     func() message { return new(EmulationRequest) },
	"Command":              func() message { return new(Command) },
	"Reply":                func() message { return new(Reply) },
	"Push":                 func() message { return new(Push) },
	"Dictionary":           func() message { return new(Dictionary) },
	"ClientInfo":           func() message { return new(ClientInfo) },
	"Publication":          func() message { return new(Publication) },
	"Join":                 func() message { return new(Join) },
	"Leave":                func() message { return new(Leave) },
	"Unsubscribe":          func() message { return new(Unsubscribe) },
	"Subscribe":            func() message { return new(Subscribe) },
	"Message":              func() message { return new(Message) },
	"Connect":              func() message { return new(Connect) },
	"Disconnect":           func() message { return new(Disconnect) },
	"Refresh":              func() message { return new(Refresh) },
	"ConnectRequest":       func() message { return new(ConnectRequest) },
	"ConnectResult":        func() message { return new(ConnectResult) },
	"RefreshRequest":       func() message { return new(RefreshRequest) },
	"RefreshResult":        func() message { return new(RefreshResult) },
	"SubscribeRequest":     func() message { return new(SubscribeRequest) },
	"SubscribeResult":      func() message { return new(SubscribeResult) },
	"KeyedItem":            func() message { return new(KeyedItem) },
	"TrackBatch":           func() message { return new(TrackBatch) },
	"SubRefreshRequest":    func() message { return new(SubRefreshRequest) },
	"SubRefreshResult":     func() message { return new(SubRefreshResult) },
	"UnsubscribeRequest":   func() message { return new(UnsubscribeRequest) },
	"UnsubscribeResult":    func() message { return new(UnsubscribeResult) },
	"PublishRequest":       func() message { return new(PublishRequest) },
	"PublishResult":        func() message { return new(PublishResult) },
	"PresenceRequest":      func() message { return new(PresenceRequest) },
	"PresenceResult":       func() message { return new(PresenceResult) },
	"PresenceStatsRequest": func() message { return new(PresenceStatsRequest) },
	"PresenceStatsResult":  func() message { return new(PresenceStatsResult) },
	"StreamPosition":       func() message { return new(StreamPosition) },
	"HistoryRequest":       func() message { return new(HistoryRequest) },
	"HistoryResult":        func() message { return new(HistoryResult) },
	"PingRequest":          func() message { return new(PingRequest) },
	"PingResult":           func() message { return new(PingResult) },
	"RPCRequest":           func() message { return new(RPCRequest) },
	"RPCResult":            func() message { return new(RPCResult) },
	"SendRequest":          func() message { return new(SendRequest) },
	"FilterNode":           func() message { return new(FilterNode) },
}

// messageNames are the names of messageTypes, sorted.
var messageNames = func() []string {
	names := make([]string, 0, len(messageTypes))
	for name := range messageTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}()

func newMessage(name string) message {
	return messageTypes[name]()
}
